package imaging

import (
	"encoding/binary"
	"image"
)

// Orientasi EXIF: satu angka kecil yang menentukan gambarnya terlihat tegak
// atau miring, dan yang paling mudah dilupakan.
//
// Kamera ponsel hampir tidak pernah memutar piksel saat memotret. Dia menyimpan
// gambarnya apa adanya — melintang, sebagaimana sensornya terpasang — lalu
// menitipkan satu angka di metadata yang artinya "putar segini sebelum
// ditampilkan". Browser modern MENURUTI angka itu secara bawaan.
//
// Akibatnya, kalau turunan dibuat tanpa memperhatikannya: berkas aslinya tampil
// tegak di layar (browser memutarnya), sedangkan turunannya tampil rebah —
// karena dekoder Go menyerahkan piksel mentah, dan hasil enkode ulang kita
// tidak lagi membawa metadata apa pun untuk diikuti browser. Bukan turunan yang
// buram atau salah ukuran, melainkan turunan yang MIRING: satu-satunya cacat di
// fase ini yang langsung terlihat siapa pun yang mengirim foto dari ponsel.
//
// Paket exif utuh tidak dibutuhkan untuk ini. Yang dicari cuma satu tag, dan
// jalan menuju tag itu pendek.
const (
	orientasiNormal = 1
	tagOrientasi    = 0x0112
)

// readOrientation mengembalikan nilai orientasi EXIF, atau orientasiNormal bila
// tidak ada — termasuk untuk PNG, GIF, dan WebP, yang memang tidak membawanya.
//
// Seluruh fungsi ini membaca byte kiriman orang lain, jadi setiap langkah
// memeriksa panjang lebih dulu. Berkas yang cacat atau sengaja dipotong harus
// berakhir sebagai "tidak ada orientasi", bukan sebagai panic.
func readOrientation(src []byte) int {
	app1 := findEXIF(src)
	if app1 == nil {
		return orientasiNormal
	}
	return orientationFromTIFF(app1)
}

// findEXIF menelusuri segmen JPEG sampai menemukan APP1 yang berisi EXIF.
func findEXIF(src []byte) []byte {
	if len(src) < 4 || src[0] != 0xFF || src[1] != 0xD8 {
		return nil // bukan JPEG; format lain tidak membawa orientasi
	}

	for i := 2; i+4 <= len(src); {
		if src[i] != 0xFF {
			return nil // bukan awal segmen: berkasnya cacat, berhenti
		}

		marker := src[i+1]
		// SOS (0xDA) menandai awal data gambar, dan EXIF selalu sebelum itu.
		// EOI (0xD9) berarti berkasnya habis.
		if marker == 0xDA || marker == 0xD9 {
			return nil
		}

		// Panjang segmen sudah termasuk dua byte panjangnya sendiri, jadi
		// apa pun di bawah dua adalah berkas yang cacat — dan membiarkannya
		// lewat berarti penelusuran ini tidak pernah maju.
		length := int(binary.BigEndian.Uint16(src[i+2 : i+4]))
		if length < 2 || i+2+length > len(src) {
			return nil
		}

		if marker == 0xE1 {
			payload := src[i+4 : i+2+length]
			const magic = "Exif\x00\x00"
			if len(payload) > len(magic) && string(payload[:len(magic)]) == magic {
				return payload[len(magic):]
			}
		}

		i += 2 + length
	}
	return nil
}

// orientationFromTIFF membaca IFD0 dan mencari satu tag di dalamnya.
//
// Blok EXIF berbentuk TIFF, dan TIFF menyimpan urutan byte-nya sendiri di dua
// karakter pertama: "II" untuk little-endian, "MM" untuk big-endian. Keduanya
// benar-benar dipakai di alam liar — Canon menulis yang pertama, Apple yang
// kedua — jadi menebak salah satunya berarti separuh foto terbaca kacau.
func orientationFromTIFF(b []byte) int {
	if len(b) < 8 {
		return orientasiNormal
	}

	var order binary.ByteOrder
	switch string(b[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return orientasiNormal
	}

	if order.Uint16(b[2:4]) != 42 { // penanda tetap TIFF
		return orientasiNormal
	}

	// Offset dihitung dari awal blok TIFF, bukan dari awal berkas.
	ifd := int(order.Uint32(b[4:8]))
	if ifd < 8 || ifd+2 > len(b) {
		return orientasiNormal
	}

	count := int(order.Uint16(b[ifd : ifd+2]))
	for i := range count {
		entry := ifd + 2 + i*12
		if entry+12 > len(b) {
			return orientasiNormal
		}
		if order.Uint16(b[entry:entry+2]) != tagOrientasi {
			continue
		}

		// Tipe SHORT dengan satu nilai muat di dalam field 4 byte-nya sendiri,
		// jadi tidak ada offset yang perlu diikuti. Nilainya menempati dua
		// byte pertama, dan dua sisanya diisi apa saja.
		v := int(order.Uint16(b[entry+8 : entry+10]))
		if v >= 1 && v <= 8 {
			return v
		}
		return orientasiNormal
	}
	return orientasiNormal
}

// orientationSwapsSides melaporkan apakah orientasi ini menukar lebar dengan
// tinggi. Nilai 5 sampai 8 semuanya memuat seperempat putaran.
func orientationSwapsSides(o int) bool { return o >= 5 && o <= 8 }

// applyOrientation memutar atau mencerminkan gambar sesuai orientasi EXIF-nya.
//
// Dikerjakan SETELAH gambarnya diperkecil, jadi yang dipindahkan paling banyak
// beberapa ratus ribu piksel alih-alih dua belas juta.
func applyOrientation(src *image.RGBA, o int) *image.RGBA {
	if o <= orientasiNormal || o > 8 {
		return src
	}

	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	dw, dh := w, h
	if orientationSwapsSides(o) {
		dw, dh = h, w
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))

	// Tiap cabang menjawab pertanyaan yang sama: piksel tujuan (x, y) ini
	// diambil dari piksel sumber yang mana. Menuliskannya sebagai pemetaan
	// mundur — bukan maju — memastikan setiap piksel tujuan terisi tepat sekali,
	// tanpa lubang yang muncul dari pembulatan.
	for y := range dh {
		for x := range dw {
			var sx, sy int
			switch o {
			case 2: // dicerminkan mendatar
				sx, sy = w-1-x, y
			case 3: // diputar setengah putaran
				sx, sy = w-1-x, h-1-y
			case 4: // dicerminkan tegak
				sx, sy = x, h-1-y
			case 5: // dicerminkan pada diagonal utama
				sx, sy = y, x
			case 6: // diputar seperempat searah jarum jam
				sx, sy = y, h-1-x
			case 7: // dicerminkan pada diagonal lawan
				sx, sy = w-1-y, h-1-x
			case 8: // diputar seperempat berlawanan jarum jam
				sx, sy = w-1-y, x
			}
			copy(
				dst.Pix[dst.PixOffset(x, y):][:4],
				src.Pix[src.PixOffset(sx, sy):][:4],
			)
		}
	}
	return dst
}
