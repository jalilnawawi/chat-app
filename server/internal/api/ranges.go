package api

import (
	"errors"
	"strconv"
	"strings"

	"github.com/jalilnawawi/chat-app/server/internal/blob"
)

// errRangeTidakTerpenuhi berarti rentangnya sah bentuknya tapi menunjuk ke luar
// berkas — satu-satunya keadaan yang wajib dijawab 416.
//
// Dibedakan dari "tidak ada rentang" karena akibatnya berlawanan: yang satu
// mengirim seluruh berkas, yang satu menolak mengirim apa pun. Menyamakan
// keduanya berarti pemutar video yang salah menghitung posisi diam-diam
// menerima berkas utuh dan merakitnya di tempat yang salah.
var errRangeTidakTerpenuhi = errors.New("rentang di luar ukuran berkas")

// parseRange menerjemahkan header Range menjadi rentang byte konkret.
//
// Hanya SATU rentang yang dilayani. Beberapa rentang sekaligus menuntut jawaban
// multipart/byteranges, dan tidak ada satu pun pemutar video atau browser yang
// memintanya untuk memutar berkas — yang memintanya adalah pengunduh, dan
// mengirim berkas utuh sudah menjawab mereka dengan benar.
//
// ok=false tanpa error berarti "layani seluruhnya". Itu jawaban yang sah untuk
// rentang apa pun: header Range adalah PERMINTAAN, bukan perintah, dan standar
// menuntut rentang yang bentuknya salah diabaikan, bukan ditolak.
func parseRange(header string, size int64) (blob.Range, bool, error) {
	const prefix = "bytes="

	spec, found := strings.CutPrefix(strings.TrimSpace(header), prefix)
	if !found {
		// Termasuk header kosong, dan satuan selain byte ("items=0-10").
		return blob.Range{}, false, nil
	}
	if strings.Contains(spec, ",") {
		return blob.Range{}, false, nil
	}

	first, last, ok := strings.Cut(spec, "-")
	if !ok {
		return blob.Range{}, false, nil
	}
	first, last = strings.TrimSpace(first), strings.TrimSpace(last)

	// Berkas kosong tidak punya satu byte pun yang bisa diminta. Tanpa cabang
	// ini, "bytes=0-" pada berkas nol byte menghasilkan rentang 0..-1.
	if size <= 0 {
		return blob.Range{}, false, errRangeTidakTerpenuhi
	}

	// "-500" berarti LIMA RATUS BYTE TERAKHIR, bukan mulai dari minus lima
	// ratus. Bentuk inilah yang dipakai pemutar video untuk membaca indeks MP4
	// yang tersimpan di ujung berkas — dan salah membacanya berarti video yang
	// diunggah dari ponsel tidak pernah bisa diputar sama sekali.
	if first == "" {
		n, err := strconv.ParseInt(last, 10, 64)
		if err != nil || n < 0 {
			return blob.Range{}, false, nil
		}
		if n == 0 {
			return blob.Range{}, false, errRangeTidakTerpenuhi
		}
		if n > size {
			n = size
		}
		return blob.Range{Start: size - n, End: size - 1}, true, nil
	}

	start, err := strconv.ParseInt(first, 10, 64)
	if err != nil || start < 0 {
		return blob.Range{}, false, nil
	}
	if start >= size {
		return blob.Range{}, false, errRangeTidakTerpenuhi
	}

	// Ujung yang kosong berarti sampai akhir berkas. Ini bentuk yang dikirim
	// browser saat memulai pemutaran: "bytes=0-".
	end := size - 1
	if last != "" {
		if end, err = strconv.ParseInt(last, 10, 64); err != nil {
			return blob.Range{}, false, nil
		}
		// Ujung yang melewati akhir berkas dipangkas, bukan ditolak — client
		// yang meminta sejuta byte dari berkas seratus byte sedang meminta
		// "sisanya", dan itu permintaan yang masuk akal.
		if end >= size {
			end = size - 1
		}
	}
	if end < start {
		return blob.Range{}, false, nil
	}

	return blob.Range{Start: start, End: end}, true, nil
}

// contentRange menyusun header Content-Range untuk jawaban 206.
func contentRange(start, end, total int64) string {
	return "bytes " + strconv.FormatInt(start, 10) + "-" +
		strconv.FormatInt(end, 10) + "/" + strconv.FormatInt(total, 10)
}
