package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/jalilnawawi/chat-app/server/internal/blob"
	"github.com/jalilnawawi/chat-app/server/internal/store"
)

// maxAttachmentsPerMessage membatasi berapa lampiran boleh menempel di satu
// pesan. Bukan soal penyimpanan — tiap berkasnya sudah dibatasi sendiri —
// melainkan soal bentuk: sepuluh lampiran masih bisa ditampilkan sebagai satu
// pesan, seratus tidak.
const maxAttachmentsPerMessage = 10

// sniffLen adalah jumlah byte yang dibaca untuk menebak tipe berkas.
// http.DetectContentType tidak pernah melihat lebih jauh dari ini.
const sniffLen = 512

// inlineTypes adalah SATU-SATUNYA tipe yang boleh dirender langsung di dalam
// halaman. Selebihnya dipaksa terunduh.
//
// Ini garis pertahanan, bukan sekadar pilihan tampilan. Lampiran disajikan dari
// origin yang sama dengan aplikasi, jadi berkas yang bisa dieksekusi browser —
// HTML, dan terutama SVG, yang boleh memuat <script> — akan berjalan sebagai
// bagian dari aplikasi ini bila dibuka inline. Cookie sesi memang httpOnly dan
// tidak bisa dibaca script, tapi script yang berjalan di origin ini tetap bisa
// MEMAKAI sesi itu lewat fetch. Empat tipe di bawah tidak punya kemampuan
// mengeksekusi apa pun.
var inlineTypes = map[string]bool{
	"image/png":  true,
	"image/jpeg": true,
	"image/gif":  true,
	"image/webp": true,
}

func (s *Server) handleUploadAttachment(w http.ResponseWriter, r *http.Request) {
	me := userFrom(r.Context())

	if s.blobs == nil {
		writeError(w, http.StatusServiceUnavailable, "lampiran tidak aktif di server ini")
		return
	}

	// Batasnya dipasang pada BODY, bukan diperiksa setelah berkasnya masuk.
	// Memeriksa belakangan berarti sudah terlanjur menerima — dan menerima
	// adalah persis yang ingin dihindari. Kelonggaran 64 KB menampung header
	// multipart, yang ikut terhitung di sini tapi bukan bagian dari berkasnya.
	r.Body = http.MaxBytesReader(w, r.Body, s.cfg.MaxUploadBytes+(64<<10))

	part, filename, err := firstFilePart(r)
	if err != nil {
		s.m.AttachmentUploads.WithLabelValues("ditolak").Inc()
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	defer part.Close() //nolint:errcheck

	// Tipe berkas ditentukan dari ISINYA, bukan dari yang diakui client.
	// Header Content-Type pada unggahan adalah pernyataan pengunggah, dan
	// pengunggah bisa saja berbohong — misalnya menyebut berkas HTML sebagai
	// gambar supaya lolos ke jalur yang dirender inline.
	head := make([]byte, sniffLen)
	n, err := io.ReadFull(part, head)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		s.writeUploadError(w, err)
		return
	}
	head = head[:n]
	contentType := http.DetectContentType(head)

	id, err := uuid.NewV7()
	if err != nil {
		s.writeStoreError(w, err, "id lampiran")
		return
	}

	key := storageKey(id, contentType, filename)

	// Byte yang sudah terbaca untuk menebak tipe disambung kembali di depan
	// sisanya, sehingga penyimpanan menerima berkas utuh tanpa pernah ada
	// salinan lengkapnya di memori.
	body := &countingReader{r: io.MultiReader(bytes.NewReader(head), part)}

	// Batas waktu sendiri untuk unggahan. Tanpa ini, satu client yang mengirim
	// satu byte per menit bisa menahan goroutine dan koneksi penyimpanan
	// berjam-jam — cara termurah menghabiskan sumber daya server tanpa
	// melanggar satu pun batas ukuran.
	putCtx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()

	if err := s.blobs.Put(putCtx, key, contentType, body, -1); err != nil {
		// Unggahan yang gagal di tengah jalan bisa meninggalkan berkas separuh
		// di penyimpanan. Tidak ada baris yang menunjuk ke sana, jadi tidak ada
		// yang bisa membacanya — tapi tetap memakan ruang, dan pembersih
		// lampiran yatim hanya tahu tentang baris.
		s.deleteBlobQuietly(key)
		s.writeUploadError(w, err)
		return
	}

	att := store.Attachment{
		ID:   id,
		URL:  store.AttachmentURL(id),
		Name: safeName(filename),
		MIME: contentType,
		Size: body.n,
	}
	att.Width, att.Height = dimensions(r.URL.Query())

	if err := s.store.CreateAttachment(r.Context(), me.ID, key, att); err != nil {
		// Baris gagal ditulis: buang lagi byte-nya, jangan tinggalkan berkas
		// yang tidak akan pernah ada yang menyebutnya.
		s.deleteBlobQuietly(key)
		s.m.AttachmentUploads.WithLabelValues("gagal").Inc()
		s.writeStoreError(w, err, "catat lampiran")
		return
	}

	s.m.AttachmentUploads.WithLabelValues("ok").Inc()
	s.m.AttachmentBytes.Add(float64(body.n))
	writeJSON(w, http.StatusCreated, att)
}

