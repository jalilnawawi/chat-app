package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/jalilnawawi/chat-app/server/internal/blob"
	"github.com/jalilnawawi/chat-app/server/internal/imaging"
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

// playableTypes adalah rekaman yang boleh diputar di dalam halaman.
//
// Menambah tipe ke daftar inline adalah keputusan keamanan, jadi alasannya
// harus sama kuatnya dengan yang di atas: tidak satu pun format ini punya
// kemampuan mengeksekusi apa pun. Tidak ada script di dalam MP4, tidak ada
// script di dalam MP3 — berbeda dari SVG dan HTML, yang justru DIRANCANG untuk
// membawa kode dan karena itu tetap dipaksa terunduh.
//
// Tanpa daftar ini, membuka video di tab baru berarti mengunduh berkas
// setengah gigabyte alih-alih memutarnya. Tiga lapis lain — tipe dari isi,
// nosniff, dan CSP — tetap berlaku persis sama.
var playableTypes = map[string]bool{
	"video/mp4":       true,
	"video/webm":      true,
	"audio/mpeg":      true,
	"audio/wave":      true,
	"application/ogg": true,

	// Rekaman suara dari MediaRecorder. Wadahnya sama persis dengan tiga
	// baris di atas — lihat narrowContainer untuk bagaimana tipe ini bisa
	// tercatat tanpa pernah dihasilkan oleh sniffing.
	"audio/webm": true,
	"audio/mp4":  true,
	"audio/ogg":  true,
}

// audioOfContainer memetakan tipe WADAH hasil sniffing ke versi suaranya.
//
// http.DetectContentType membaca beberapa byte pertama, dan beberapa byte
// pertama WebM, MP4, dan Ogg tidak mengatakan apa pun tentang isinya — hanya
// tentang wadahnya. Rekaman suara dari Chrome karenanya tercatat sebagai
// "video/webm", dan client yang jujur mengikuti tipe itu merender pemutar
// video hitam untuk sebuah pesan suara.
var audioOfContainer = map[string]string{
	"video/webm":      "audio/webm",
	"video/mp4":       "audio/mp4",
	"application/ogg": "audio/ogg",
}

// narrowContainer menerima pernyataan pengunggah HANYA untuk satu hal: bahwa
// wadah yang dikenali sniffing berisi suara, bukan gambar bergerak.
//
// Aturan "tipe dari isi, bukan dari pengakuan" tetap utuh, karena pengakuan
// ini tidak bisa memindahkan berkas ke kelas lain. Wadahnya tetap yang
// dikenali dari byte-nya; yang berubah cuma label di dalam kelas yang sama,
// dan kedua labelnya sama-sama ada di playableTypes. Arah sebaliknya — suara
// jadi video, atau apa pun jadi wadah lain — tidak pernah diterima.
//
// Kalau pengakuannya bohong, akibatnya sebuah video tampil dengan pemutar
// suara: tidak ada byte yang dieksekusi, tidak ada header yang berubah selain
// Content-Type yang tetap berupa tipe media tanpa kemampuan menjalankan apa pun.
func narrowContainer(sniffed, claimed string, head []byte) string {
	base, _, err := mime.ParseMediaType(claimed)
	if err != nil {
		return sniffed
	}

	// Pengenal MP4 milik net/http hanya menerima berkas yang salah satu
	// brand-nya berawalan "mp4". Rekaman M4A — brand "M4A ", "isom" — lolos
	// dari sana sebagai octet-stream, padahal wadahnya ISO-BMFF yang sama.
	// Kotak ftyp di awal berkas cukup untuk mengenali wadahnya; yang tidak
	// punya kotak itu tetap octet-stream.
	if sniffed == "application/octet-stream" && base == "audio/mp4" && isISOBMFF(head) {
		return "audio/mp4"
	}

	audio, ok := audioOfContainer[sniffed]
	if !ok || base != audio {
		return sniffed
	}
	return audio
}

// isISOBMFF memeriksa kotak pertama berkas: panjang empat byte, lalu "ftyp",
// dan panjangnya masuk akal untuk sebuah kotak ftyp.
func isISOBMFF(head []byte) bool {
	if len(head) < 16 || string(head[4:8]) != "ftyp" {
		return false
	}
	size := int(head[0])<<24 | int(head[1])<<16 | int(head[2])<<8 | int(head[3])
	return size >= 16 && size <= 256 && size%4 == 0
}

