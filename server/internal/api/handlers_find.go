package api

import (
	"net/http"
	"strconv"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/jalilnawawi/chat-app/server/internal/hub"
	"github.com/jalilnawawi/chat-app/server/internal/store"
)

const (
	// maxSearchQueryLen dalam rune. Kueri yang lebih panjang dari ini adalah
	// kalimat yang ditempel, bukan sesuatu yang sedang dicari.
	maxSearchQueryLen = 200

	defaultSearchPage = 20
	maxSearchPage     = 50
)

// handleSearch mencari isi pesan, di satu percakapan atau di semuanya.
//
// Percakapan yang bukan milik pemanggil TIDAK dijawab 404 di sini, berbeda dari
// jalur riwayat: izinnya dievaluasi di dalam kuerinya sendiri, dan jawaban
// untuk percakapan orang lain adalah hasil kosong — persis seperti percakapan
// yang memang tidak memuat kata itu. Tidak ada yang bisa dipelajari dari
// perbedaan yang tidak ada.
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	me := userFrom(r.Context())
	q := r.URL.Query()

	text := q.Get("q")
	if utf8.RuneCountInString(text) > maxSearchQueryLen {
		writeError(w, http.StatusBadRequest, "kata pencarian terlalu panjang")
		return
	}

	query := store.SearchQuery{
		ViewerID: me.ID,
		Text:     text,
		Cursor:   q.Get("cursor"),
		Limit:    defaultSearchPage,
	}
	if v, err := strconv.Atoi(q.Get("limit")); err == nil && v > 0 {
		query.Limit = min(v, maxSearchPage)
	}
	if raw := q.Get("conversationId"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "id percakapan tidak valid")
			return
		}
		query.ConversationID = &id
	}

	res, err := s.store.SearchMessages(r.Context(), query)
	if err != nil {
		s.writeStoreError(w, err, "cari pesan")
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleListPins(w http.ResponseWriter, r *http.Request) {
	me := userFrom(r.Context())
	convID, ok := s.authorizedConversation(w, r)
	if !ok {
		return
	}

	pins, err := s.store.Pins(r.Context(), convID, me.ID)
	if err != nil {
		s.writeStoreError(w, err, "daftar sematan")
		return
	}
	writeJSON(w, http.StatusOK, pins)
}

func (s *Server) handlePin(w http.ResponseWriter, r *http.Request) {
	s.pin(w, r, true)
}

func (s *Server) handleUnpin(w http.ResponseWriter, r *http.Request) {
	s.pin(w, r, false)
}

// pin menyematkan atau melepas.
//
// Yang disiarkan cuma catatan sistemnya, lewat event pesan baru yang sudah
// ada. Tidak ada event "sematan berubah": catatan itu sendiri yang menjadi
// tanda bagi client untuk membaca ulang daftar sematan, dan dia juga yang
// menyusul lewat cursor `seq` bagi yang sedang offline. Dua kabar untuk satu
// perubahan berarti dua jalur yang bisa tidak sampai sendiri-sendiri.
//
// Seperti reaksi, sematan tidak membangunkan push notification.
func (s *Server) pin(w http.ResponseWriter, r *http.Request, pin bool) {
	me := userFrom(r.Context())

	msgID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "id pesan tidak valid")
		return
	}

	var change store.PinChange
	if pin {
		change, err = s.store.PinMessage(r.Context(), msgID, me.ID)
	} else {
		change, err = s.store.UnpinMessage(r.Context(), msgID, me.ID)
	}
	if err != nil {
		s.writeStoreError(w, err, "ubah sematan")
		return
	}

	if change.Changed {
		s.publishToMembers(r, change.ConversationID,
			hub.Event{Type: hub.EventMessageNew, Payload: change.Notice})
	}

	body := map[string]any{
		"conversationId": change.ConversationID,
		"messageId":      msgID,
		"pinned":         pin,
		"changed":        change.Changed,
	}
	if change.Changed {
		body["notice"] = change.Notice
	}
	writeJSON(w, http.StatusOK, body)
}
