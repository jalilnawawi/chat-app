package api

import (
	"mime"
	"net/http/httptest"
	"net/url"
	"path"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/jalilnawawi/chat-app/server/internal/store"
)

// Nama berkas dari komputer orang lain tidak boleh ikut menentukan tempat
// penyimpanan. Ini pemeriksaan yang paling penting di berkas ini.
func TestStorageKeyTidakPernahMemakaiNamaKiriman(t *testing.T) {
	id := uuid.MustParse("018f2c80-0000-7000-8000-000000000001")

	jahat := []string{
		"../../../etc/passwd",
		`..\..\windows\system32\config`,
		"/etc/shadow",
		"berkas\x00.png",
		strings.Repeat("a/", 100) + "dalam.png",
	}

	for _, nama := range jahat {
		key := storageKey(id, "image/png", nama)

		if strings.Contains(key, "..") {
			t.Errorf("%q menghasilkan kunci dengan '..': %q", nama, key)
		}
		if strings.HasPrefix(key, "/") {
			t.Errorf("%q menghasilkan kunci absolut: %q", nama, key)
		}
		// Satu-satunya bagian yang boleh berubah-ubah adalah ekstensi, dan id
		// selalu ada di dalamnya.
		if !strings.Contains(key, id.String()) {
			t.Errorf("%q menghasilkan kunci tanpa id: %q", nama, key)
		}
		if strings.Count(key, "/") != 2 {
			t.Errorf("%q menghasilkan kedalaman tak terduga: %q", nama, key)
		}
	}
}

func TestStorageKeyMengambilEkstensiDariTipeIsi(t *testing.T) {
	id := uuid.New()

	// Tipe ditebak dari isi berkas, jadi ekstensinya pun harus mengikuti isi —
	// bukan mengikuti nama yang diberikan pengunggah.
	key := storageKey(id, "image/png", "sebenarnya-bukan.txt")
	if !strings.HasSuffix(key, ".png") {
		t.Errorf("kunci = %q, mau berakhiran .png", key)
	}
}

// Tipe hasil sniffing membawa parameter ("; charset=utf-8"), dan ekstensi yang
// dipilih harus tetap yang lazim dikenal orang — bukan entri alfabetis pertama
// yang kebetulan terdaftar untuk tipe itu.
func TestStorageKeyMemakaiEkstensiYangLazim(t *testing.T) {
	id := uuid.New()

	cases := map[string]string{
		"text/html; charset=utf-8":  ".html",
		"text/plain; charset=utf-8": ".txt",
		"image/jpeg":                ".jpg",
		"application/pdf":           ".pdf",
	}
	for contentType, want := range cases {
		if key := storageKey(id, contentType, "apa.bin"); !strings.HasSuffix(key, want) {
			t.Errorf("%s menghasilkan %q, mau berakhiran %s", contentType, key, want)
		}
	}
}

func TestSafeNameMembersihkanNamaTampilan(t *testing.T) {
	cases := map[string]string{
		"../../rahasia.pdf":     "rahasia.pdf",
		`C:\Users\a\foto.png`:   "foto.png",
		"":                      "lampiran",
		"   ":                   "lampiran",
		"..":                    "lampiran",
		"laporan triwulan.xlsx": "laporan triwulan.xlsx",
	}
	for in, want := range cases {
		if got := safeName(in); got != want {
			t.Errorf("safeName(%q) = %q, mau %q", in, got, want)
		}
	}
}

func TestSafeNameMemotongNamaYangKepanjangan(t *testing.T) {
	if got := safeName(strings.Repeat("x", 500)); len(got) != 200 {
		t.Errorf("panjang = %d, mau 200", len(got))
	}
}

// Gambar boleh tampil di dalam halaman; selebihnya harus terunduh. Ini yang
// menjaga berkas HTML atau SVG kiriman orang tidak berjalan sebagai bagian
// dari aplikasi.
func TestHanyaGambarAmanYangDisajikanInline(t *testing.T) {
	cases := map[string]string{
		"image/png":              "inline",
		"image/jpeg":             "inline",
		"image/gif":              "inline",
		"image/webp":             "inline",
		"image/svg+xml":          "attachment",
		"text/html":              "attachment",
		"application/pdf":        "attachment",
		"application/javascript": "attachment",
		"text/plain":             "attachment",
	}

	for mime, want := range cases {
		w := httptest.NewRecorder()
		setAttachmentHeaders(w, store.Attachment{Name: "berkas", MIME: mime})

		got := w.Header().Get("Content-Disposition")
		if !strings.HasPrefix(got, want) {
			t.Errorf("%s disajikan sebagai %q, mau %s", mime, got, want)
		}
		if w.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("%s: nosniff hilang", mime)
		}
		if w.Header().Get("Content-Security-Policy") == "" {
			t.Errorf("%s: CSP hilang", mime)
		}
	}
}

