package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/jalilnawawi/chat-app/server/internal/hub"
)

const (
	defaultPageSize = 50
	maxPageSize     = 100
	maxMessageLen   = 4000
)

func (s *Server) handleSearchUsers(w http.ResponseWriter, r *http.Request) {
	me := userFrom(r.Context())
	users, err := s.store.SearchUsers(r.Context(), r.URL.Query().Get("q"), me.ID, 20)
	if err != nil {
		s.writeStoreError(w, err, "search users")
		return
	}
	writeJSON(w, http.StatusOK, users)
}

func (s *Server) handleListConversations(w http.ResponseWriter, r *http.Request) {
	me := userFrom(r.Context())
	convs, err := s.store.ListConversations(r.Context(), me.ID)
	if err != nil {
		s.writeStoreError(w, err, "list conversations")
		return
	}
	writeJSON(w, http.StatusOK, convs)
}

func (s *Server) handleCreateDirect(w http.ResponseWriter, r *http.Request) {
	me := userFrom(r.Context())

	var req struct {
		PeerID uuid.UUID `json:"peerId"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "body tidak valid")
		return
	}

	convID, err := s.store.GetOrCreateDirect(r.Context(), me.ID, req.PeerID)
	if err != nil {
		s.writeStoreError(w, err, "create direct")
		return
	}

	s.notifyConversationCreated(r, convID)
	writeJSON(w, http.StatusOK, map[string]any{"conversationId": convID})
}

func (s *Server) handleCreateGroup(w http.ResponseWriter, r *http.Request) {
	me := userFrom(r.Context())

	var req struct {
		Title     string      `json:"title"`
		MemberIDs []uuid.UUID `json:"memberIds"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "body tidak valid")
		return
	}

	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		writeError(w, http.StatusBadRequest, "judul grup wajib diisi")
		return
	}

	convID, err := s.store.CreateGroup(r.Context(), me.ID, req.Title, req.MemberIDs)
	if err != nil {
		s.writeStoreError(w, err, "create group")
		return
	}

	s.notifyConversationCreated(r, convID)
	writeJSON(w, http.StatusCreated, map[string]any{"conversationId": convID})
}

func (s *Server) handleMembers(w http.ResponseWriter, r *http.Request) {
	convID, ok := s.authorizedConversation(w, r)
	if !ok {
		return
	}

	members, err := s.store.Members(r.Context(), convID)
	if err != nil {
		s.writeStoreError(w, err, "members")
		return
	}
	writeJSON(w, http.StatusOK, members)
}

func (s *Server) handleListMessages(w http.ResponseWriter, r *http.Request) {
	convID, ok := s.authorizedConversation(w, r)
	if !ok {
		return
	}

	q := r.URL.Query()
	before, _ := strconv.ParseInt(q.Get("before"), 10, 64)

	limit := defaultPageSize
	if v, err := strconv.Atoi(q.Get("limit")); err == nil && v > 0 {
		limit = min(v, maxPageSize)
	}

	messages, err := s.store.ListMessages(r.Context(), convID, before, limit)
	if err != nil {
		s.writeStoreError(w, err, "list messages")
		return
	}

	// hasMore memberi tahu client apakah masih ada halaman di atas, tanpa perlu
	// COUNT(*) yang mahal di tabel pesan.
	writeJSON(w, http.StatusOK, map[string]any{
		"messages": messages,
		"hasMore":  len(messages) == limit,
	})
}