func (s *Server) handleDownloadAttachment(w http.ResponseWriter, r *http.Request) {
	me := userFrom(r.Context())

	if s.blobs == nil {
		writeError(w, http.StatusServiceUnavailable, "lampiran tidak aktif di server ini")
		return
	}

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "id lampiran tidak valid")
		return
	}

	att, key, err := s.store.AttachmentForRead(r.Context(), id, me.ID)
	if err != nil {
		s.m.AttachmentDownloads.WithLabelValues("ditolak").Inc()
		s.writeStoreError(w, err, "baca lampiran")
		return
	}

	obj, err := s.blobs.Get(r.Context(), key)
	if errors.Is(err, blob.ErrNotFound) {
		// Baris ada tapi isinya hilang. Ini bukan 404 biasa dari sudut pandang
		// operator — artinya penyimpanan dan database sudah tidak sepakat.
		s.m.AttachmentDownloads.WithLabelValues("hilang").Inc()
		s.log.Error("isi lampiran tidak ada di penyimpanan", "attachment", id, "key", key)
		writeError(w, http.StatusNotFound, "isi lampiran tidak ditemukan")
		return
	}
	if err != nil {
		s.m.AttachmentDownloads.WithLabelValues("gagal").Inc()
		s.log.Error("ambil lampiran", "attachment", id, "err", err)
		writeError(w, http.StatusBadGateway, "penyimpanan lampiran tidak bisa dihubungi")
		return
	}
	defer obj.Body.Close() //nolint:errcheck

	setAttachmentHeaders(w, att)
	if obj.Size >= 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(obj.Size, 10))
	}

	s.m.AttachmentDownloads.WithLabelValues("ok").Inc()
	if _, err := io.Copy(w, obj.Body); err != nil {
		// Browser yang menutup tab di tengah unduhan sampai ke sini. Tidak ada
		// yang bisa — atau perlu — dilakukan; header sudah terkirim.
		s.log.Debug("unduhan lampiran terputus", "attachment", id, "err", err)
	}
}

// setAttachmentHeaders memasang seluruh aturan penyajian lampiran di satu
// tempat, supaya tidak ada jalur yang lupa salah satunya.
func setAttachmentHeaders(w http.ResponseWriter, att store.Attachment) {
	disposition := "attachment"
	if inlineTypes[att.MIME] {
		disposition = "inline"
	}

	w.Header().Set("Content-Type", att.MIME)
	w.Header().Set("Content-Disposition", disposition+"; "+encodeFilename(att.Name))

	// nosniff mencegah browser menebak tipe sendiri dan mengabaikan yang kita
	// sebutkan — tebakan itu yang membuat berkas "gambar" berisi HTML pernah
	// dieksekusi sebagai halaman.
	w.Header().Set("X-Content-Type-Options", "nosniff")

	// Lapis terakhir: sekalipun sesuatu lolos sampai dirender, halaman ini
	// tidak boleh memuat apa pun dan berjalan di origin buntu.
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")

	// Isi sebuah lampiran tidak pernah berubah — id-nya menunjuk byte yang itu
	// saja. "private" menjaga cache bersama (proxy perusahaan, CDN) tidak ikut
	// menyimpannya, karena hak baca lampiran ditentukan per user.
	w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
}