// maxDurationMS sama dengan batas CHECK di 0009_pesan_suara.sql. Angka di luar
// jangkauan dibuang di sini, bukan dibiarkan ditolak database — durasi yang
// salah bukan alasan menggagalkan unggahan yang byte-nya sudah tersimpan.
const maxDurationMS = 3_600_000

// waveformBars adalah panjang persis string bentuk gelombang, sama dengan CHECK
// di 0010_bentuk_gelombang.sql. Bukan batas atas: 40 batang atau tidak sama
// sekali. Gelombang separuh panjang akan digambar client sebagai rekaman yang
// berakhir di tengah.
const waveformBars = 40

// thumbWait adalah berapa lama unggahan mau menunggu giliran mendekode gambar.
//
// Habisnya waktu ini BUKAN kegagalan: lampirannya sudah tersimpan dan sudah
// benar, hanya tanpa turunan. Menunggu lebih lama berarti menahan koneksi orang
// demi turunan yang bisa hidup tanpanya — dan pada saat server sesibuk itu,
// menahan koneksi justru yang paling tidak boleh dilakukan.
const thumbWait = 3 * time.Second

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
	contentType := narrowContainer(http.DetectContentType(head), part.Header.Get("Content-Type"), head)

	id, err := uuid.NewV7()
	if err != nil {
		s.writeStoreError(w, err, "id lampiran")
		return
	}

	key := storageKey(id, contentType, filename)

	// Byte yang sudah terbaca untuk menebak tipe disambung kembali di depan
	// sisanya, sehingga penyimpanan menerima berkas utuh tanpa pernah ada
	// salinan lengkapnya di memori.
	var stream io.Reader = io.MultiReader(bytes.NewReader(head), part)

	// Kecuali untuk gambar yang akan dibuatkan turunan. Turunan menuntut
	// gambarnya utuh — tidak ada cara memperkecil sesuatu sambil hanya melihat
	// sepotong kecilnya — jadi khusus untuk itu salinannya memang ditahan.
	//
	// Yang menjaga ini tetap terbatas adalah MaxBytesReader di atas: salinan
	// ini tidak mungkin melewati batas ukuran unggahan, karena body-nya sendiri
	// tidak mungkin. Untuk berkas selain gambar — justru yang paling besar,
	// video dan arsip — tidak ada salinan sama sekali, persis seperti sebelumnya.
	var source *bytes.Buffer
	if s.cfg.Thumbnails() && inlineTypes[contentType] {
		source = &bytes.Buffer{}
		stream = io.TeeReader(stream, source)
	}

	body := &countingReader{r: stream}

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

	sa := store.StoredAttachment{
		Attachment: store.Attachment{
			ID:   id,
			URL:  store.AttachmentURL(id),
			Name: safeName(filename),
			MIME: contentType,
			Size: body.n,
		},
		Key: key,
	}
	sa.Width, sa.Height = dimensions(r.URL.Query())
	sa.DurationMS = duration(r.URL.Query(), contentType)
	sa.Peaks = peaks(r.URL.Query(), contentType)

	// Ukuran yang diakui client hanya dipakai kalau server tidak punya
	// salinannya untuk diperiksa sendiri — yaitu saat turunan dimatikan. Di
	// luar itu, yang dipercaya adalah header berkasnya: angka dari client bisa
	// salah tanpa niat jahat (browser lama), dan bisa salah dengan niat jahat
	// (ruang selayar penuh yang dipesan oleh satu gambar 1x1).
	s.attachThumbnail(r.Context(), &sa, source)

	if err := s.store.CreateAttachment(r.Context(), me.ID, sa); err != nil {
		// Baris gagal ditulis: buang lagi byte-nya, jangan tinggalkan berkas
		// yang tidak akan pernah ada yang menyebutnya. Turunannya ikut, karena
		// dia pun tidak akan pernah disebut baris mana pun.
		s.deleteBlobQuietly(key)
		if sa.ThumbKey != "" {
			s.deleteBlobQuietly(sa.ThumbKey)
		}
		s.m.AttachmentUploads.WithLabelValues("gagal").Inc()
		s.writeStoreError(w, err, "catat lampiran")
		return
	}

	s.m.AttachmentUploads.WithLabelValues("ok").Inc()
	s.m.AttachmentBytes.Add(float64(body.n))
	writeJSON(w, http.StatusCreated, sa.Attachment)
}