func (s *Server) handleSendMessage(w http.ResponseWriter, r *http.Request) {
	me := userFrom(r.Context())
	convID, ok := s.authorizedConversation(w, r)
	if !ok {
		return
	}

	var req struct {
		// ID dibuat client (UUIDv7). Inilah kunci idempotensi: kirim ulang
		// setelah timeout jaringan mengembalikan pesan yang sama, bukan duplikat.
		ID   uuid.UUID `json:"id"`
		Body string    `json:"body"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "body tidak valid")
		return
	}

	if req.ID == uuid.Nil {
		writeError(w, http.StatusBadRequest, "id pesan wajib diisi client")
		return
	}

	req.Body = strings.TrimSpace(req.Body)
	if req.Body == "" {
		writeError(w, http.StatusBadRequest, "pesan kosong")
		return
	}
	if len(req.Body) > maxMessageLen {
		writeError(w, http.StatusBadRequest, "pesan terlalu panjang")
		return
	}

	msg, created, err := s.store.SendMessage(r.Context(), req.ID, convID, me.ID, req.Body)
	if err != nil {
		s.writeStoreError(w, err, "send message")
		return
	}

	// Kirim ulang yang duplikat tidak disiarkan lagi — penerima sudah punya.
	if created {
		s.publishToMembers(r, convID, hub.Event{Type: hub.EventMessageNew, Payload: msg})
	}
	writeJSON(w, http.StatusCreated, msg)
}

func (s *Server) handleEditMessage(w http.ResponseWriter, r *http.Request) {
	me := userFrom(r.Context())

	msgID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "id pesan tidak valid")
		return
	}

	var req struct {
		Body string `json:"body"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "body tidak valid")
		return
	}

	req.Body = strings.TrimSpace(req.Body)
	if req.Body == "" || len(req.Body) > maxMessageLen {
		writeError(w, http.StatusBadRequest, "isi pesan tidak valid")
		return
	}

	msg, err := s.store.EditMessage(r.Context(), msgID, me.ID, req.Body)
	if err != nil {
		s.writeStoreError(w, err, "edit message")
		return
	}

	s.publishToMembers(r, msg.ConversationID, hub.Event{Type: hub.EventMessageUpdated, Payload: msg})
	writeJSON(w, http.StatusOK, msg)
}

func (s *Server) handleDeleteMessage(w http.ResponseWriter, r *http.Request) {
	me := userFrom(r.Context())

	msgID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "id pesan tidak valid")
		return
	}

	msg, err := s.store.DeleteMessage(r.Context(), msgID, me.ID)
	if err != nil {
		s.writeStoreError(w, err, "delete message")
		return
	}

	s.publishToMembers(r, msg.ConversationID, hub.Event{Type: hub.EventMessageUpdated, Payload: msg})
	writeJSON(w, http.StatusOK, msg)
}

func (s *Server) handleMarkRead(w http.ResponseWriter, r *http.Request) {
	me := userFrom(r.Context())
	convID, ok := s.authorizedConversation(w, r)
	if !ok {
		return
	}

	var req struct {
		Seq int64 `json:"seq"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "body tidak valid")
		return
	}

	current, err := s.store.MarkRead(r.Context(), convID, me.ID, req.Seq)
	if err != nil {
		s.writeStoreError(w, err, "mark read")
		return
	}

	s.publishToMembers(r, convID, hub.Event{Type: hub.EventReadUpdated, Payload: map[string]any{
		"conversationId": convID,
		"userId":         me.ID,
		"lastReadSeq":    current,
	}})
	writeJSON(w, http.StatusOK, map[string]any{"lastReadSeq": current})
}

// ---------- helper ----------

// authorizedConversation mengurai id percakapan dari path dan memastikan
// pemanggil memang anggotanya. Satu-satunya gerbang otorisasi untuk endpoint
// yang menyentuh isi percakapan.
func (s *Server) authorizedConversation(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	me := userFrom(r.Context())

	convID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "id percakapan tidak valid")
		return uuid.Nil, false
	}

	member, err := s.store.IsMember(r.Context(), convID, me.ID)
	if err != nil {
		s.writeStoreError(w, err, "cek keanggotaan")
		return uuid.Nil, false
	}
	if !member {
		// 404, bukan 403: jangan mengungkap keberadaan percakapan yang bukan
		// milik pemanggil.
		writeError(w, http.StatusNotFound, "tidak ditemukan")
		return uuid.Nil, false
	}
	return convID, true
}

func (s *Server) publishToMembers(r *http.Request, convID uuid.UUID, ev hub.Event) {
	members, err := s.store.MemberIDs(r.Context(), convID)
	if err != nil {
		s.log.Error("ambil anggota untuk siaran", "conversation", convID, "err", err)
		return
	}
	s.hub.Publish(members, ev)
}

// notifyConversationCreated memberi tahu anggota bahwa ada percakapan baru,
// supaya sidebar mereka langsung menampilkannya tanpa perlu refresh.
func (s *Server) notifyConversationCreated(r *http.Request, convID uuid.UUID) {
	members, err := s.store.MemberIDs(r.Context(), convID)
	if err != nil {
		return
	}
	for _, id := range members {
		convs, err := s.store.ListConversations(r.Context(), id)
		if err != nil {
			continue
		}
		for _, c := range convs {
			if c.ID == convID {
				s.hub.Publish([]uuid.UUID{id}, hub.Event{Type: hub.EventConversationNew, Payload: c})
				break
			}
		}
	}
}
