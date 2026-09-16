// Package mail mengirim surat yang jumlahnya sedikit tapi taruhannya besar:
// tautan verifikasi alamat, dan tautan pemulihan password.
//
// Tiga keputusan menentukan bentuk paket ini, dan ketiganya meniru paket push
// dengan sengaja — masalahnya memang sama:
//
//  1. **SMTP_URL kosong berarti fitur email MATI, dan aplikasinya tetap utuh.**
//     Pola yang sama dengan REDIS_URL, SEAWEED_FILER_URL, dan kunci VAPID. Yang
//     hilang cuma verifikasi dan pemulihan; mendaftar, login, dan seluruh chat
//     tidak berubah sebaris pun. Sender nil adalah bentuk "dimatikan", supaya
//     tidak ada pemanggil yang perlu menulis `if mailer != nil` — pemeriksaan
//     seperti itu selalu ada satu yang terlupa.
//
//  2. **Pengiriman tidak pernah menahan permintaan HTTP.** Server SMTP orang
//     lain bisa memakan beberapa detik, dan orang yang menekan "kirim tautan"
//     tidak sedang menunggu itu. Antrean dengan beberapa pekerja, dan kiriman
//     yang DIBUANG kalau antreannya penuh.
//
//  3. **Isi suratnya teks biasa.** Tidak ada HTML, tidak ada gambar, tidak ada
//     pelacak. Yang perlu sampai cuma satu tautan, dan surat yang isinya satu
//     tautan yang bisa dibaca mata telanjang justru yang paling sulit dipalsukan
//     bentuknya oleh orang lain.
package mail

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"net"
	"net/smtp"
	"net/url"
	"strings"
	"sync"
	"time"
)

// sendTimeout membatasi satu percakapan SMTP. Server surat sesekali menggantung,
// dan satu pekerja yang menunggu selamanya berarti satu pekerja yang hilang.
const sendTimeout = 30 * time.Second

// queueSize dan workers jauh lebih kecil daripada milik push: surat di aplikasi
// ini hanya lahir dari tindakan yang dilakukan orang beberapa kali seumur akun,
// bukan dari setiap pesan yang dikirim.
const (
	queueSize = 128
	workers   = 2
)

// Message adalah satu surat. Sengaja tanpa field HTML — lihat keputusan ketiga
// di atas.
type Message struct {
	To      string
	Subject string
	Body    string
}

// Sender mengantre surat untuk dikirim. Nil berarti fitur email dimatikan, dan
// setiap methodnya aman dipanggil pada nilai nil.
type Sender struct {
	from string
	dial dialer
	log  *slog.Logger

	queue chan Message
	wg    sync.WaitGroup

	// mu menjaga queue dari penutupan yang bertabrakan dengan pengiriman —
	// alasan yang sama persis dengan push.Dispatcher: mengirim ke channel
	// tertutup selalu panik, dan select-dengan-default tidak melindungi dari itu.
	mu     sync.RWMutex
	closed bool
}

// dialer membuka satu percakapan SMTP yang siap dipakai. Dipisahkan dari Sender
// supaya pengujian bisa menggantinya tanpa menyalakan server surat.
type dialer func(ctx context.Context) (*smtp.Client, error)

// New menyiapkan pengirim dari SMTP_URL. Mengembalikan nil bila URL-nya kosong —
// itulah bentuk "fitur ini dimatikan".
//
// URL-nya diuraikan SAAT START, bukan saat surat pertama dikirim. Alamat yang
// salah ketik lebih baik muncul sebagai satu baris kegagalan saat menyalakan
// server daripada sebagai tautan verifikasi yang tidak pernah sampai berminggu-
// minggu kemudian.
func New(smtpURL, from string, log *slog.Logger) (*Sender, error) {
	if smtpURL == "" {
		return nil, nil
	}

	u, err := url.Parse(smtpURL)
	if err != nil {
		return nil, fmt.Errorf("SMTP_URL: %w", err)
	}

	var implicitTLS bool
	switch u.Scheme {
	case "smtp":
	case "smtps":
		implicitTLS = true
	default:
		return nil, errors.New("SMTP_URL harus diawali smtp:// atau smtps://")
	}

	host := u.Hostname()
	if host == "" {
		return nil, errors.New("SMTP_URL tidak menyebutkan host")
	}
	port := u.Port()
	if port == "" {
		port = "587"
		if implicitTLS {
			port = "465"
		}
	}
	addr := net.JoinHostPort(host, port)

	var auth smtp.Auth
	if u.User != nil {
		pass, _ := u.User.Password()
		// PlainAuth menolak dirinya sendiri di koneksi yang belum terenkripsi,
		// dan itu justru yang diinginkan: kredensial SMTP yang lewat polos di
		// jaringan adalah kredensial yang sudah bocor.
		auth = smtp.PlainAuth("", u.User.Username(), pass, host)
	}

	s := &Sender{
		from:  from,
		log:   log,
		queue: make(chan Message, queueSize),
		dial:  newDialer(addr, host, implicitTLS, auth),
	}

	s.wg.Add(workers)
	for range workers {
		go s.work()
	}
	return s, nil
}

// Enabled melaporkan apakah pengiriman surat menyala di instance ini.
func (s *Sender) Enabled() bool { return s != nil }