// attachThumbnail mengisi ukuran sebenarnya dan, bila perlu, membuat turunan.
//
// Tidak pernah mengembalikan error, dan itu disengaja. Lampiran yang sudah
// tersimpan dengan benar tidak boleh gagal hanya karena gambarnya tidak bisa
// diperkecil — berkasnya tetap ada, tetap bisa diunduh, tetap bisa dikirim.
// Yang hilang cuma penghematan, dan itu hilang dengan tercatat di metrik.
func (s *Server) attachThumbnail(ctx context.Context, sa *store.StoredAttachment, source *bytes.Buffer) {
	if source == nil {
		return
	}
	src := source.Bytes()

	// Giliran dulu, baru bekerja. Membaca header pun sudah mendekode sebagian,
	// dan yang dijaga di sini adalah memori — bukan lamanya kerja.
	select {
	case s.thumbSem <- struct{}{}:
		defer func() { <-s.thumbSem }()
	case <-time.After(thumbWait):
		s.m.Thumbnails.WithLabelValues("sibuk").Inc()
		return
	case <-ctx.Done():
		return
	}

	// Ukuran dibaca lebih dulu, dan berguna justru pada gambar yang TIDAK
	// dibuatkan turunan: yang sudah kecil tetap butuh ruang yang dipesan dengan
	// benar di layar. Gagal di sini berarti berkasnya bukan gambar yang bisa
	// kita baca sama sekali — termasuk yang pikselnya terlalu banyak untuk
	// didekode — dan tidak ada gunanya melanjutkan.
	size, err := imaging.Measure(src, s.cfg.ThumbMaxPixels)
	if err != nil {
		s.m.Thumbnails.WithLabelValues("gagal").Inc()
		s.log.Warn("membaca ukuran gambar gagal", "attachment", sa.ID, "mime", sa.MIME, "err", err)
		return
	}
	sa.Width, sa.Height = &size.Width, &size.Height

	start := time.Now()
	thumb, err := imaging.Make(src, s.cfg.ThumbMaxDim, s.cfg.ThumbMaxPixels)
	s.m.ThumbnailSeconds.Observe(time.Since(start).Seconds())

	switch {
	case errors.Is(err, imaging.ErrTidakPerlu):
		s.m.Thumbnails.WithLabelValues("dilewati").Inc()
		return
	case err != nil:
		// Gambar yang headernya terbaca tapi isinya rusak. Tetap sah sebagai
		// lampiran — hanya tidak sebagai gambar yang bisa kita olah.
		s.m.Thumbnails.WithLabelValues("gagal").Inc()
		s.log.Warn("membuat turunan gambar gagal", "attachment", sa.ID, "mime", sa.MIME, "err", err)
		return
	}

	thumbKey := thumbStorageKey(sa.Key, thumb.MIME)
	putCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := s.blobs.Put(putCtx, thumbKey, thumb.MIME, bytes.NewReader(thumb.Bytes), int64(len(thumb.Bytes))); err != nil {
		// Unggahan turunan yang gagal di tengah bisa meninggalkan berkas
		// separuh, dan tidak akan ada baris yang menunjuk ke sana.
		s.deleteBlobQuietly(thumbKey)
		s.m.Thumbnails.WithLabelValues("gagal").Inc()
		s.log.Warn("menyimpan turunan gambar gagal", "attachment", sa.ID, "err", err)
		return
	}

	sa.ThumbKey, sa.ThumbMIME, sa.ThumbSize = thumbKey, thumb.MIME, int64(len(thumb.Bytes))
	sa.ThumbURL = store.AttachmentThumbURL(sa.ID)

	s.m.Thumbnails.WithLabelValues("ok").Inc()
	s.m.ThumbnailBytes.Add(float64(len(thumb.Bytes)))
}

