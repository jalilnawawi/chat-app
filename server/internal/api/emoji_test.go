package api

import "testing"

func TestValidReaction(t *testing.T) {
	boleh := []struct{ name, in string }{
		{"jempol", "👍"},
		{"hati dengan penanda variasi", "❤️"},
		{"hati tanpa penanda", "❤"},
		{"tertawa", "😂"},
		{"api", "🔥"},
		{"centang dingbat", "✅"},
		{"jempol berwarna kulit", "👍🏽"},
		{"rangkaian ZWJ", "👨‍👩‍👧"},
		{"bendera dua huruf regional", "🇮🇩"},
		{"keycap angka", "1️⃣"},
		{"keycap tagar", "#️⃣"},
	}
	for _, c := range boleh {
		if !validReaction(c.in) {
			t.Errorf("%s (%q) seharusnya diterima", c.name, c.in)
		}
	}

	tolak := []struct{ name, in string }{
		{"kosong", ""},
		// Inti dari pemeriksaan ini: kolom teks bebas di samping pesan orang
		// lain adalah pesan kedua yang menyamar jadi reaksi.
		{"kalimat", "setuju banget"},
		{"satu huruf", "a"},
		{"angka tanpa keycap", "7"},
		{"spasi", " "},
		{"tautan", "https://contoh.id"},
		{"emoji dengan teks", "👍 mantap"},

		// Dua emoji yang ditempel tanpa penyambung adalah DUA grafem. Tanpa
		// aturan ini, satu tombol reaksi bisa memuat sederet emoji dan dipakai
		// sebagai cara menyelundupkan panjang.
		{"dua emoji", "👍👍"},
		{"tiga emoji", "👍🔥😂"},
		{"satu huruf regional", "🇮"},
		{"tiga huruf regional", "🇮🇩🇦"},

		{"karakter kendali", "\x01"},
		{"spasi lebar nol", "\u200b"},
		{"terlalu panjang", "👨‍👩‍👧‍👦👨‍👩‍👧‍👦👨‍👩‍👧‍👦"},
	}
	for _, c := range tolak {
		if validReaction(c.in) {
			t.Errorf("%s (%q) seharusnya ditolak", c.name, c.in)
		}
	}
}

// Yang lolos di sini tidak boleh ditolak lagi oleh CHECK di database — dua
// lapis pemeriksaan yang batasnya berbeda berarti satu kegagalan yang muncul
// hanya di produksi, sebagai kesalahan server, untuk masukan yang sudah
// dinyatakan sah.
func TestPanjangEmojiDiBawahBatasDatabase(t *testing.T) {
	const batasDatabase = 24 // char_length(emoji) BETWEEN 1 AND 24 di 0005
	if maxEmojiRunes > batasDatabase {
		t.Fatalf("batas aplikasi (%d rune) melebihi batas database (%d)",
			maxEmojiRunes, batasDatabase)
	}
}
