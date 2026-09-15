package api

import (
	"errors"
	"testing"
)

func TestParseRangeMembacaBentukYangDikirimBrowser(t *testing.T) {
	const size = 1000

	cases := []struct {
		header     string
		start, end int64
		nama       string
	}{
		{"bytes=0-499", 0, 499, "rentang tertutup"},
		{"bytes=500-999", 500, 999, "sampai byte terakhir"},
		{"bytes=0-", 0, 999, "ujung terbuka — bentuk saat pemutaran dimulai"},
		{"bytes=900-", 900, 999, "melanjutkan dari tengah"},
		{"bytes=0-0", 0, 0, "satu byte"},
		{"bytes=999-999", 999, 999, "byte terakhir saja"},
		// Ujung yang melewati akhir berkas adalah cara wajar mengatakan
		// "sisanya", bukan kesalahan.
		{"bytes=990-99999", 990, 999, "ujung dipangkas ke ukuran berkas"},
		{"  bytes=10-20  ", 10, 20, "spasi di sekeliling header"},
	}

	for _, c := range cases {
		rng, ok, err := parseRange(c.header, size)
		if err != nil || !ok {
			t.Errorf("%s (%q): ok=%v err=%v", c.nama, c.header, ok, err)
			continue
		}
		if rng.Start != c.start || rng.End != c.end {
			t.Errorf("%s (%q): %d-%d, mau %d-%d",
				c.nama, c.header, rng.Start, rng.End, c.start, c.end)
		}
	}
}

// "-500" berarti lima ratus byte TERAKHIR. Bentuk inilah yang dipakai pemutar
// video untuk membaca indeks MP4 yang tersimpan di ujung berkas — salah
// membacanya berarti video dari ponsel tidak pernah bisa diputar sama sekali.
func TestParseRangeMembacaBentukSufiks(t *testing.T) {
	cases := []struct {
		header     string
		size       int64
		start, end int64
	}{
		{"bytes=-500", 1000, 500, 999},
		{"bytes=-1", 1000, 999, 999},
		// Sufiks yang lebih panjang dari berkasnya berarti seluruh berkas.
		{"bytes=-5000", 1000, 0, 999},
	}

	for _, c := range cases {
		rng, ok, err := parseRange(c.header, c.size)
		if err != nil || !ok {
			t.Errorf("%q: ok=%v err=%v", c.header, ok, err)
			continue
		}
		if rng.Start != c.start || rng.End != c.end {
			t.Errorf("%q: %d-%d, mau %d-%d", c.header, rng.Start, rng.End, c.start, c.end)
		}
	}
}

// Rentang yang bentuknya salah harus DIABAIKAN, bukan ditolak: header Range
// adalah permintaan, bukan perintah, dan mengirim seluruh berkas selalu jawaban
// yang sah. Menolaknya dengan 416 akan mematahkan client yang sebenarnya cuma
// mengirim sesuatu yang tidak kita kenali.
func TestParseRangeMengabaikanBentukYangTidakDikenali(t *testing.T) {
	diabaikan := []string{
		"",
		"bytes=",
		"bytes=abc-def",
		"bytes=-abc",
		"bytes=10-5",           // ujung sebelum awal
		"bytes=-10-20",         // awal negatif
		"items=0-10",           // satuan selain byte
		"0-100",                // tanpa satuan
		"bytes=0-100, 200-300", // beberapa rentang sekaligus
	}

	for _, h := range diabaikan {
		rng, ok, err := parseRange(h, 1000)
		if ok || err != nil {
			t.Errorf("%q: ok=%v err=%v rng=%+v — seharusnya diabaikan diam-diam", h, ok, err, rng)
		}
	}
}

// Yang menunjuk ke luar berkas justru WAJIB ditolak. Menyamakannya dengan
// "abaikan" berarti pemutar video yang salah menghitung posisi diam-diam
// menerima berkas utuh dan merakitnya di tempat yang salah.
func TestParseRangeMenolakYangDiLuarUkuran(t *testing.T) {
	cases := []struct {
		header string
		size   int64
	}{
		{"bytes=1000-1010", 1000}, // mulai tepat di akhir berkas
		{"bytes=5000-", 1000},
		{"bytes=-0", 1000}, // nol byte terakhir tidak menunjuk apa pun
		{"bytes=0-", 0},    // berkas kosong tidak punya byte untuk diminta
		{"bytes=-10", 0},
	}

	for _, c := range cases {
		_, ok, err := parseRange(c.header, c.size)
		if ok || !errors.Is(err, errRangeTidakTerpenuhi) {
			t.Errorf("%q pada %d byte: ok=%v err=%v, mau errRangeTidakTerpenuhi",
				c.header, c.size, ok, err)
		}
	}
}

func TestContentRangeMenyusunBentukYangDitungguClient(t *testing.T) {
	if got := contentRange(0, 499, 1000); got != "bytes 0-499/1000" {
		t.Errorf("hasil = %q", got)
	}
}