// thumbStorageKey menurunkan kunci turunan dari kunci aslinya, bukan menyusunnya
// dari awal.
//
// Keduanya jadi bertetangga di penyimpanan — "2026/09/<uuid>.jpg" dan
// "2026/09/<uuid>_t.jpg" — sehingga siapa pun yang membuka filer bisa langsung
// melihat pasangannya. Id yang sudah ada di dalam kunci aslinya yang menjamin
// tidak ada dua turunan bertabrakan.
// thumbFileName menyesuaikan ekstensi nama tampilan dengan isi turunannya.
//
// Berkas WebP menghasilkan turunan PNG, dan menamainya "logo.webp" berarti
// berbohong kepada siapa pun yang menyimpannya: yang tersimpan PNG. Namanya
// sendiri tidak menentukan apa pun soal penyajian — tipenya selalu datang dari
// database — tapi nama yang salah tetap nama yang salah.
func thumbFileName(name, thumbMIME string) string {
	base := strings.TrimSuffix(name, path.Ext(name))
	if base == "" {
		base = "turunan"
	}
	return base + commonExtensions[thumbMIME]
}

func thumbStorageKey(key, thumbMIME string) string {
	return strings.TrimSuffix(key, path.Ext(key)) + "_t" + commonExtensions[thumbMIME]
}

func (s *Server) handleDownloadAttachment(w http.ResponseWriter, r *http.Request) {
	sa, ok := s.attachmentForRequest(w, r)
	if !ok {
		return
	}
	s.serveBlob(w, r, sa.Key, sa.Attachment, sa.Size)
}

// handleDownloadThumbnail menyajikan turunan kecil.
//
// Izinnya datang dari query yang sama persis dengan berkas aslinya — bukan dari
// pemeriksaan tersendiri yang kebetulan mirip. Turunan adalah gambar yang sama,
// hanya lebih kecil; siapa pun yang tidak boleh melihat yang satu tidak boleh
// melihat yang lain, dan satu-satunya cara memastikan kedua aturan itu tidak
// pernah menyimpang adalah dengan tidak pernah menulisnya dua kali.
func (s *Server) handleDownloadThumbnail(w http.ResponseWriter, r *http.Request) {
	sa, ok := s.attachmentForRequest(w, r)
	if !ok {
		return
	}
	if sa.ThumbKey == "" {
		// Tidak pernah ada turunan untuk lampiran ini. Client hanya meminta
		// alamat ini kalau thumbUrl terisi, jadi sampai ke sini berarti alamatnya
		// dirangkai sendiri — dan jawaban jujurnya memang "tidak ada".
		writeError(w, http.StatusNotFound, "lampiran ini tidak punya turunan")
		return
	}

	// Nama dan tipe turunan BUKAN nama dan tipe aslinya: yang tersaji di sini
	// adalah JPEG atau PNG hasil enkode ulang, apa pun masukannya. Menyebut
	// tipe aslinya berarti mengirim PNG dengan label WebP.
	shown := sa.Attachment
	shown.MIME = sa.ThumbMIME
	shown.Size = sa.ThumbSize
	shown.Name = thumbFileName(sa.Name, sa.ThumbMIME)

	s.serveBlob(w, r, sa.ThumbKey, shown, sa.ThumbSize)
}

// attachmentForRequest menyatukan tiga langkah yang harus dilewati kedua jalur
// baca: lampiran menyala, id-nya masuk akal, dan orang ini berhak membacanya.
func (s *Server) attachmentForRequest(w http.ResponseWriter, r *http.Request) (store.StoredAttachment, bool) {
	if s.blobs == nil {
		writeError(w, http.StatusServiceUnavailable, "lampiran tidak aktif di server ini")
		return store.StoredAttachment{}, false
	}

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "id lampiran tidak valid")
		return store.StoredAttachment{}, false
	}

	sa, err := s.store.AttachmentForRead(r.Context(), id, userFrom(r.Context()).ID)
	if err != nil {
		s.m.AttachmentDownloads.WithLabelValues("ditolak").Inc()
		s.writeStoreError(w, err, "baca lampiran")
		return store.StoredAttachment{}, false
	}
	return sa, true
}

