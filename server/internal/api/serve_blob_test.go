package api

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/jalilnawawi/chat-app/server/internal/blob"
	"github.com/jalilnawawi/chat-app/server/internal/metrics"
	"github.com/jalilnawawi/chat-app/server/internal/store"
)

// fakeBlobs adalah penyimpanan dalam memori. Cukup untuk menguji satu-satunya
// hal yang menarik di serveBlob: penerjemahan antara apa yang diminta browser,
// apa yang dijawab penyimpanan, dan status HTTP yang keluar.
type fakeBlobs struct {
	data         map[string][]byte
	abaikanRange bool
	diminta      *blob.Range
}

func (f *fakeBlobs) Put(context.Context, string, string, io.Reader, int64) error { return nil }
func (f *fakeBlobs) Delete(context.Context, string) error                        { return nil }
func (f *fakeBlobs) Ping(context.Context) error                                  { return nil }

func (f *fakeBlobs) Get(_ context.Context, key string, rng *blob.Range) (blob.Object, error) {
	body, ok := f.data[key]
	if !ok {
		return blob.Object{}, blob.ErrNotFound
	}
	f.diminta = rng

	if rng == nil || f.abaikanRange {
		return blob.Object{
			Body:  io.NopCloser(bytes.NewReader(body)),
			Size:  int64(len(body)),
			Total: int64(len(body)),
		}, nil
	}

	potong := body[rng.Start : rng.End+1]
	return blob.Object{
		Body:    io.NopCloser(bytes.NewReader(potong)),
		Size:    int64(len(potong)),
		Total:   int64(len(body)),
		Partial: true,
	}, nil
}

const isiUji = "0123456789"

func serverUji(t *testing.T) (*Server, *fakeBlobs) {
	t.Helper()

	blobs := &fakeBlobs{data: map[string][]byte{"k": []byte(isiUji)}}
	return &Server{
		blobs: blobs,
		m:     metrics.New(),
		log:   slog.New(slog.DiscardHandler),
	}, blobs
}

func sajikan(t *testing.T, s *Server, header string) *httptest.ResponseRecorder {
	t.Helper()

	r := httptest.NewRequest(http.MethodGet, "/api/attachments/x", nil)
	if header != "" {
		r.Header.Set("Range", header)
	}
	w := httptest.NewRecorder()

	s.serveBlob(w, r, "k", store.Attachment{
		ID: uuid.New(), Name: "video.mp4", MIME: "video/mp4",
	}, int64(len(isiUji)))
	return w
}

func TestServeBlobTanpaRangeMengirimSeluruhnya(t *testing.T) {
	s, blobs := serverUji(t)
	w := sajikan(t, s, "")

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, mau 200", w.Code)
	}
	if blobs.diminta != nil {
		t.Errorf("penyimpanan diminta sepotong padahal tidak ada header Range: %+v", blobs.diminta)
	}
	if w.Body.String() != isiUji {
		t.Errorf("isi = %q, mau %q", w.Body.String(), isiUji)
	}
	if got := w.Header().Get("Content-Length"); got != "10" {
		t.Errorf("Content-Length = %q, mau 10", got)
	}
	if w.Header().Get("Content-Range") != "" {
		t.Error("Content-Range muncul di jawaban utuh")
	}
}

func TestServeBlobMenjawabSepotongDengan206(t *testing.T) {
	s, blobs := serverUji(t)
	w := sajikan(t, s, "bytes=2-5")

	if w.Code != http.StatusPartialContent {
		t.Fatalf("status = %d, mau 206", w.Code)
	}
	if blobs.diminta == nil || blobs.diminta.Start != 2 || blobs.diminta.End != 5 {
		t.Errorf("rentang yang diteruskan = %+v, mau 2-5", blobs.diminta)
	}
	if w.Body.String() != "2345" {
		t.Errorf("isi = %q, mau 2345", w.Body.String())
	}
	if got := w.Header().Get("Content-Range"); got != "bytes 2-5/10" {
		t.Errorf("Content-Range = %q, mau bytes 2-5/10", got)
	}
	// Panjangnya harus panjang POTONGANNYA, bukan panjang berkasnya. Salah di
	// sini berarti koneksi menggantung menunggu byte yang tidak akan datang.
	if got := w.Header().Get("Content-Length"); got != "4" {
		t.Errorf("Content-Length = %q, mau 4", got)
	}
}

