package imaging

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"testing"
)

// denganEXIF menyisipkan segmen APP1 berisi satu tag orientasi ke dalam JPEG.
//
// Ini yang dilakukan kamera ponsel: pikselnya dibiarkan melintang sebagaimana
// sensornya terpasang, lalu satu angka dititipkan di metadata. Disusun sendiri
// di sini supaya ujinya tidak bergantung pada berkas contoh yang harus ikut
// disimpan di repositori.
func denganEXIF(jpg []byte, orientasi uint16, besarDulu bool) []byte {
	// AppendByteOrder, bukan ByteOrder: yang pertama yang punya AppendUintN.
	order := binary.AppendByteOrder(binary.LittleEndian)
	tanda := "II"
	if besarDulu {
		order, tanda = binary.BigEndian, "MM"
	}

	var tiff bytes.Buffer
	tiff.WriteString(tanda)
	tiff.Write(order.AppendUint16(nil, 42)) // penanda tetap TIFF
	tiff.Write(order.AppendUint32(nil, 8))  // IFD0 tepat setelah header

	tiff.Write(order.AppendUint16(nil, 1))         // satu entri
	tiff.Write(order.AppendUint16(nil, 0x0112))    // tag orientasi
	tiff.Write(order.AppendUint16(nil, 3))         // tipe SHORT
	tiff.Write(order.AppendUint32(nil, 1))         // satu nilai
	tiff.Write(order.AppendUint16(nil, orientasi)) // nilainya, muat di tempat
	tiff.Write([]byte{0, 0})                       // dua byte sisa
	tiff.Write(order.AppendUint32(nil, 0))         // tidak ada IFD berikutnya

	payload := append([]byte("Exif\x00\x00"), tiff.Bytes()...)

	var out bytes.Buffer
	out.Write(jpg[:2]) // SOI
	out.Write([]byte{0xFF, 0xE1})
	out.Write(binary.BigEndian.AppendUint16(nil, uint16(len(payload)+2)))
	out.Write(payload)
	out.Write(jpg[2:])
	return out.Bytes()
}

func TestReadOrientationMembacaKeduaUrutanByte(t *testing.T) {
	jpg := jpegBytes(t, gambarUji(64, 32, 255))

	for _, besarDulu := range []bool{false, true} {
		got := readOrientation(denganEXIF(jpg, 6, besarDulu))
		if got != 6 {
			t.Errorf("besarDulu=%v: orientasi = %d, mau 6", besarDulu, got)
		}
	}
}

func TestReadOrientationJatuhKeNormalTanpaEXIF(t *testing.T) {
	// JPEG tanpa EXIF, dan PNG yang memang tidak pernah membawanya.
	if got := readOrientation(jpegBytes(t, gambarUji(8, 8, 255))); got != orientasiNormal {
		t.Errorf("JPEG polos = %d, mau %d", got, orientasiNormal)
	}
	if got := readOrientation(pngBytes(t, gambarUji(8, 8, 255))); got != orientasiNormal {
		t.Errorf("PNG = %d, mau %d", got, orientasiNormal)
	}
}

// Seluruh pembacaan EXIF menyentuh byte kiriman orang lain. Berkas yang cacat
// atau sengaja dipotong harus berakhir sebagai "tidak ada orientasi", bukan
// sebagai panic yang menjatuhkan proses.
func TestReadOrientationTidakPanikPadaBerkasCacat(t *testing.T) {
	lengkap := denganEXIF(jpegBytes(t, gambarUji(16, 16, 255)), 6, false)

	cacat := [][]byte{
		nil,
		{0xFF},
		{0xFF, 0xD8},
		{0xFF, 0xD8, 0xFF, 0xE1},
		{0xFF, 0xD8, 0xFF, 0xE1, 0x00, 0x00}, // panjang segmen mustahil
		{0xFF, 0xD8, 0xFF, 0xE1, 0xFF, 0xFF, 'E', 'x'}, // panjang melewati berkas
		append([]byte{}, lengkap[:12]...),
		append([]byte{}, lengkap[:24]...),
		append([]byte{}, lengkap[:30]...),
		bytes.Repeat([]byte{0xFF}, 64),
	}

	for i, b := range cacat {
		if got := readOrientation(b); got < 1 || got > 8 {
			t.Errorf("kasus %d menghasilkan orientasi mustahil: %d", i, got)
		}
	}
}