// serveBlob mengalirkan satu berkas dari penyimpanan ke browser — seluruhnya,
// atau sepotong bila diminta.
//
// size datang dari DATABASE, bukan dari penyimpanan. Itu yang membuat rentang
// bisa dinilai — dan ditolak dengan 416 — tanpa satu pun permintaan jaringan
// ke penyimpanan. Client yang meleset menghitung posisi adalah hal biasa saat
// video di-seek berulang kali, dan tiap kesalahannya tidak perlu jadi beban
// bagi penyimpanan yang sedang melayani orang lain.
func (s *Server) serveBlob(
	w http.ResponseWriter,
	r *http.Request,
	key string,
	shown store.Attachment,
	size int64,
) {
	rng, partial, err := parseRange(r.Header.Get("Range"), size)
	if errors.Is(err, errRangeTidakTerpenuhi) {
		s.m.RangeRequests.WithLabelValues("tidak_terpenuhi").Inc()
		// Bentuk "*/panjang" adalah cara memberi tahu client ukuran yang
		// sebenarnya, supaya percobaan berikutnya tidak meleset lagi.
		w.Header().Set("Content-Range", "bytes */"+strconv.FormatInt(size, 10))
		writeError(w, http.StatusRequestedRangeNotSatisfiable, "rentang di luar ukuran berkas")
		return
	}

	var want *blob.Range
	if partial {
		want = &rng
	}

	obj, err := s.blobs.Get(r.Context(), key, want)
	if errors.Is(err, blob.ErrNotFound) {
		// Baris ada tapi isinya hilang. Ini bukan 404 biasa dari sudut pandang
		// operator — artinya penyimpanan dan database sudah tidak sepakat.
		s.m.AttachmentDownloads.WithLabelValues("hilang").Inc()
		s.log.Error("isi lampiran tidak ada di penyimpanan", "attachment", shown.ID, "key", key)
		writeError(w, http.StatusNotFound, "isi lampiran tidak ditemukan")
		return
	}
	if err != nil {
		s.m.AttachmentDownloads.WithLabelValues("gagal").Inc()
		s.log.Error("ambil lampiran", "attachment", shown.ID, "err", err)
		writeError(w, http.StatusBadGateway, "penyimpanan lampiran tidak bisa dihubungi")
		return
	}
	defer obj.Body.Close() //nolint:errcheck

	// Header penyajian dipasang di sini, bukan di awal — SETELAH dipastikan ada
	// yang benar-benar akan disajikan.
	//
	// Salah satunya Cache-Control setahun penuh yang bertanda immutable, dan
	// itu benar untuk byte lampiran yang memang tidak pernah berubah. Menempel
	// di jawaban 404 atau 416, dia berubah jadi kesalahan yang menetap: browser
	// menyimpannya selama setahun dan tidak pernah lagi bertanya, bahkan setelah
	// penyebabnya diperbaiki di sisi server.
	setAttachmentHeaders(w, shown)

	status := http.StatusOK
	length := size

	// obj.Partial, bukan `partial`. Yang menentukan status jawaban adalah apa
	// yang BENAR-BENAR dikirim penyimpanan, bukan apa yang kita minta darinya:
	// penyimpanan yang mengabaikan Range menjawab berkas utuh, dan menandainya
	// 206 berarti pemutar video merakit berkas yang isinya tumpang tindih.
	if obj.Partial {
		end := rng.End
		if obj.Size >= 0 {
			// Panjang yang dilaporkan penyimpanan yang menentukan ujungnya,
			// supaya Content-Range dan Content-Length tidak mungkin berselisih.
			end = rng.Start + obj.Size - 1
		}
		total := obj.Total
		if total < 0 {
			total = size
		}

		status = http.StatusPartialContent
		length = end - rng.Start + 1
		w.Header().Set("Content-Range", contentRange(rng.Start, end, total))
		s.m.RangeRequests.WithLabelValues("sepotong").Inc()
	} else if partial {
		s.m.RangeRequests.WithLabelValues("utuh").Inc()
	}

	if length >= 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(length, 10))
	}

	s.m.AttachmentDownloads.WithLabelValues("ok").Inc()
	w.WriteHeader(status)

	// Permintaan HEAD sampai ke sini juga — ServeMux mencocokkannya dengan pola
	// GET — dan net/http yang membuang body-nya sendiri sambil mempertahankan
	// Content-Length. Itu yang dipakai pemutar video untuk mengetahui panjang
	// berkas sebelum meminta potongan pertamanya.
	if _, err := io.Copy(w, obj.Body); err != nil {
		// Browser yang menutup tab di tengah unduhan sampai ke sini, dan
		// begitu juga pemutar video yang membatalkan potongan karena orangnya
		// melompat ke tempat lain — yang terakhir justru tanda fiturnya bekerja.
		s.log.Debug("unduhan lampiran terputus", "attachment", shown.ID, "err", err)
	}
}

