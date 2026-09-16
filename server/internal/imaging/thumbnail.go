// Package imaging membuat turunan kecil dari sebuah gambar.
//
// Satu-satunya alasan paket ini ada: foto dua belas megapiksel dari ponsel
// ditampilkan selebar tiga ratus piksel di dalam gelembung pesan, dan sebelum
// ini seluruh berkas beberapa megabyte itu harus diunduh utuh — oleh setiap
// anggota percakapan, setiap kali percakapannya dibuka.
//
// Yang TIDAK dilakukan paket ini: menyimpan, mengunggah, atau tahu apa pun
// tentang lampiran. Masukannya byte, keluarannya byte. Itu yang membuatnya bisa
// diuji tanpa database maupun penyimpanan objek.
package imaging

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"

	// Dekoder didaftarkan lewat efek samping. Yang TIDAK ada di sini sama
	// pentingnya dengan yang ada: tidak ada dekoder untuk format yang tidak
	// pernah disajikan inline, karena satu-satunya gambar yang perlu turunan
	// adalah gambar yang memang ditampilkan di dalam halaman.
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

// ErrTidakPerlu berarti gambarnya sudah cukup kecil. Bukan kegagalan: turunan
// yang lebih besar dari aslinya adalah kerja dan ruang yang terbuang.
var ErrTidakPerlu = errors.New("gambar sudah cukup kecil untuk dipakai apa adanya")

// ErrTerlaluBesar berarti dimensinya melewati batas yang pantas didekode.
//
// Ini penjagaan terhadap decompression bomb, dan batasnya PIKSEL, bukan byte.
// Berkas PNG seratus kilobyte bisa berisi kanvas 40.000 x 40.000 — sah menurut
// standar, dan enam gigabyte begitu dibentangkan di memori. Batas ukuran
// unggahan tidak menolongnya sama sekali, karena yang meledak bukan berkasnya
// melainkan hasil dekodenya.
var ErrTerlaluBesar = errors.New("dimensi gambar melewati batas yang wajar")

// Thumbnail adalah turunan yang sudah jadi, siap ditulis ke penyimpanan.
type Thumbnail struct {
	Bytes  []byte
	MIME   string
	Width  int
	Height int
}

// Size adalah ukuran gambar setelah orientasi EXIF diperhitungkan — yaitu
// ukuran sebagaimana AKAN TERLIHAT, bukan sebagaimana tersimpan.
type Size struct {
	Width  int
	Height int
}

// Measure membaca ukuran gambar tanpa mendekode satu piksel pun.
//
// image.DecodeConfig hanya membaca header, jadi ini murah bahkan untuk berkas
// besar — dan itu yang membuatnya bisa dipakai sebagai gerbang sebelum dekode
// penuh. Dipanggil juga untuk gambar yang tidak akan dibuatkan turunan, karena
// ukuran dari sini lebih layak dipercaya daripada ukuran yang diakui client.
func Measure(src []byte, maxPixels int) (Size, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(src))
	if err != nil {
		return Size{}, fmt.Errorf("baca header gambar: %w", err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return Size{}, errors.New("dimensi gambar tidak masuk akal")
	}
	if maxPixels > 0 && cfg.Width*cfg.Height > maxPixels {
		return Size{}, fmt.Errorf("%w: %dx%d", ErrTerlaluBesar, cfg.Width, cfg.Height)
	}

	// Orientasi EXIF menukar sisi panjang dan pendek untuk separuh nilainya.
	// Ukuran yang dikirim ke client dipakai memesan ruang di layar sebelum
	// gambarnya termuat; kalau tertukar, daftar pesan justru melompat — persis
	// hal yang ingin dicegah dengan menyimpan ukuran.
	if orientationSwapsSides(readOrientation(src)) {
		return Size{Width: cfg.Height, Height: cfg.Width}, nil
	}
	return Size{Width: cfg.Width, Height: cfg.Height}, nil
}

// Make membuat turunan yang sisi terpanjangnya paling banyak maxDim.
//
// maxPixels diperiksa lebih dulu lewat header, sebelum dekode penuh. Urutan itu
// yang penting — memeriksa setelah mendekode berarti ledakannya sudah terjadi.
func Make(src []byte, maxDim, maxPixels int) (Thumbnail, error) {
	size, err := Measure(src, maxPixels)
	if err != nil {
		return Thumbnail{}, err
	}
	if size.Width <= maxDim && size.Height <= maxDim {
		return Thumbnail{}, ErrTidakPerlu
	}

	img, _, err := image.Decode(bytes.NewReader(src))
	if err != nil {
		return Thumbnail{}, fmt.Errorf("dekode gambar: %w", err)
	}

	// Diperkecil lebih dulu, baru diputar. Urutan sebaliknya memutar gambar
	// berukuran penuh — kerja yang sama hasilnya dengan biaya dua belas
	// megapiksel, bukan seperempat megapiksel.
	small := scale(img, maxDim)
	small = applyOrientation(small, readOrientation(src))

	return encode(small)
}