func TestOrientationSwapsSidesHanyaUntukSeperempatPutaran(t *testing.T) {
	for o := 1; o <= 4; o++ {
		if orientationSwapsSides(o) {
			t.Errorf("orientasi %d seharusnya tidak menukar sisi", o)
		}
	}
	for o := 5; o <= 8; o++ {
		if !orientationSwapsSides(o) {
			t.Errorf("orientasi %d seharusnya menukar sisi", o)
		}
	}
}

// Ukuran yang dikirim ke client dipakai memesan ruang di layar sebelum
// gambarnya termuat. Kalau tertukar, daftar pesan justru melompat — persis hal
// yang ingin dicegah dengan menyimpan ukuran.
func TestMeasureMenukarSisiUntukFotoTegakDariPonsel(t *testing.T) {
	// Piksel tersimpan melintang 800x600, dengan titipan "putar seperempat".
	src := denganEXIF(jpegBytes(t, gambarUji(800, 600, 255)), 6, false)

	size, err := Measure(src, 50_000_000)
	if err != nil {
		t.Fatalf("Measure: %v", err)
	}
	if size.Width != 600 || size.Height != 800 {
		t.Errorf("ukuran = %dx%d, mau 600x800", size.Width, size.Height)
	}
}

func TestMakeMenghasilkanTurunanYangTegak(t *testing.T) {
	src := denganEXIF(jpegBytes(t, gambarUji(800, 600, 255)), 6, false)

	thumb, err := Make(src, 200, 50_000_000)
	if err != nil {
		t.Fatalf("Make: %v", err)
	}
	if thumb.Height <= thumb.Width {
		t.Errorf("turunan %dx%d masih rebah; seharusnya lebih tinggi dari lebar",
			thumb.Width, thumb.Height)
	}
}

// applyOrientation diuji langsung pada piksel, bukan lewat JPEG: enkode JPEG
// mengaburkan batas warna, dan yang ingin dibuktikan di sini adalah peta
// perpindahannya benar — bukan seberapa mirip warnanya setelah dikompresi.
func TestApplyOrientationMemindahkanPikselKeTempatYangBenar(t *testing.T) {
	// Satu piksel putih di pojok kiri atas, sisanya hitam.
	src := image.NewRGBA(image.Rect(0, 0, 4, 2))
	for y := range 2 {
		for x := range 4 {
			src.Set(x, y, color.RGBA{A: 255})
		}
	}
	src.Set(0, 0, color.RGBA{R: 255, G: 255, B: 255, A: 255})

	// Ke mana pojok kiri atas pindah, untuk tiap orientasi.
	mau := map[int][2]int{
		2: {3, 0}, // dicerminkan mendatar
		3: {3, 1}, // setengah putaran
		4: {0, 1}, // dicerminkan tegak
		5: {0, 0}, // diagonal utama
		6: {1, 0}, // seperempat searah jarum jam
		7: {1, 3}, // diagonal lawan
		8: {0, 3}, // seperempat berlawanan jarum jam
	}

	for o, titik := range mau {
		dst := applyOrientation(src, o)

		w, h := 4, 2
		if orientationSwapsSides(o) {
			w, h = 2, 4
		}
		if dst.Bounds().Dx() != w || dst.Bounds().Dy() != h {
			t.Errorf("orientasi %d: ukuran %dx%d, mau %dx%d",
				o, dst.Bounds().Dx(), dst.Bounds().Dy(), w, h)
			continue
		}

		r, g, b, _ := dst.At(titik[0], titik[1]).RGBA()
		if r != 0xFFFF || g != 0xFFFF || b != 0xFFFF {
			t.Errorf("orientasi %d: piksel penanda tidak sampai di (%d,%d)",
				o, titik[0], titik[1])
		}
	}
}

func TestApplyOrientationMembiarkanGambarNormalApaAdanya(t *testing.T) {
	src := gambarUji(4, 4, 255)
	if got := applyOrientation(src, orientasiNormal); got != src {
		t.Error("orientasi normal seharusnya mengembalikan gambar yang sama persis")
	}
}
