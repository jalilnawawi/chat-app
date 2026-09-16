package api

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/jalilnawawi/chat-app/server/internal/hub"
	"github.com/jalilnawawi/chat-app/server/internal/store"
)

// Pengelolaan grup.
//
// Kelima handler di bawah menjalankan tindakan yang berbeda, tapi menyiarkan
// hal yang sama persis — dan itu disengaja. Lihat broadcastGroupChange: satu
// tempat yang tahu siapa harus diberi tahu tentang apa, sehingga menambah
// tindakan keenam tidak berarti menebak ulang jawabannya.
//
// Tidak satu pun memanggil notifyOffline. Perubahan keanggotaan bukan pesan
// yang ditujukan kepada seseorang, dan membangunkan ponsel dua ratus orang
// karena judul grup diganti adalah cara membuat notifikasi aplikasi ini
// dimatikan orang — alasan yang sama dengan reaksi di Fase 9.

func (s *Server) handleRenameGroup(w http.ResponseWriter, r *http.Request) {
	me := userFrom(r.Context())
	convID, ok := s.authorizedConversation(w, r)
	if !ok {
		return
	}

	var req struct {
		Title string `json:"title"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "body tidak valid")
		return
	}

	title, err := store.NormalizeGroupTitle(req.Title)
	if err != nil {
		s.writeStoreError(w, err, "judul grup")
		return
	}

	change, err := s.store.RenameGroup(r.Context(), convID, me.ID, title)
	if err != nil {
		s.writeStoreError(w, err, "ganti judul grup")
		return
	}
	s.finishGroupChange(w, r, convID, change)
}

func (s *Server) handleAddMembers(w http.ResponseWriter, r *http.Request) {
	me := userFrom(r.Context())
	convID, ok := s.authorizedConversation(w, r)
	if !ok {
		return
	}

	var req struct {
		UserIDs []uuid.UUID `json:"userIds"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "body tidak valid")
		return
	}
	if len(req.UserIDs) == 0 {
		writeError(w, http.StatusBadRequest, "tidak ada orang yang dipilih")
		return
	}

	change, err := s.store.AddMembers(r.Context(), convID, me.ID, req.UserIDs)
	if err != nil {
		s.writeStoreError(w, err, "tambah anggota")
		return
	}
	s.finishGroupChange(w, r, convID, change)
}

func (s *Server) handleRemoveMember(w http.ResponseWriter, r *http.Request) {
	me := userFrom(r.Context())
	convID, ok := s.authorizedConversation(w, r)
	if !ok {
		return
	}

	targetID, err := uuid.Parse(r.PathValue("userId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "id anggota tidak valid")
		return
	}

	change, err := s.store.RemoveMember(r.Context(), convID, me.ID, targetID)
	if err != nil {
		s.writeStoreError(w, err, "keluarkan anggota")
		return
	}
	s.finishGroupChange(w, r, convID, change)
}

func (s *Server) handleLeaveGroup(w http.ResponseWriter, r *http.Request) {
	me := userFrom(r.Context())
	convID, ok := s.authorizedConversation(w, r)
	if !ok {
		return
	}

	change, err := s.store.LeaveGroup(r.Context(), convID, me.ID)
	if err != nil {
		s.writeStoreError(w, err, "keluar grup")
		return
	}
	s.finishGroupChange(w, r, convID, change)
}

func (s *Server) handleTransferOwnership(w http.ResponseWriter, r *http.Request) {
	me := userFrom(r.Context())
	convID, ok := s.authorizedConversation(w, r)
	if !ok {
		return
	}

	var req struct {
		UserID uuid.UUID `json:"userId"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "body tidak valid")
		return
	}

	change, err := s.store.TransferOwnership(r.Context(), convID, me.ID, req.UserID)
	if err != nil {
		s.writeStoreError(w, err, "pindah kepemilikan")
		return
	}
	s.finishGroupChange(w, r, convID, change)
}

// finishGroupChange menyiarkan hasilnya lalu menjawab pemanggil.
func (s *Server) finishGroupChange(w http.ResponseWriter, r *http.Request, convID uuid.UUID, change store.GroupChange) {
	s.broadcastGroupChange(r, convID, change)
	writeJSON(w, http.StatusOK, map[string]any{
		"conversationId": convID,
		"title":          change.Title,
		"members":        change.Members,
	})
}

// broadcastGroupChange memberi tahu semua pihak yang terlibat.
//
// Ada TIGA kiriman, dan ketiganya menjawab pertanyaan yang berbeda:
//
//  1. Catatan sistem ke anggota yang tersisa — supaya kejadiannya muncul di
//     tempatnya di dalam percakapan, lewat jalur pesan yang sudah ada.
//  2. Keadaan grup sekarang ke anggota yang tersisa — supaya judul dan daftar
//     anggota di layar mereka ikut berubah tanpa memuat ulang apa pun.
//  3. Kabar "kamu bukan anggota lagi" ke orang yang baru saja pergi.
//
// Yang ketiga dikirim TERPISAH dan setelah yang lain, karena orangnya sudah
// tidak ada di daftar anggota — dia tidak akan pernah menerima dua kiriman
// pertama. Yang dia butuhkan justru kebalikannya: percakapan itu harus hilang
// dari layarnya.
func (s *Server) broadcastGroupChange(r *http.Request, convID uuid.UUID, change store.GroupChange) {
	// Notice tanpa id berarti tidak ada yang benar-benar berubah — misalnya
	// judul diganti jadi judul yang sama. Tidak ada yang layak disiarkan.
	if change.Notice.ID == uuid.Nil {
		return
	}

	remaining := make([]uuid.UUID, 0, len(change.Members))
	for _, m := range change.Members {
		remaining = append(remaining, m.UserID)
	}

	// Orang yang BARU bergabung belum punya percakapan ini di layarnya, jadi
	// "keadaan grup berubah" tidak berarti apa-apa untuknya — yang dia butuhkan
	// adalah percakapannya muncul lebih dulu. Dikirim SEBELUM yang lain supaya
	// catatan sistem yang menyusul punya tempat untuk mendarat.
	if ev := change.Notice.SystemEvent; ev != nil && ev.Type == store.SystemMemberAdded {
		joined := make([]uuid.UUID, 0, len(ev.Targets))
		for _, t := range ev.Targets {
			joined = append(joined, t.ID)
		}
		s.notifyConversationCreatedTo(r, convID, joined)
	}

	s.hub.Publish(remaining, hub.Event{Type: hub.EventMessageNew, Payload: change.Notice})
	s.hub.Publish(remaining, hub.Event{Type: hub.EventConversationUpdated, Payload: map[string]any{
		"conversationId": convID,
		"title":          change.Title,
		"members":        change.Members,
	}})

	if change.Departed != nil {
		s.hub.Publish([]uuid.UUID{*change.Departed}, hub.Event{
			Type:    hub.EventConversationRemoved,
			Payload: map[string]any{"conversationId": convID},
		})
	}
}
