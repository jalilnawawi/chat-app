// Package blob menyimpan isi lampiran di luar database.
//
// Seperti hub.Broadcaster dan ratelimit.Limiter, yang dipakai aplikasi adalah
// INTERFACE-nya, bukan implementasinya. Pemilihan penyimpanan adalah keputusan
// deployment: hari ini SeaweedFS lewat filer-nya, besok bisa S3 atau disk
// biasa, dan handler yang mengurus izin, kuota, serta siaran tidak berubah
// sebaris pun.
//
// Yang sengaja TIDAK ada di sini: URL publik. Isi lampiran hanya boleh dibaca
// anggota percakapannya, dan penyimpanan objek tidak tahu apa-apa tentang
// keanggotaan. Jadi tidak ada jalan ke byte-nya selain lewat server ini.
package blob

import (
	"context"
	"errors"
	"io"
)

// ErrNotFound dikembalikan saat kunci tidak ada di penyimpanan.
var ErrNotFound = errors.New("blob tidak ditemukan")

// Object adalah isi sebuah lampiran beserta panjangnya.
type Object struct {
	Body io.ReadCloser
	Size int64
}

type Store interface {
	// Put menulis isi r ke key. size boleh -1 bila tidak diketahui.
	Put(ctx context.Context, key, contentType string, r io.Reader, size int64) error

	// Get membuka isi key. Pemanggil wajib menutup Object.Body.
	Get(ctx context.Context, key string) (Object, error)

	// Delete membuang key. Menghapus key yang sudah tidak ada bukan error:
	// pembersih lampiran yatim berjalan berulang dan harus aman diulang.
	Delete(ctx context.Context, key string) error

	// Ping memastikan penyimpanan bisa dijangkau. Dipakai probe kesiapan.
	Ping(ctx context.Context) error
}