// Enqueue menitipkan surat, tanpa pernah memblokir.
func (s *Sender) Enqueue(m Message) {
	if s == nil || m.To == "" {
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		s.log.Warn("surat tidak jadi dikirim: instance sedang pamit", "kepada", m.To)
		return
	}

	select {
	case s.queue <- m:
	default:
		// Antrean penuh berarti server surat sedang lambat. Yang hilang di sini
		// adalah satu tautan, dan orangnya bisa memintanya lagi — berbeda dari
		// pesan chat, tidak ada isi yang ikut hilang bersamanya.
		s.log.Warn("antrean surat penuh, kiriman dibuang", "kepada", m.To)
	}
}

func (s *Sender) work() {
	defer s.wg.Done()
	for m := range s.queue {
		if err := s.send(m); err != nil {
			// Alamat penerima TIDAK ikut dicatat pada tingkat error yang
			// dikirim ke pengumpul log bersama: sebuah alamat email adalah
			// keterangan pribadi, dan alasan kegagalan sudah cukup untuk
			// mendiagnosis server surat yang bermasalah.
			s.log.Error("kirim surat gagal", "subjek", m.Subject, "err", err)
		}
	}
}

func (s *Sender) send(m Message) error {
	ctx, cancel := context.WithTimeout(context.Background(), sendTimeout)
	defer cancel()

	c, err := s.dial(ctx)
	if err != nil {
		return err
	}
	defer c.Close() //nolint:errcheck

	if err := c.Mail(s.from); err != nil {
		return fmt.Errorf("MAIL FROM: %w", err)
	}
	if err := c.Rcpt(m.To); err != nil {
		return fmt.Errorf("RCPT TO: %w", err)
	}

	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("DATA: %w", err)
	}
	if _, err := w.Write(s.compose(m)); err != nil {
		return fmt.Errorf("tulis isi surat: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("tutup isi surat: %w", err)
	}
	return c.Quit()
}

// compose menyusun surat RFC 5322 yang paling sederhana yang masih benar.
//
// Subjeknya disandikan: "Verifikasi alamat émail kamu" yang dikirim mentah akan
// sampai sebagai deretan karakter rusak di client surat mana pun yang mengikuti
// standar, karena header hanya boleh berisi ASCII.
func (s *Sender) compose(m Message) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", s.from)
	fmt.Fprintf(&b, "To: %s\r\n", m.To)
	fmt.Fprintf(&b, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", m.Subject))
	fmt.Fprintf(&b, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	b.WriteString("Content-Transfer-Encoding: 8bit\r\n")

	// Auto-Submitted memberi tahu server surat penerima bahwa ini bukan surat
	// yang ditulis orang, sehingga balasan otomatis ("saya sedang cuti") tidak
	// dikirim balik ke alamat pengirim aplikasi ini.
	b.WriteString("Auto-Submitted: auto-generated\r\n")
	b.WriteString("\r\n")

	// Titik di awal baris punya arti khusus di dalam DATA: dia mengakhiri
	// suratnya. Baris isi yang kebetulan dimulai dengan titik harus digandakan.
	for line := range strings.SplitSeq(strings.ReplaceAll(m.Body, "\r\n", "\n"), "\n") {
		if strings.HasPrefix(line, ".") {
			b.WriteString(".")
		}
		b.WriteString(line)
		b.WriteString("\r\n")
	}
	b.WriteString(".\r\n")
	return []byte(b.String())
}

// newDialer menyiapkan cara membuka koneksi, sekali, saat start.
func newDialer(addr, host string, implicitTLS bool, auth smtp.Auth) dialer {
	return func(ctx context.Context) (*smtp.Client, error) {
		d := &net.Dialer{}
		tlsCfg := &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}

		var (
			conn net.Conn
			err  error
		)
		if implicitTLS {
			conn, err = (&tls.Dialer{NetDialer: d, Config: tlsCfg}).DialContext(ctx, "tcp", addr)
		} else {
			conn, err = d.DialContext(ctx, "tcp", addr)
		}
		if err != nil {
			return nil, fmt.Errorf("hubungi server surat: %w", err)
		}

		// Batas waktu dipasang pada socket-nya, bukan hanya pada konteks: paket
		// net/smtp tidak menerima konteks sama sekali, jadi tanpa deadline ini
		// sebuah server yang menerima koneksi lalu diam akan menahan pekerja
		// selamanya.
		if deadline, ok := ctx.Deadline(); ok {
			_ = conn.SetDeadline(deadline)
		}

		c, err := smtp.NewClient(conn, host)
		if err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("mulai percakapan SMTP: %w", err)
		}

		if !implicitTLS {
			if ok, _ := c.Extension("STARTTLS"); ok {
				if err := c.StartTLS(tlsCfg); err != nil {
					_ = c.Close()
					return nil, fmt.Errorf("STARTTLS: %w", err)
				}
			}
		}

		if auth != nil {
			if err := c.Auth(auth); err != nil {
				_ = c.Close()
				return nil, fmt.Errorf("autentikasi SMTP: %w", err)
			}
		}
		return c, nil
	}
}

// Close menutup antrean dan menunggu kiriman yang sedang berjalan selesai.
func (s *Sender) Close() {
	if s == nil {
		return
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	close(s.queue)
	s.mu.Unlock()

	s.wg.Wait()
}
