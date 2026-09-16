package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/jalilnawawi/chat-app/server/internal/hub"
	"github.com/jalilnawawi/chat-app/server/internal/store"
)

const (
	defaultPageSize = 50
	maxPageSize     = 100
	maxMessageLen   = 4000

	// maxMentionsPerMessage membatasi penyebutan satu per satu. Yang ingin
	// membangunkan lebih banyak orang dari ini sebenarnya sedang memaksudkan
	// @semua — dan @semua punya kuotanya sendiri, justru supaya jalan ini tidak
	// dipakai untuk menghindarinya.
	maxMentionsPerMessage = 50
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

	// Aturan judul dipakai bersama dengan jalur ganti judul. Dua tempat yang
	// memeriksa hal yang sama dengan caranya masing-masing adalah dua tempat
	// yang suatu hari akan berbeda pendapat.
	title, err := store.NormalizeGroupTitle(req.Title)
	if err != nil {
		s.writeStoreError(w, err, "judul grup")
		return
	}

	convID, err := s.store.CreateGroup(r.Context(), me.ID, title, req.MemberIDs)
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
	me := userFrom(r.Context())
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

	// Pembaca ikut disebut karena ringkasan reaksi memuat "apakah aku ikut" —
	// satu-satunya bagian sebuah pesan yang jawabannya berbeda per orang.
	messages, err := s.store.ListMessages(r.Context(), convID, me.ID, before, limit)
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
		// AttachmentIDs menunjuk berkas yang sudah diunggah lebih dulu lewat
		// POST /api/attachments. Pemisahan itu disengaja: unggahan bisa lama
		// dan bisa gagal sendiri, sedangkan pengiriman pesan harus tetap satu
		// tindakan yang cepat dan punya jawaban pasti.
		AttachmentIDs []uuid.UUID `json:"attachmentIds"`

		// ReplyToID menunjuk pesan yang dibalas. Pemeriksaan bahwa pesan itu
		// berada di percakapan yang SAMA dilakukan di store, di dalam transaksi
		// pengiriman — lihat catatan kebocoran di store.replyPreview.
		ReplyToID *uuid.UUID `json:"replyToId"`

		// MentionedUserIDs dikirim EKSPLISIT oleh client; server tidak pernah
		// mengurai "@nama" dari teks.
		//
		// Nama tampilan boleh mengandung spasi, jadi penguraian teks akan
		// selalu punya kasus tepi — tapi bukan itu alasan utamanya. Alasan
		// utamanya: siapa yang dibangunkan tidak boleh ditentukan oleh cara
		// sebuah string kebetulan ditulis. Orang yang menulis "kirim ke
		// @budi ya" dalam kalimat biasa tidak sedang memanggil Budi, dan orang
		// yang mengganti nama tampilannya tidak boleh membuat pesan lama
		// berhenti memanggilnya.
		MentionedUserIDs []uuid.UUID `json:"mentionedUserIds"`
		MentionsAll      bool        `json:"mentionsAll"`
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
	// Pesan boleh tanpa teks ASALKAN membawa lampiran — mengirim foto tanpa
	// keterangan adalah hal yang paling biasa dilakukan orang.
	if req.Body == "" && len(req.AttachmentIDs) == 0 {
		writeError(w, http.StatusBadRequest, "pesan kosong")
		return
	}
	if len(req.Body) > maxMessageLen {
		writeError(w, http.StatusBadRequest, "pesan terlalu panjang")
		return
	}
	if len(req.AttachmentIDs) > maxAttachmentsPerMessage {
		writeError(w, http.StatusBadRequest,
			fmt.Sprintf("maksimal %d lampiran per pesan", maxAttachmentsPerMessage))
		return
	}
	if hasDuplicate(req.AttachmentIDs) {
		// Lampiran yang sama disebut dua kali akan lolos UPDATE pertama lalu
		// gagal dihitung — ditolak di sini supaya pesannya tidak batal di
		// tengah transaksi karena kesalahan yang gampang dikenali.
		writeError(w, http.StatusBadRequest, "lampiran yang sama disebut lebih dari sekali")
		return
	}
	if len(req.MentionedUserIDs) > maxMentionsPerMessage {
		writeError(w, http.StatusBadRequest,
			fmt.Sprintf("maksimal %d sebutan per pesan", maxMentionsPerMessage))
		return
	}

	// @semua punya kuotanya sendiri, terpisah dari kuota pesan biasa, dan
	// dihitung per percakapan.
	//
	// Satu orang yang membangunkan dua ratus orang sekaligus adalah hal yang
	// harus DIBATASI, bukan dilarang: rapat yang benar-benar mendesak memang
	// ada. Yang tidak boleh ada adalah kemampuan mengulanginya setiap beberapa
	// detik. Kuncinya memakai token bucket yang sama dengan kuota lain, jadi
	// batasnya tetap satu walau koneksinya tersebar ke beberapa instance.
	if req.MentionsAll {
		key := "mentionall:" + me.ID.String() + ":" + convID.String()
		if !s.allow(w, r, "mention_all", key, s.cfg.MentionAllRate) {
			return
		}
	}

	msg, created, err := s.store.SendMessage(r.Context(), store.SendParams{
		ID:             req.ID,
		ConversationID: convID,
		SenderID:       me.ID,
		Body:           req.Body,
		AttachmentIDs:  req.AttachmentIDs,
		ReplyToID:      req.ReplyToID,
		Mentions:       req.MentionedUserIDs,
		MentionsAll:    req.MentionsAll,
	})
	if err != nil {
		s.writeStoreError(w, err, "send message")
		return
	}

	// Kirim ulang yang duplikat tidak disiarkan lagi — penerima sudah punya,
	// dan membangunkan orang dua kali untuk satu pesan adalah bug yang hanya
	// muncul pada jaringan buruk, yaitu justru saat retry paling sering
	// terjadi.
	if created {
		// Daftar anggota diambil SEKALI lalu dipakai dua kali. Ini jalur
		// terpanas di aplikasi — satu query per pesan terkirim, dikali jumlah
		// orang yang mengetik — dan mengambilnya dua kali untuk jawaban yang
		// sama persis adalah biaya yang tidak membeli apa pun.
		members, err := s.store.MemberIDs(r.Context(), convID)
		if err != nil {
			s.log.Error("ambil anggota untuk siaran", "conversation", convID, "err", err)
		} else {
			s.hub.Publish(members, hub.Event{Type: hub.EventMessageNew, Payload: msg})
			s.notifyOffline(r, msg, me, members)
		}
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

// handleAckMentions menurunkan penanda "ada yang menyebut kamu".
//
// Terpisah dari /read dengan sengaja, dan itu seluruh gunanya. Menandai
// terbaca terjadi begitu percakapannya dibuka; ini baru terjadi setelah pesan
// yang menyebut namanya benar-benar muncul di layar orangnya. Membuka ruang
// sekilas untuk mengintip tidak boleh menghapus panggilan yang belum dilihat —
// dan kalau keduanya dijadikan satu endpoint, dia akan menghapusnya.
func (s *Server) handleAckMentions(w http.ResponseWriter, r *http.Request) {
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

	current, err := s.store.AckMentions(r.Context(), convID, me.ID, req.Seq)
	if err != nil {
		s.writeStoreError(w, err, "ack sebutan")
		return
	}

	// Tidak disiarkan ke siapa pun. Sejauh mana seseorang sudah melihat
	// panggilan untuk dirinya sendiri bukan urusan anggota lain — berbeda
	// dengan read receipt, yang memang ditujukan untuk dilihat lawan bicara.
	writeJSON(w, http.StatusOK, map[string]any{"mentionAckSeq": current})
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

// hasDuplicate melaporkan apakah ada id yang disebut lebih dari sekali.
func hasDuplicate(ids []uuid.UUID) bool {
	seen := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			return true
		}
		seen[id] = struct{}{}
	}
	return false
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
	s.notifyConversationCreatedTo(r, convID, members)
}

// notifyConversationCreatedTo mengirim kabar percakapan baru hanya kepada orang
// yang disebut.
//
// Bentuk percakapan berbeda untuk tiap orang — jumlah belum dibaca, penanda
// sebutan, dan lawan bicara pada DM — jadi tidak ada satu payload yang bisa
// dipakai bersama, dan tiap penerima memang butuh query sendiri.
//
// Itulah sebabnya daftar penerimanya harus bisa dipersempit: saat seseorang
// ditambahkan ke grup berisi dua ratus orang, yang perlu diberi tahu "ada
// percakapan baru" cuma dia. Seratus sembilan puluh sembilan sisanya sudah
// punya percakapan itu, dan mengirimi mereka semua berarti dua ratus query
// untuk satu orang yang bergabung.
func (s *Server) notifyConversationCreatedTo(r *http.Request, convID uuid.UUID, targets []uuid.UUID) {
	for _, id := range targets {
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
