package blob

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// fakeFiler menirukan filer SeaweedFS secukupnya untuk membuktikan bahwa
// client ini berbicara dengan benar: path yang tepat, multipart yang bisa
// diurai, dan status yang diterjemahkan jadi error yang tepat.
type fakeFiler struct {
	objects map[string][]byte

	lastPath        string
	lastFilename    string
	lastContentType string
	lastRange       string
	status          int

	// abaikanRange menirukan penyimpanan yang tidak mendukung permintaan
	// sepotong. Header Range boleh diabaikan menurut standar, dan client harus
	// tetap benar saat itu terjadi.
	abaikanRange bool
}

func (f *fakeFiler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.lastPath = r.URL.Path

	switch r.Method {
	case http.MethodPost:
		if f.status != 0 {
			w.WriteHeader(f.status)
			return
		}
		mr, err := r.MultipartReader()
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		part, err := mr.NextPart()
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		f.lastFilename = part.FileName()
		f.lastContentType = part.Header.Get("Content-Type")
		body, _ := io.ReadAll(part)
		f.objects[r.URL.Path] = body
		w.WriteHeader(http.StatusCreated)

	case http.MethodGet:
		body, ok := f.objects[r.URL.Path]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		f.lastRange = r.Header.Get("Range")

		if f.abaikanRange {
			// Filer yang tidak mengenal Range menjawab berkas utuh dengan 200.
			// Itu jawaban yang SAH, dan client harus menyadarinya — kalau tidak,
			// dia meneruskan 206 berisi berkas penuh ke browser.
			_, _ = w.Write(body)
			return
		}
		// ServeContent mengurus Range persis seperti server HTTP sungguhan,
		// termasuk 206, Content-Range, dan 416 — jadi yang diuji di sini adalah
		// client-nya, bukan tiruan aturan yang mungkin salah kita tulis sendiri.
		http.ServeContent(w, r, r.URL.Path, time.Time{}, bytes.NewReader(body))

	case http.MethodDelete:
		if _, ok := f.objects[r.URL.Path]; !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		delete(f.objects, r.URL.Path)
		w.WriteHeader(http.StatusNoContent)
	}
}

func newTestStore(t *testing.T) (*Seaweed, *fakeFiler) {
	t.Helper()

	filer := &fakeFiler{objects: map[string][]byte{}}
	srv := httptest.NewServer(filer)
	t.Cleanup(srv.Close)

	s, err := NewSeaweed(srv.URL, "/chat/attachments")
	if err != nil {
		t.Fatalf("NewSeaweed: %v", err)
	}
	return s, filer
}

func TestPutMenyimpanDenganPathDanTipeYangBenar(t *testing.T) {
	s, filer := newTestStore(t)
	ctx := t.Context()

	isi := "halo lampiran"
	if err := s.Put(ctx, "2026/09/berkas.png", "image/png", strings.NewReader(isi), -1); err != nil {
		t.Fatalf("Put: %v", err)
	}

	// Prefix harus ikut, dan kunci menempel persis di belakangnya.
	if want := "/chat/attachments/2026/09/berkas.png"; filer.lastPath != want {
		t.Errorf("path = %q, mau %q", filer.lastPath, want)
	}
	// Tipe asli harus ikut tersimpan, bukan application/octet-stream bawaan
	// CreateFormFile — itu alasan header multipart disusun manual.
	if filer.lastContentType != "image/png" {
		t.Errorf("content-type = %q, mau image/png", filer.lastContentType)
	}
	if filer.lastFilename != "berkas.png" {
		t.Errorf("filename = %q, mau berkas.png", filer.lastFilename)
	}
	if got := string(filer.objects[filer.lastPath]); got != isi {
		t.Errorf("isi = %q, mau %q", got, isi)
	}
}

func TestGetMengembalikanErrNotFoundUntuk404(t *testing.T) {
	s, _ := newTestStore(t)

	_, err := s.Get(t.Context(), "tidak/ada.png", nil)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, mau ErrNotFound", err)
	}
}

func TestGetMengalirkanIsi(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := t.Context()

	if err := s.Put(ctx, "a.txt", "text/plain", strings.NewReader("isi berkas"), -1); err != nil {
		t.Fatalf("Put: %v", err)
	}

	obj, err := s.Get(ctx, "a.txt", nil)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer obj.Body.Close()

	body, _ := io.ReadAll(obj.Body)
	if string(body) != "isi berkas" {
		t.Errorf("isi = %q", body)
	}
}