// Rentang yang menunjuk ke luar berkas dijawab 416 TANPA menyentuh penyimpanan
// sama sekali: ukurannya sudah diketahui dari database. Client yang meleset
// menghitung posisi adalah hal biasa saat video di-seek berulang kali, dan tiap
// kesalahannya tidak perlu jadi beban bagi penyimpanan.
func TestServeBlobMenolakRentangDiLuarUkuranTanpaMenyentuhPenyimpanan(t *testing.T) {
	s, blobs := serverUji(t)
	w := sajikan(t, s, "bytes=50-60")

	if w.Code != http.StatusRequestedRangeNotSatisfiable {
		t.Fatalf("status = %d, mau 416", w.Code)
	}
	if blobs.diminta != nil {
		t.Error("penyimpanan tetap dihubungi untuk rentang yang sudah pasti salah")
	}
	if got := w.Header().Get("Content-Range"); got != "bytes */10" {
		t.Errorf("Content-Range = %q, mau bytes */10", got)
	}
	// Jawaban salah yang tersimpan setahun di browser akan bertahan jauh lebih
	// lama daripada penyebabnya.
	if got := w.Header().Get("Cache-Control"); got != "" {
		t.Errorf("jawaban 416 membawa Cache-Control %q", got)
	}
}

// Penyimpanan boleh mengabaikan Range dan menjawab berkas utuh. Yang keluar
// harus ikut jadi 200 — menandainya 206 berarti mengirim Content-Range yang
// berbohong, dan pemutar video yang mempercayainya merakit berkas yang isinya
// tumpang tindih.
func TestServeBlobIkutJadiUtuhSaatPenyimpananMengabaikanRange(t *testing.T) {
	s, blobs := serverUji(t)
	blobs.abaikanRange = true

	w := sajikan(t, s, "bytes=2-5")

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, mau 200", w.Code)
	}
	if w.Header().Get("Content-Range") != "" {
		t.Error("Content-Range dikirim padahal jawabannya utuh")
	}
	if w.Body.String() != isiUji {
		t.Errorf("isi = %q, mau %q", w.Body.String(), isiUji)
	}
	if got := w.Header().Get("Content-Length"); got != "10" {
		t.Errorf("Content-Length = %q, mau 10", got)
	}
}

// Rentang yang bentuknya tidak dikenali diabaikan, dan penyimpanan tidak perlu
// tahu apa-apa tentangnya.
func TestServeBlobMengabaikanRangeYangTidakDikenali(t *testing.T) {
	s, blobs := serverUji(t)
	w := sajikan(t, s, "bytes=abc-def")

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, mau 200", w.Code)
	}
	if blobs.diminta != nil {
		t.Errorf("rentang tak dikenali tetap diteruskan: %+v", blobs.diminta)
	}
}

// Isi yang hilang dari penyimpanan tidak boleh ikut tersimpan setahun di
// browser: yang perlu diperbaiki ada di sisi server, dan perbaikannya tidak
// akan pernah terlihat kalau client berhenti bertanya.
func TestServeBlobTidakMenandaiJawabanHilangSebagaiAbadi(t *testing.T) {
	s, _ := serverUji(t)

	r := httptest.NewRequest(http.MethodGet, "/api/attachments/x", nil)
	w := httptest.NewRecorder()
	s.serveBlob(w, r, "tidak-ada", store.Attachment{ID: uuid.New(), MIME: "image/png"}, 10)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, mau 404", w.Code)
	}
	if got := w.Header().Get("Cache-Control"); got != "" {
		t.Errorf("jawaban 404 membawa Cache-Control %q", got)
	}
}
