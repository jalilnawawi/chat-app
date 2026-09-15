package imaging

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

// gambarUji membuat gambar berukuran tertentu dengan separuh kiri merah dan
// separuh kanan biru — asimetris, supaya perputaran bisa dibuktikan, bukan
// hanya ukurannya yang dicocokkan.
func gambarUji(w, h int, alpha uint8) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			c := color.RGBA{R: 220, A: alpha}
			if x >= w/2 {
				c = color.RGBA{B: 220, A: alpha}
			}
			img.Set(x, y, c)
		}
	}
	return img
}

func pngBytes(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png.Encode: %v", err)
	}
	return buf.Bytes()
}

func jpegBytes(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("jpeg.Encode: %v", err)
	}
	return buf.Bytes()
}

func TestMakeMemperkecilSambilMenjagaPerbandinganSisi(t *testing.T) {
	src := pngBytes(t, gambarUji(1600, 800, 255))

	thumb, err := Make(src, 400, 50_000_000)
	if err != nil {
		t.Fatalf("Make: %v", err)
	}

	if thumb.Width != 400 || thumb.Height != 200 {
		t.Errorf("ukuran = %dx%d, mau 400x200", thumb.Width, thumb.Height)
	}
	// Seluruh alasan paket ini ada adalah ukuran. Turunan yang tidak lebih kecil
	// dari aslinya berarti kerjanya sia-sia.
	if len(thumb.Bytes) >= len(src) {
		t.Errorf("turunan %d byte tidak lebih kecil dari asli %d byte", len(thumb.Bytes), len(src))
	}
}

// Sisi terpanjang yang menentukan, bukan lebar. Gambar tegak yang diperkecil
// mengikuti lebarnya akan tetap jauh lebih tinggi dari batas.
func TestMakeMengikutiSisiTerpanjang(t *testing.T) {
	thumb, err := Make(pngBytes(t, gambarUji(300, 1200, 255)), 240, 50_000_000)
	if err != nil {
		t.Fatalf("Make: %v", err)
	}
	if thumb.Height != 240 || thumb.Width != 60 {
		t.Errorf("ukuran = %dx%d, mau 60x240", thumb.Width, thumb.Height)
	}
}

func TestMakeMelewatiGambarYangSudahKecil(t *testing.T) {
	_, err := Make(pngBytes(t, gambarUji(200, 100, 255)), 480, 50_000_000)
	if !errors.Is(err, ErrTidakPerlu) {
		t.Fatalf("err = %v, mau ErrTidakPerlu", err)
	}
}

// Gambar yang persis sebesar batas juga tidak perlu diperkecil. Batas ini
// paling mudah salah satu piksel ke arah yang salah, dan akibatnya adalah
// turunan seukuran aslinya yang disimpan selamanya untuk semua orang.
func TestMakeMelewatiGambarYangPersisSebesarBatas(t *testing.T) {
	_, err := Make(pngBytes(t, gambarUji(480, 480, 255)), 480, 50_000_000)
	if !errors.Is(err, ErrTidakPerlu) {
		t.Fatalf("err = %v, mau ErrTidakPerlu", err)
	}
}

// Foto tanpa bagian tembus pandang jadi JPEG; yang punya jadi PNG. Salah
// memilih berarti logo bertepi transparan keluar dengan latar hitam pekat.
func TestMakeMemilihFormatDariAdaTidaknyaAlpha(t *testing.T) {
	padat, err := Make(pngBytes(t, gambarUji(1000, 1000, 255)), 200, 50_000_000)
	if err != nil {
		t.Fatalf("Make padat: %v", err)
	}
	if padat.MIME != "image/jpeg" {
		t.Errorf("gambar padat jadi %s, mau image/jpeg", padat.MIME)
	}

	tembus, err := Make(pngBytes(t, gambarUji(1000, 1000, 128)), 200, 50_000_000)
	if err != nil {
		t.Fatalf("Make tembus: %v", err)
	}
	if tembus.MIME != "image/png" {
		t.Errorf("gambar tembus pandang jadi %s, mau image/png", tembus.MIME)
	}
}

// pngHeaderRaksasa menyusun PNG yang HANYA berisi header, dengan ukuran kanvas
// yang mustahil. Panjangnya beberapa puluh byte; hasil dekodenya berukuran
// gigabyte. Persis bentuk serangan yang tidak tersentuh sama sekali oleh batas
// ukuran unggahan.
func pngHeaderRaksasa(w, h uint32) []byte {
	var b bytes.Buffer
	b.WriteString("\x89PNG\r\n\x1a\n")

	// Isi chunk IHDR: lebar, tinggi, lalu lima byte penjelas format.
	isi := make([]byte, 0, 17)
	isi = append(isi, "IHDR"...)
	isi = binary.BigEndian.AppendUint32(isi, w)
	isi = binary.BigEndian.AppendUint32(isi, h)
	isi = append(isi, 8, 6, 0, 0, 0) // kedalaman bit, tipe warna, dst

	// CRC-nya harus benar, kalau tidak dekoder menolak berkasnya sebelum
	// sempat membaca ukuran — dan ujinya jadi membuktikan hal yang lain.
	b.Write(binary.BigEndian.AppendUint32(nil, uint32(len(isi)-4)))
	b.Write(isi)
	b.Write(binary.BigEndian.AppendUint32(nil, crc32.ChecksumIEEE(isi)))
	return b.Bytes()
}

func TestMakeMenolakGambarYangTerlaluBanyakPikselnya(t *testing.T) {
	// 40.000 x 40.000 = 1,6 miliar piksel, sekitar enam gigabyte saat
	// dibentangkan. Berkasnya sendiri kurang dari seratus byte.
	src := pngHeaderRaksasa(40000, 40000)

	if len(src) > 100 {
		t.Fatalf("berkas ujinya harus mungil, malah %d byte", len(src))
	}
	_, err := Make(src, 480, 50_000_000)
	if !errors.Is(err, ErrTerlaluBesar) {
		t.Fatalf("err = %v, mau ErrTerlaluBesar", err)
	}
}

func TestMakeMenolakBerkasYangBukanGambar(t *testing.T) {
	if _, err := Make([]byte("ini teks biasa, bukan gambar"), 480, 50_000_000); err == nil {
		t.Fatal("berkas sembarang diterima sebagai gambar")
	}
}

func TestMeasureMembacaUkuranTanpaMendekode(t *testing.T) {
	size, err := Measure(pngBytes(t, gambarUji(640, 360, 255)), 50_000_000)
	if err != nil {
		t.Fatalf("Measure: %v", err)
	}
	if size.Width != 640 || size.Height != 360 {
		t.Errorf("ukuran = %dx%d, mau 640x360", size.Width, size.Height)
	}
}

func TestMakeTahanTerhadapJPEGYangDipotong(t *testing.T) {
	utuh := jpegBytes(t, gambarUji(800, 600, 255))
	potong := utuh[:len(utuh)/3]

	// Yang penting bukan hasilnya, melainkan bahwa berkas cacat berakhir
	// sebagai error — bukan sebagai panic yang menjatuhkan seluruh proses.
	if _, err := Make(potong, 200, 50_000_000); err == nil {
		t.Log("JPEG terpotong masih bisa didekode sebagian; tidak apa-apa")
	}
}