// Penghapusan berulang harus aman: penyapu lampiran yatim berjalan terus dan
// beberapa instance bisa menyapu berkas yang sama.
func TestDeleteYangSudahTidakAdaBukanError(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := t.Context()

	if err := s.Put(ctx, "b.txt", "text/plain", strings.NewReader("x"), -1); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if err := s.Delete(ctx, "b.txt"); err != nil {
		t.Fatalf("Delete pertama: %v", err)
	}
	if err := s.Delete(ctx, "b.txt"); err != nil {
		t.Fatalf("Delete kedua harus no-op, malah: %v", err)
	}
}

func TestPutMeneruskanPenolakanFiler(t *testing.T) {
	s, filer := newTestStore(t)
	filer.status = http.StatusInsufficientStorage

	err := s.Put(t.Context(), "c.txt", "text/plain", strings.NewReader("x"), -1)
	if err == nil {
		t.Fatal("Put harus gagal saat filer menolak")
	}
}

// Kesalahan di sisi pembaca harus keluar sebagai error, bukan sebagai berkas
// separuh yang tersimpan diam-diam.
func TestPutMeneruskanErrorDariSumber(t *testing.T) {
	s, _ := newTestStore(t)

	rusak := io.MultiReader(strings.NewReader("awal"), errReader{})
	if err := s.Put(t.Context(), "d.txt", "text/plain", rusak, -1); err == nil {
		t.Fatal("Put harus gagal saat sumbernya error")
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("sumber rusak") }

// Rentang yang dilayani harus sampai ke filer sebagai header Range, dan
// jawabannya harus dikenali sebagai sepotong — bukan sebagai berkas utuh.
func TestGetMeneruskanRentangKeFiler(t *testing.T) {
	s, filer := newTestStore(t)
	ctx := t.Context()

	isi := "0123456789abcdef"
	if err := s.Put(ctx, "v.bin", "application/octet-stream", strings.NewReader(isi), -1); err != nil {
		t.Fatalf("Put: %v", err)
	}

	obj, err := s.Get(ctx, "v.bin", &Range{Start: 4, End: 9})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer obj.Body.Close()

	if filer.lastRange != "bytes=4-9" {
		t.Errorf("header Range = %q, mau bytes=4-9", filer.lastRange)
	}
	if !obj.Partial {
		t.Error("jawaban 206 tidak ditandai sepotong")
	}
	if obj.Total != int64(len(isi)) {
		t.Errorf("Total = %d, mau %d", obj.Total, len(isi))
	}
	if obj.Size != 6 {
		t.Errorf("Size = %d, mau 6", obj.Size)
	}

	body, _ := io.ReadAll(obj.Body)
	if string(body) != "456789" {
		t.Errorf("isi = %q, mau 456789", body)
	}
}

// Ini yang paling mudah salah: penyimpanan yang mengabaikan Range menjawab
// berkas UTUH dengan status 200. Menandainya sepotong berarti mengirim
// Content-Range yang berbohong, dan pemutar video yang mempercayainya akan
// merakit berkas yang isinya tumpang tindih.
func TestGetTidakMengakuSepotongSaatFilerMengabaikanRange(t *testing.T) {
	s, filer := newTestStore(t)
	filer.abaikanRange = true
	ctx := t.Context()

	isi := "0123456789"
	if err := s.Put(ctx, "w.bin", "application/octet-stream", strings.NewReader(isi), -1); err != nil {
		t.Fatalf("Put: %v", err)
	}

	obj, err := s.Get(ctx, "w.bin", &Range{Start: 2, End: 4})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer obj.Body.Close()

	if obj.Partial {
		t.Fatal("jawaban 200 berisi berkas utuh ditandai sepotong")
	}
	body, _ := io.ReadAll(obj.Body)
	if string(body) != isi {
		t.Errorf("isi = %q, mau %q", body, isi)
	}
}

func TestRangeLengthMenghitungKeduaUjungnya(t *testing.T) {
	// Kedua ujung inklusif: bytes=0-0 adalah satu byte, bukan nol.
	if got := (Range{Start: 0, End: 0}).Length(); got != 1 {
		t.Errorf("Length = %d, mau 1", got)
	}
	if got := (Range{Start: 10, End: 19}).Length(); got != 10 {
		t.Errorf("Length = %d, mau 10", got)
	}
}