func TestEncodeFilenameMenyertakanDuaBentuk(t *testing.T) {
	got := encodeFilename("Laporan – Final.pdf")

	// Bentuk ASCII untuk client lama: tidak boleh ada byte di luar ASCII yang
	// bocor ke dalam tanda kutip.
	if !strings.Contains(got, `filename="Laporan `) {
		t.Errorf("bentuk ASCII hilang: %q", got)
	}
	if !strings.Contains(got, "filename*=UTF-8''") {
		t.Errorf("bentuk UTF-8 hilang: %q", got)
	}
}

func TestEncodeFilenameMenetralkanKutipDanBackslash(t *testing.T) {
	// Tanda kutip yang lolos akan menutup atribut lebih awal dan membuat sisa
	// nama dibaca sebagai parameter header tersendiri.
	got := encodeFilename(`a"; attachment; b\c.png`)

	ascii := got[len(`filename="`):strings.Index(got, `"; filename*=`)]
	if strings.ContainsAny(ascii, `"\`) {
		t.Errorf("kutip/backslash lolos ke header: %q", got)
	}
}

// Nama berkas yang mengandung titik koma dan tanda kutip adalah percobaan
// menyelundupkan parameter header kedua — dan parameter `filename` kedua itulah
// yang akan dipakai browser untuk menamai berkas yang tersimpan.
//
// Diuji dengan mengurai hasilnya memakai parser header sungguhan, bukan dengan
// mencocokkan teks: yang menentukan aman atau tidak adalah bagaimana header itu
// DIBACA, bukan bagaimana dia terlihat.
func TestEncodeFilenameTidakBisaMenyelundupkanParameter(t *testing.T) {
	jahat := []string{
		`nota; filename="lain.exe`,
		`a", filename="b.exe`,
		`x.pdf"; filename*=UTF-8''jahat.exe`,
	}

	for _, nama := range jahat {
		_, params, err := mime.ParseMediaType("attachment; " + encodeFilename(nama))
		if err != nil {
			t.Errorf("%q menghasilkan header yang tidak bisa diurai: %v", nama, err)
			continue
		}

		// Parser memilih filename* bila ada, dan itu harus mengembalikan nama
		// aslinya utuh — bukan nama sisipan.
		if params["filename"] != nama {
			t.Errorf("%q terbaca sebagai %q", nama, params["filename"])
		}
	}
}

func TestEncodeFilenameMempertahankanNamaUTF8(t *testing.T) {
	// "é" adalah 0xC3 0xA9 dalam UTF-8; keduanya harus dipersenkan.
	got := encodeAttrChars("café.pdf")
	if got != "caf%C3%A9.pdf" {
		t.Errorf("hasil = %q, mau caf%%C3%%A9.pdf", got)
	}
}

func TestDimensionsHanyaMenerimaAngkaWajar(t *testing.T) {
	ok := url.Values{"w": {"800"}, "h": {"600"}}
	w, h := dimensions(ok)
	if w == nil || h == nil || *w != 800 || *h != 600 {
		t.Fatalf("ukuran sah ditolak: %v %v", w, h)
	}

	buruk := []url.Values{
		{},
		{"w": {"800"}},
		{"w": {"0"}, "h": {"600"}},
		{"w": {"-1"}, "h": {"600"}},
		{"w": {"abc"}, "h": {"600"}},
		{"w": {"999999"}, "h": {"600"}},
	}
	for _, q := range buruk {
		if w, h := dimensions(q); w != nil || h != nil {
			t.Errorf("%v seharusnya ditolak, dapat %v %v", q, *w, *h)
		}
	}
}

func TestHasDuplicateMenangkapLampiranGanda(t *testing.T) {
	a, b := uuid.New(), uuid.New()

	if hasDuplicate([]uuid.UUID{a, b}) {
		t.Error("dua id berbeda dianggap duplikat")
	}
	if !hasDuplicate([]uuid.UUID{a, b, a}) {
		t.Error("id yang berulang tidak terdeteksi")
	}
	if hasDuplicate(nil) {
		t.Error("daftar kosong dianggap duplikat")
	}
}

