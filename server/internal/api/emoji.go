package api

import (
	"unicode/utf8"
)

// Validasi emoji reaksi.
//
// Kolom `emoji` adalah teks bebas, dan teks bebas yang ditampilkan di samping
// pesan orang lain adalah pesan kedua yang menyamar jadi reaksi. Tanpa
// pemeriksaan di sini, "reaksi" bisa berisi kalimat, tautan, atau spasi kosong
// yang tak terlihat tapi tetap menghasilkan tombol di layar semua anggota.
//
// Yang dipaksakan ada dua, dan keduanya perlu:
//
//  1. Setiap rune harus termasuk simbol yang memang dipakai sebagai emoji —
//     daftar IZIN, bukan daftar larangan. Daftar larangan pada Unicode selalu
//     ketinggalan satu blok.
//  2. Hasilnya harus SATU grafem: satu hal yang terlihat, bukan deretan.
//     "👍👍👍" adalah tiga, dan tiga emoji dalam satu tombol adalah cara
//     menyelundupkan panjang.
//
// Go tidak punya pemecah grafem UAX#29 di pustaka standarnya, dan menambah
// dependensi untuk satu pemeriksaan masukan terasa mahal. Yang dipakai di sini
// adalah aturan yang menutup bentuk-bentuk emoji yang nyata: rangkaian ZWJ,
// pengubah warna kulit, penanda variasi, keycap, dan pasangan bendera. Ia
// sengaja lebih KETAT daripada UAX#29 — emoji yang sangat baru mungkin ditolak,
// dan itu kegagalan yang benar arahnya.

// maxEmojiRunes di bawah batas CHECK database (24 karakter), supaya yang lolos
// di sini tidak pernah ditolak lagi satu lapis di bawahnya. Rangkaian terpanjang
// yang wajar — keluarga berempat dengan warna kulit — adalah sebelas rune.
const maxEmojiRunes = 16

const (
	zwj             = 0x200D // penyambung rangkaian: 👨‍👩‍👧
	variationText   = 0xFE0E
	variationEmoji  = 0xFE0F
	combiningKeycap = 0x20E3 // yang membuat "1" jadi "1️⃣"
)

// validReaction melaporkan apakah s layak dipakai sebagai reaksi.
func validReaction(s string) bool {
	if s == "" || !utf8.ValidString(s) || utf8.RuneCountInString(s) > maxEmojiRunes {
		return false
	}

	var (
		bases    int  // hal yang terlihat, sebelum disambung
		joiners  int  // ZWJ
		regional int  // huruf bendera
		keycap   bool // ada U+20E3
		digit    bool // ada angka/#/* yang menunggu keycap
	)

	for _, r := range s {
		switch {
		case !emojiRune(r):
			return false

		case r == zwj:
			joiners++

		case r == variationText || r == variationEmoji || isSkinTone(r):
			// Pengubah: menempel pada rune sebelumnya, tidak menambah grafem.

		case r == combiningKeycap:
			keycap = true

		case isRegionalIndicator(r):
			regional++

		case isKeycapBase(r):
			digit = true

		default:
			bases++
		}
	}

	// Angka, "#", dan "*" hanya boleh muncul sebagai dasar keycap. Tanpa syarat
	// ini, "7" polos lolos sebagai reaksi.
	if digit != keycap {
		return false
	}
	if digit {
		return bases == 0 && regional == 0 && joiners == 0
	}

	// Bendera adalah tepat DUA huruf regional dan tidak boleh bercampur dengan
	// apa pun. Satu huruf bukan bendera, tiga huruf adalah bendera plus sisa.
	if regional > 0 {
		return regional == 2 && bases == 0 && joiners == 0
	}

	// Sisanya: rangkaian ZWJ menyatukan n hal terlihat dengan n-1 penyambung.
	// Dua emoji yang ditempel tanpa penyambung menghasilkan bases=2, joiners=0,
	// dan ditolak di sini — persis yang diinginkan.
	return bases >= 1 && bases == joiners+1
}

func isSkinTone(r rune) bool          { return r >= 0x1F3FB && r <= 0x1F3FF }
func isRegionalIndicator(r rune) bool { return r >= 0x1F1E6 && r <= 0x1F1FF }
func isKeycapBase(r rune) bool        { return r == '#' || r == '*' || (r >= '0' && r <= '9') }

// emojiRune adalah daftar izinnya. Blok-blok di bawah adalah tempat simbol yang
// dipakai orang sebagai reaksi benar-benar tinggal.
func emojiRune(r rune) bool {
	switch {
	case r == zwj, r == variationText, r == variationEmoji, r == combiningKeycap:
		return true
	case isKeycapBase(r):
		return true
	case r == 0x00A9, r == 0x00AE, r == 0x203C, r == 0x2049,
		r == 0x2122, r == 0x2139, r == 0x3030, r == 0x303D,
		r == 0x3297, r == 0x3299:
		return true
	case r >= 0x2190 && r <= 0x21FF: // panah
		return true
	case r >= 0x2300 && r <= 0x23FF: // ⌚ ⏰ ⏳
		return true
	case r >= 0x25A0 && r <= 0x25FF: // bentuk geometris
		return true
	case r >= 0x2600 && r <= 0x27BF: // simbol lain-lain + dingbats: ☀ ✅ ❤
		return true
	case r >= 0x2934 && r <= 0x2935:
		return true
	case r >= 0x2B00 && r <= 0x2BFF:
		return true
	case r >= 0x1F000 && r <= 0x1FAFF: // blok emoji utama
		return true
	default:
		return false
	}
}