// setAttachmentHeaders memasang seluruh aturan penyajian lampiran di satu
// tempat, supaya tidak ada jalur yang lupa salah satunya.
func setAttachmentHeaders(w http.ResponseWriter, att store.Attachment) {
	disposition := "attachment"
	if inlineTypes[att.MIME] || playableTypes[att.MIME] {
		disposition = "inline"
	}

	w.Header().Set("Content-Type", att.MIME)
	w.Header().Set("Content-Disposition", disposition+"; "+encodeFilename(att.Name))

	// Tanpa ini, browser tidak akan pernah MENCOBA meminta sepotong: pemutar
	// video menganggap berkas tidak bisa dilompati dan mengunduhnya dari awal.
	// Dipasang untuk semua tipe, bukan hanya video — pengunduh yang putus di
	// tengah memakai jalur yang sama untuk melanjutkan, bukan mengulang.
	w.Header().Set("Accept-Ranges", "bytes")

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
	// mime.ExtensionsByType tidak mengenal ketiganya di banyak sistem, dan
	// tanpa entri ini rekaman suara tersimpan tanpa ekstensi.
	"audio/webm": ".weba",
	"audio/mp4":  ".m4a",
	"audio/ogg":  ".ogg",
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

// duration membaca panjang rekaman dari query, dengan alasan yang sama dengan
// dimensions: query sudah lengkap sebelum byte pertama berkasnya tiba.
//
// Hanya untuk suara. Durasi yang menempel pada gambar atau PDF tidak berarti
// apa-apa, dan client yang menampilkannya akan menggambar pemutar untuk
// sesuatu yang tidak bisa diputar.
func duration(q url.Values, contentType string) *int {
	if !strings.HasPrefix(contentType, "audio/") {
		return nil
	}
	d, err := strconv.Atoi(q.Get("d"))
	if err != nil || d <= 0 || d > maxDurationMS {
		return nil
	}
	return &d
}

// peaks membaca bentuk gelombang rekaman dari query, dengan alasan yang sama
// dengan duration: query sudah lengkap sebelum byte pertama berkasnya tiba.
//
// Tepat 40 karakter base64url, tiap karakter satu batang. Yang tidak berbentuk
// itu dibuang, bukan ditolak — gelombang yang salah bukan alasan menggagalkan
// unggahan yang byte-nya sudah tersimpan, persis seperti durasi. Yang dijaga di
// sini bukan kebenaran angkanya (server tidak punya cara memeriksanya) melainkan
// bentuknya: 40 byte dari himpunan karakter yang sempit, jadi string apa pun
// yang menempel di kolom ini aman digambar dan aman dikirim ulang.
func peaks(q url.Values, contentType string) *string {
	if !strings.HasPrefix(contentType, "audio/") {
		return nil
	}
	p := q.Get("p")
	if len(p) != waveformBars {
		return nil
	}
	for i := 0; i < len(p); i++ {
		c := p[i]
		ok := (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
			(c >= '0' && c <= '9') || c == '-' || c == '_'
		if !ok {
			return nil
		}
	}
	return &p
}

// firstFilePart mengambil bagian multipart pertama yang membawa berkas.
//
// Sengaja memakai MultipartReader, bukan ParseMultipartForm: yang kedua
// menyalin seluruh unggahan ke memori dan disk sementara lebih dulu, lalu
// menyerahkannya. Untuk berkas yang hanya perlu diteruskan ke penyimpanan,
// singgah itu murni biaya.
func firstFilePart(r *http.Request) (*multipart.Part, string, error) {
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