// Video dan rekaman suara boleh diputar di dalam halaman. Menambah tipe ke
// daftar inline adalah keputusan keamanan, jadi yang diuji di sini bukan hanya
// bahwa video jadi inline — melainkan bahwa yang BISA membawa kode tetap tidak.
func TestRekamanDiputarInlineTapiBerkasAktifTetapTerunduh(t *testing.T) {
	cases := map[string]string{
		"video/mp4":       "inline",
		"video/webm":      "inline",
		"audio/mpeg":      "inline",
		"application/ogg": "inline",

		// Keduanya DIRANCANG untuk membawa script, dan keduanya tetap di luar.
		"image/svg+xml": "attachment",
		"text/html":     "attachment",
		// Video yang tidak ada di daftar ikut terunduh: daftarnya izin, bukan
		// larangan, jadi tipe baru tidak pernah lolos tanpa disebut.
		"video/quicktime": "attachment",
	}

	for mime, want := range cases {
		w := httptest.NewRecorder()
		setAttachmentHeaders(w, store.Attachment{Name: "berkas", MIME: mime})

		if got := w.Header().Get("Content-Disposition"); !strings.HasPrefix(got, want) {
			t.Errorf("%s disajikan sebagai %q, mau %s", mime, got, want)
		}
	}
}

// Tanpa header ini, browser tidak akan pernah MENCOBA meminta sepotong:
// pemutar video menganggap berkasnya tidak bisa dilompati dan mengunduhnya
// dari awal sampai posisi yang diklik.
func TestAcceptRangesSelaluDisebut(t *testing.T) {
	for _, mime := range []string{"video/mp4", "image/png", "application/pdf"} {
		w := httptest.NewRecorder()
		setAttachmentHeaders(w, store.Attachment{Name: "berkas", MIME: mime})

		if got := w.Header().Get("Accept-Ranges"); got != "bytes" {
			t.Errorf("%s: Accept-Ranges = %q, mau bytes", mime, got)
		}
	}
}

func TestThumbStorageKeyBertetanggaDenganAslinya(t *testing.T) {
	// Turunan JPEG dari sumber PNG: ekstensinya mengikuti turunannya, bukan
	// sumbernya, karena byte yang tersimpan di sana memang JPEG.
	got := thumbStorageKey("2026/09/018f2c80-0000-7000-8000-000000000001.png", "image/jpeg")
	want := "2026/09/018f2c80-0000-7000-8000-000000000001_t.jpg"
	if got != want {
		t.Errorf("kunci turunan = %q, mau %q", got, want)
	}

	// Tetap satu direktori dengan aslinya — itu yang membuat pasangannya
	// terlihat saat seseorang membuka penyimpanan.
	if path.Dir(got) != path.Dir("2026/09/x.png") {
		t.Errorf("turunan pindah direktori: %q", got)
	}
	// Dan tidak pernah bertabrakan dengan kunci aslinya.
	if got == "2026/09/018f2c80-0000-7000-8000-000000000001.png" {
		t.Error("kunci turunan sama dengan kunci aslinya")
	}
}

func TestThumbStorageKeyUntukTurunanPNG(t *testing.T) {
	got := thumbStorageKey("2026/09/abc.webp", "image/png")
	if want := "2026/09/abc_t.png"; got != want {
		t.Errorf("kunci turunan = %q, mau %q", got, want)
	}
}

// Turunan dari WebP adalah PNG. Menamainya "logo.webp" berarti berbohong
// kepada siapa pun yang menyimpannya — yang tersimpan PNG.
func TestThumbFileNameMengikutiIsiTurunannya(t *testing.T) {
	cases := []struct{ nama, mime, mau string }{
		{"logo.webp", "image/png", "logo.png"},
		{"foto.png", "image/jpeg", "foto.jpg"},
		{"tanpa-ekstensi", "image/jpeg", "tanpa-ekstensi.jpg"},
		{".gitignore", "image/png", "turunan.png"},
	}
	for _, c := range cases {
		if got := thumbFileName(c.nama, c.mime); got != c.mau {
			t.Errorf("thumbFileName(%q, %q) = %q, mau %q", c.nama, c.mime, got, c.mau)
		}
	}
}