// Normalize menyandikan ulang sebuah gambar dengan sisi terpanjang PALING
// BANYAK maxDim, dan tidak pernah mengembalikan ErrTidakPerlu.
//
// Bedanya dengan Make ada pada apa yang terjadi pada gambar yang sudah kecil.
// Make menolak bekerja untuknya, dan itu jawaban yang benar untuk turunan
// lampiran: berkas aslinya tetap tersimpan dan tetap bisa disajikan apa adanya.
// Foto profil tidak punya "aslinya" — yang diunggah dibuang begitu ini selesai —
// jadi menolak bekerja di sana berarti tidak ada apa pun yang tersimpan.
//
// Penyandian ulang juga membuang seluruh metadata yang menempel pada berkas
// aslinya, termasuk koordinat GPS di dalam EXIF sebuah foto. Tidak pernah ada
// alasannya sebuah titik koordinat ikut terpasang sebagai foto profil yang bisa
// diunduh siapa pun yang sudah login.
func Normalize(src []byte, maxDim, maxPixels int) (Thumbnail, error) {
	if _, err := Measure(src, maxPixels); err != nil {
		return Thumbnail{}, err
	}

	img, _, err := image.Decode(bytes.NewReader(src))
	if err != nil {
		return Thumbnail{}, fmt.Errorf("dekode gambar: %w", err)
	}

	// Diperkecil lebih dulu, baru diputar — urutan yang sama dengan Make, dan
	// dengan alasan yang sama.
	small := scale(img, maxDim)
	small = applyOrientation(small, readOrientation(src))
	return encode(small)
}

// scale menyesuaikan img sampai sisi terpanjangnya maxDim, dengan perbandingan
// sisi yang utuh — dan TIDAK PERNAH memperbesar.
//
// Penjagaan "tidak pernah memperbesar" ada karena Normalize: Make sudah menolak
// gambar yang lebih kecil dari maxDim sebelum sampai ke sini, tapi Normalize
// justru mengirimkannya. Tanpa penjagaan itu, foto profil 64 piksel akan
// dibentangkan jadi 256 piksel — berkas empat kali lebih besar yang isinya
// persis sama buramnya.
//
// CatmullRom, bukan ApproxBiLinear. Bedanya baru terasa pada pengecilan besar —
// dan pengecilan besar persis yang terjadi di sini, dari tiga ribu piksel ke
// empat ratus. Penapis bilinear pada faktor sebesar itu hanya mencicipi
// sebagian kecil piksel sumber, sehingga rambut, teks, dan garis halus pecah
// jadi bintik. Harganya beberapa puluh milidetik untuk satu foto, dan itu
// sebabnya pemanggilnya membatasi berapa yang boleh berjalan bersamaan.
func scale(img image.Image, maxDim int) *image.RGBA {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()

	switch {
	case w <= maxDim && h <= maxDim:
		// Sudah cukup kecil: ukurannya dipertahankan, dan yang tersisa hanyalah
		// penyandian ulang oleh pemanggil.
	case w >= h:
		h = max(1, h*maxDim/w)
		w = maxDim
	default:
		w = max(1, w*maxDim/h)
		h = maxDim
	}

	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, b, xdraw.Src, nil)
	return dst
}

// encode memilih format turunan dari apakah gambarnya punya bagian tembus
// pandang, bukan dari format aslinya.
//
// JPEG tidak mengenal alpha sama sekali: logo atau tangkapan layar bertepi
// transparan akan keluar dengan latar hitam pekat. PNG mengenalnya, tapi untuk
// foto ukurannya bisa sepuluh kali lipat JPEG pada mutu yang sama — dan seluruh
// alasan paket ini ada adalah ukuran.
//
// Jadi keduanya dipakai, dan yang menentukan adalah gambarnya sendiri. Opaque()
// memindai byte alpha; pada gambar sekecil ini biayanya tidak terukur.
func encode(img *image.RGBA) (Thumbnail, error) {
	var buf bytes.Buffer
	mime := "image/jpeg"

	if img.Opaque() {
		// Mutu 82: di atas itu berkasnya tumbuh jauh lebih cepat daripada
		// bedanya terlihat, apalagi pada gambar selebar empat ratus piksel.
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 82}); err != nil {
			return Thumbnail{}, fmt.Errorf("susun turunan jpeg: %w", err)
		}
	} else {
		mime = "image/png"
		enc := png.Encoder{CompressionLevel: png.DefaultCompression}
		if err := enc.Encode(&buf, img); err != nil {
			return Thumbnail{}, fmt.Errorf("susun turunan png: %w", err)
		}
	}

	return Thumbnail{
		Bytes:  buf.Bytes(),
		MIME:   mime,
		Width:  img.Bounds().Dx(),
		Height: img.Bounds().Dy(),
	}, nil
}
