package main

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

// Pesan uji membawa waktu kirimnya di dalam badan pesan sendiri:
//
//	probe:<runID>:<unix nano>:<bantalan>
//
// Menaruh stempel waktu di dalam isi pesan, bukan di tabel terpisah di sisi
// pengirim, membuat pengukuran ikut menempuh SELURUH jalur yang ditempuh pesan
// sungguhan — REST, Postgres, hub, WebSocket — dan tetap benar walau pesannya
// sampai ke penerima dengan urutan berbeda atau lewat instance yang lain.
const probePrefix = "probe:"

func makeProbe(runID string, at time.Time, size int) string {
	head := probePrefix + runID + ":" + strconv.FormatInt(at.UnixNano(), 10) + ":"
	if pad := size - len(head); pad > 0 {
		return head + strings.Repeat("x", pad)
	}
	return head
}

// parseProbe mengambil waktu kirim dari satu event WebSocket mentah.
func parseProbe(event []byte, runID string) (time.Time, bool) {
	// Saringan murah lebih dulu: sebagian besar byte yang lewat adalah presence,
	// typing, dan read receipt yang tidak perlu di-decode sama sekali.
	marker := probePrefix + runID + ":"
	if !bytes.Contains(event, []byte(marker)) {
		return time.Time{}, false
	}

	var envelope struct {
		Type    string `json:"type"`
		Payload struct {
			Body string `json:"body"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(event, &envelope); err != nil {
		return time.Time{}, false
	}
	if envelope.Type != "message.new" {
		return time.Time{}, false
	}

	rest, ok := strings.CutPrefix(envelope.Payload.Body, marker)
	if !ok {
		return time.Time{}, false
	}
	stamp, _, _ := strings.Cut(rest, ":")

	nanos, err := strconv.ParseInt(stamp, 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	return time.Unix(0, nanos), true
}
