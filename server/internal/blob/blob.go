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

// Range adalah potongan yang diminta dari sebuah objek, dengan kedua ujungnya
// INKLUSIF — bentuk yang sama dengan header HTTP Range, supaya tidak ada
// konversi diam-diam di antara permintaan browser dan permintaan ke penyimpanan.
//
// Ada karena video. Pemutar video tidak pernah mengunduh berkasnya dari awal
// sampai akhir: dia meminta beberapa ratus kilobyte pertama untuk membaca
// indeksnya, lalu melompat ke posisi yang diklik orang. Tanpa jalur ini, satu
// klik pada menit kesepuluh berarti menunggu sembilan menit pertama terunduh
// lebih dulu.
type Range struct {
	Start int64
	End   int64
}

// Length adalah banyaknya byte yang dicakup rentang ini.
func (r Range) Length() int64 { return r.End - r.Start + 1 }

// Object adalah isi sebuah lampiran — seluruhnya, atau sepotong.
type Object struct {
	Body io.ReadCloser

	// Size adalah panjang yang akan keluar dari Body, dan Total adalah ukuran
	// penuh objeknya. Keduanya sama bila yang diminta memang seluruhnya.
	// Masing-masing -1 bila penyimpanan tidak menyebutkannya.
	Size  int64
	Total int64

	// Partial menandai bahwa penyimpanan BENAR-BENAR menjawab sepotong.
	//
	// Dipisahkan dari "kita meminta sepotong" karena keduanya bisa berbeda:
	// Range adalah permintaan yang boleh diabaikan, dan penyimpanan yang tidak
	// mendukungnya akan menjawab berkas utuh dengan status 200. Menganggap
	// jawaban itu sepotong berarti mengirim 206 dengan Content-Range yang
	// berbohong — dan pemutar video yang mempercayainya akan merakit berkas
	// yang isinya tumpang tindih.
	Partial bool
}

type Store interface {
	// Put menulis isi r ke key. size boleh -1 bila tidak diketahui.
	Put(ctx context.Context, key, contentType string, r io.Reader, size int64) error

	// Get membuka isi key, seluruhnya bila rng nil. Pemanggil wajib menutup
	// Object.Body.
	Get(ctx context.Context, key string, rng *Range) (Object, error)

	// Delete membuang key. Menghapus key yang sudah tidak ada bukan error:
	// pembersih lampiran yatim berjalan berulang dan harus aman diulang.
	Delete(ctx context.Context, key string) error

	// Ping memastikan penyimpanan bisa dijangkau. Dipakai probe kesiapan.
	Ping(ctx context.Context) error
}
