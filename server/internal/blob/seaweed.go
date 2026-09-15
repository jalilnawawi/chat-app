package blob

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"path"
	"strings"
)

// Seaweed menyimpan lampiran lewat filer SeaweedFS.
//
// Kenapa filer, bukan API master+volume yang asli? Karena filer memberi
// namespace berupa PATH, dan path itulah yang disimpan di kolom storage_key.
// Alur aslinya — minta fid ke master, lalu unggah ke volume server yang
// ditunjuk — menuntut aplikasi menyimpan fid sekaligus menangani perpindahan
// volume. Filer sudah melakukan keduanya, dan yang ditukar cuma satu lompatan
// jaringan di dalam klaster penyimpanan.
//
// Seluruh percakapan dengan filer adalah HTTP biasa: POST untuk menulis, GET
// untuk membaca, DELETE untuk membuang. Tidak ada SDK yang perlu ditambahkan.
type Seaweed struct {
	base   string
	prefix string
	client *http.Client
}

// NewSeaweed membuat client filer. filerURL contohnya http://127.0.0.1:8890.
func NewSeaweed(filerURL, prefix string) (*Seaweed, error) {
	u, err := url.Parse(filerURL)
	if err != nil {
		return nil, fmt.Errorf("SEAWEED_FILER_URL: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("SEAWEED_FILER_URL harus lengkap, mis. http://127.0.0.1:8890")
	}

	return &Seaweed{
		base:   strings.TrimSuffix(u.String(), "/"),
		prefix: "/" + strings.Trim(prefix, "/"),
		// Sengaja tanpa Timeout di client: unggahan besar sah-sah saja berjalan
		// lama, sedangkan permintaan yang menggantung harus tetap bisa
		// dihentikan. Batas waktunya datang dari context pemanggil, yang tahu
		// persis sedang menunggu apa.
		client: &http.Client{},
	}, nil
}

func (s *Seaweed) url(key string) string {
	return s.base + path.Join(s.prefix, key)
}

// Put mengunggah isi r sebagai multipart, tanpa menyangganya lebih dulu.
//
// io.Pipe yang membuat itu mungkin: bagian multipart ditulis di goroutine
// sendiri sementara http.Client membacanya sebagai body. Tanpa ini, satu
// unggahan 10 MB berarti 10 MB di heap — dan seratus unggahan bersamaan
// berarti satu gigabyte yang tidak pernah direncanakan siapa pun.
func (s *Seaweed) Put(ctx context.Context, key, contentType string, r io.Reader, size int64) error {
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)

	go func() {
		part, err := mw.CreatePart(textprotoHeader(path.Base(key), contentType))
		if err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		if _, err := io.Copy(part, r); err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		_ = pw.CloseWithError(mw.Close())
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url(key), pr)
	if err != nil {
		_ = pr.Close()
		return err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())

	res, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("unggah ke filer: %w", err)
	}
	defer drain(res)

	if res.StatusCode >= 300 {
		return fmt.Errorf("filer menolak unggahan: %s", res.Status)
	}
	return nil
}

func (s *Seaweed) Get(ctx context.Context, key string) (Object, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url(key), nil)
	if err != nil {
		return Object{}, err
	}

	res, err := s.client.Do(req)
	if err != nil {
		return Object{}, fmt.Errorf("baca dari filer: %w", err)
	}
	if res.StatusCode == http.StatusNotFound {
		drain(res)
		return Object{}, ErrNotFound
	}
	if res.StatusCode >= 300 {
		drain(res)
		return Object{}, fmt.Errorf("filer menolak pembacaan: %s", res.Status)
	}

	// Body sengaja TIDAK ditutup di sini — itu tugas pemanggil, yang akan
	// menyalurkannya langsung ke koneksi browser tanpa singgah di memori.
	return Object{Body: res.Body, Size: res.ContentLength}, nil
}

func (s *Seaweed) Delete(ctx context.Context, key string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, s.url(key), nil)
	if err != nil {
		return err
	}

	res, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("hapus di filer: %w", err)
	}
	defer drain(res)

	// 404 dianggap sukses: pembersih lampiran yatim berjalan berulang, dan
	// gagal karena "sudah tidak ada" akan membuatnya mencoba selamanya.
	if res.StatusCode >= 300 && res.StatusCode != http.StatusNotFound {
		return fmt.Errorf("filer menolak penghapusan: %s", res.Status)
	}
	return nil
}

// Ping memanggil daftar direktori dengan batas satu entri — cukup untuk
// membuktikan filer hidup dan menjawab, tanpa menariknya bekerja.
func (s *Seaweed) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.base+"/?limit=1", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")

	res, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("filer tidak menjawab: %w", err)
	}
	defer drain(res)

	if res.StatusCode >= 300 {
		return fmt.Errorf("filer menjawab %s", res.Status)
	}
	return nil
}

// textprotoHeader menyusun header bagian multipart secara manual.
//
// multipart.Writer.CreateFormFile tidak dipakai karena dia memaksa
// Content-Type "application/octet-stream". Filer memakai nilai itu apa adanya
// saat menyajikan file, dan kita ingin tipe yang sebenarnya ikut tersimpan.
func textprotoHeader(filename, contentType string) textproto.MIMEHeader {
	h := make(textproto.MIMEHeader, 2)
	h.Set("Content-Disposition",
		`form-data; name="file"; filename="`+escapeQuotes(filename)+`"`)
	h.Set("Content-Type", contentType)
	return h
}

var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, `\"`)

func escapeQuotes(s string) string { return quoteEscaper.Replace(s) }

// drain menghabiskan lalu menutup body. Tanpa menghabiskannya, koneksi
// keep-alive tidak bisa dipakai ulang dan tiap unggahan membuka socket baru.
func drain(res *http.Response) {
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 1<<16))
	_ = res.Body.Close()
}