// encodeFilename menulis nama berkas dalam dua bentuk sekaligus: versi ASCII
// untuk client lama, dan versi UTF-8 sesuai RFC 5987 untuk yang paham.
// Tanpa yang kedua, "Laporan Triwulan – Final.pdf" sampai dengan nama rusak.
func encodeFilename(name string) string {
	ascii := strings.Map(func(r rune) rune {
		if r < 32 || r > 126 || r == '"' || r == '\\' {
			return '_'
		}
		return r
	}, name)
	return fmt.Sprintf(`filename="%s"; filename*=UTF-8''%s`, ascii, encodeAttrChars(name))
}

// encodeAttrChars menyandikan nama untuk bentuk `filename*` sesuai RFC 5987.
//
// url.PathEscape tidak bisa dipakai di sini: dia meloloskan karakter yang sah di
// sebuah path tapi PUNYA ARTI di dalam header — titik koma, koma, sama dengan.
// Nama berkas berisi titik koma akan membuat sisa namanya terbaca sebagai
// parameter header tersendiri, dan parameter `filename` kedua itu yang dipakai
// browser. Jadi yang dibiarkan apa adanya hanya attr-char yang disebut standar;
// selebihnya dipersenkan.
func encodeAttrChars(name string) string {
	const unreserved = "!#$&+-.^_`|~"

	var b strings.Builder
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z', '0' <= c && c <= '9',
			strings.IndexByte(unreserved, c) >= 0:
			b.WriteByte(c)
		default:
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

// commonExtensions memetakan tipe yang sering muncul ke ekstensi yang memang
// dikenal orang.
//
// mime.ExtensionsByType mengembalikan SEMUA ekstensi terdaftar dalam urutan
// alfabetis, jadi entri pertamanya sering bukan yang lazim: text/html menjadi
// ".ehtml" dan text/plain menjadi ".asc". Keduanya sah secara standar dan
// keduanya membingungkan siapa pun yang membuka penyimpanan untuk melihat isinya.
var commonExtensions = map[string]string{
	"image/png":       ".png",
	"image/jpeg":      ".jpg",
	"image/gif":       ".gif",
	"image/webp":      ".webp",
	"text/plain":      ".txt",
	"text/html":       ".html",
	"application/pdf": ".pdf",
	"application/zip": ".zip",
}

// storageKey menyusun tempat penyimpanan dari id, BUKAN dari nama berkas
// kiriman. Nama kiriman hanya dipinjam ekstensinya, dan itu pun sebagai
// cadangan terakhir.
//
// Nama dari komputer orang lain tidak pernah aman dijadikan path: "../" dan
// kerabatnya hanya perlu lolos sekali. Dengan skema ini tidak ada yang perlu
// dibersihkan — tidak ada bagian nama kiriman yang ikut menentukan lokasi.
//
// Ekstensinya sendiri tidak menentukan apa pun: lampiran selalu disajikan
// dengan tipe yang tercatat di database, tidak pernah dengan tebakan dari nama
// berkas. Dia ada murni supaya isi penyimpanan bisa dibaca manusia.
func storageKey(id uuid.UUID, contentType, filename string) string {
	// Parameter seperti "; charset=utf-8" ikut di contentType hasil sniffing,
	// dan tidak boleh ikut menentukan ekstensi.
	base := contentType
	if mediaType, _, err := mime.ParseMediaType(contentType); err == nil {
		base = mediaType
	}

	ext := commonExtensions[base]
	if ext == "" {
		if exts, err := mime.ExtensionsByType(base); err == nil && len(exts) > 0 {
			ext = exts[0]
		}
	}
	if ext == "" {
		if e := path.Ext(filename); len(e) > 1 && len(e) <= 8 && isSimpleExt(e) {
			ext = e
		}
	}

	now := time.Now().UTC()
	// Dipecah per bulan supaya satu direktori tidak menampung jutaan entri —
	// dan supaya "berkas dari bulan lalu" bisa dilihat tanpa memindai semuanya.
	return fmt.Sprintf("%04d/%02d/%s%s", now.Year(), int(now.Month()), id.String(), ext)
}

func isSimpleExt(ext string) bool {
	for _, r := range ext[1:] {
		if !('a' <= r && r <= 'z' || 'A' <= r && r <= 'Z' || '0' <= r && r <= '9') {
			return false
		}
	}
	return true
}

// safeName membersihkan nama yang akan ditampilkan dan dikirim balik saat
// unduh. Tidak menentukan apa pun soal penyimpanan — lihat storageKey.
func safeName(name string) string {
	name = path.Base(strings.ReplaceAll(name, `\`, "/"))
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." {
		return "lampiran"
	}
	if len(name) > 200 {
		name = name[:200]
	}
	return name
}

// dimensions membaca ukuran gambar dari query, bukan dari form.
//
// Unggahan diproses sebagai aliran: berkasnya dilewatkan ke penyimpanan sambil
// dibaca, tanpa pernah utuh di memori. Itu berarti field form yang datang
// SETELAH berkas tidak akan pernah terbaca tepat waktu. Query string tidak
// punya urutan — dia sudah lengkap sebelum byte pertama tiba.
func dimensions(q url.Values) (*int, *int) {
	w, wErr := strconv.Atoi(q.Get("w"))
	h, hErr := strconv.Atoi(q.Get("h"))
	if wErr != nil || hErr != nil || w <= 0 || h <= 0 || w > 100000 || h > 100000 {
		return nil, nil
	}
	return &w, &h
}

// firstFilePart mengambil bagian multipart pertama yang membawa berkas.
//
// Sengaja memakai MultipartReader, bukan ParseMultipartForm: yang kedua
// menyalin seluruh unggahan ke memori dan disk sementara lebih dulu, lalu
// menyerahkannya. Untuk berkas yang hanya perlu diteruskan ke penyimpanan,
// singgah itu murni biaya.
func firstFilePart(r *http.Request) (io.ReadCloser, string, error) {
	mr, err := r.MultipartReader()
	if err != nil {
		return nil, "", errors.New("unggahan harus berupa multipart/form-data")
	}

	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			return nil, "", errors.New("tidak ada berkas di unggahan")
		}
		if err != nil {
			return nil, "", errors.New("unggahan tidak bisa dibaca")
		}
		if part.FileName() != "" {
			return part, part.FileName(), nil
		}
		_ = part.Close()
	}
}

// deleteBlobQuietly membuang byte yang terlanjur tersimpan saat unggahannya
// gagal. Konteksnya sengaja lepas dari permintaan: yang memicu pembersihan ini
// justru permintaan yang sudah gagal, dan konteksnya kemungkinan besar sudah
// mati.
func (s *Server) deleteBlobQuietly(key string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.blobs.Delete(ctx, key); err != nil {
		s.log.Warn("membersihkan lampiran gagal", "key", key, "err", err)
	}
}

// writeUploadError memisahkan "kebesaran" dari kegagalan lain, karena hanya
// yang pertama bisa diperbaiki oleh orang yang mengunggah.
func (s *Server) writeUploadError(w http.ResponseWriter, err error) {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		s.m.AttachmentUploads.WithLabelValues("ditolak").Inc()
		writeError(w, http.StatusRequestEntityTooLarge,
			fmt.Sprintf("berkas terlalu besar, maksimal %d MB", s.cfg.MaxUploadBytes>>20))
		return
	}
	s.m.AttachmentUploads.WithLabelValues("gagal").Inc()
	s.log.Error("unggah lampiran", "err", err)
	writeError(w, http.StatusBadGateway, "penyimpanan lampiran tidak bisa dihubungi")
}

// countingReader menghitung berapa byte yang benar-benar lewat. Ukuran berkas
// tidak diambil dari yang diakui client — hanya yang sudah tersimpan yang
// dihitung.
type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}
