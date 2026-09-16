package api

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/jalilnawawi/chat-app/server/internal/hub"
	"github.com/jalilnawawi/chat-app/server/internal/store"
)

// Reaksi TIDAK pernah membangunkan notifikasi push.
//
// Itu keputusan produk, dan ditulis di sini karena tempat yang paling mungkin
// melanggarnya adalah berkas ini. Sebuah pesan adalah sesuatu yang ditujukan
// kepada seseorang; sebuah emoji adalah tepukan di bahu. Membangunkan ponsel
// orang di tengah malam untuk tepukan di bahu adalah cara tercepat membuat
// seluruh notifikasi aplikasi ini dimatikan orang — termasuk yang benar-benar
// penting.
//
// Karena itu handler di bawah memanggil s.hub.Publish dan berhenti di situ.
// Tidak ada notifyOffline, dan tidak seharusnya ada.

func (s *Server) handleAddReaction(w http.ResponseWriter, r *http.Request) {
	s.reaction(w, r, true)
}

func (s *Server) handleRemoveReaction(w http.ResponseWriter, r *http.Request) {
	s.reaction(w, r, false)
}

// reaction memegang bagian yang sama antara memasang dan mencabut. Keduanya
// memvalidasi masukan yang sama, menyiarkan bentuk payload yang sama, dan
// menjawab hal yang sama — yang berbeda cuma satu baris di tengah.
func (s *Server) reaction(w http.ResponseWriter, r *http.Request, add bool) {
	me := userFrom(r.Context())

	msgID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "id pesan tidak valid")
		return
	}

	var req struct {
		Emoji string `json:"emoji"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "body tidak valid")
		return
	}
	if !validReaction(req.Emoji) {
		writeError(w, http.StatusBadRequest, "reaksi harus berupa satu emoji")
		return
	}

	var change store.ReactionChange
	if add {
		change, err = s.store.AddReaction(r.Context(), msgID, me.ID, req.Emoji)
	} else {
		change, err = s.store.RemoveReaction(r.Context(), msgID, me.ID, req.Emoji)
	}
	if err != nil {
		s.writeStoreError(w, err, "ubah reaksi")
		return
	}

	// Tidak ada yang berubah — emoji yang sama ditekan dua kali, atau dilepas
	// padahal tidak pernah ada. Menyiarkannya akan membuat setiap client
	// menurunkan hitungan yang tidak pernah naik.
	if change.Changed {
		event := hub.EventReactionAdded
		if !add {
			event = hub.EventReactionRemoved
		}
		s.publishToMembers(r, change.ConversationID, hub.Event{Type: event, Payload: map[string]any{
			"conversationId": change.ConversationID,
			"messageId":      change.MessageID,
			"userId":         change.UserID,
			"emoji":          change.Emoji,
			"reactionSeq":    change.ReactionSeq,
		}})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"messageId":   change.MessageID,
		"emoji":       change.Emoji,
		"changed":     change.Changed,
		"reactionSeq": change.ReactionSeq,
	})
}
