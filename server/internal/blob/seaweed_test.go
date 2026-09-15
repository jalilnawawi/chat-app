package blob

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeFiler menirukan filer SeaweedFS secukupnya untuk membuktikan bahwa
// client ini berbicara dengan benar: path yang tepat, multipart yang bisa
// diurai, dan status yang diterjemahkan jadi error yang tepat.
type fakeFiler struct {
	objects map[string][]byte

	lastPath        string
	lastFilename    string
	lastContentType string
	status          int
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
		_, _ = w.Write(body)

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

	_, err := s.Get(t.Context(), "tidak/ada.png")
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

	obj, err := s.Get(ctx, "a.txt")
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
