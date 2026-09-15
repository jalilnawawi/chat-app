package api

import (
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/jalilnawawi/chat-app/server/internal/push"
	"github.com/jalilnawawi/chat-app/server/internal/store"
)

// handlePushConfig memberi tahu client apakah notifikasi tersedia, sekaligus
// kunci publik yang dibutuhkan browser untuk mendaftar.
//
// Client TIDAK boleh menyimpan kunci ini di kodenya sendiri. Kunci adalah
// urusan deployment, dan browser mengunci langganannya pada kunci yang dipakai
// saat mendaftar — kunci yang tertinggal di bundel frontend akan jadi kunci
// yang salah begitu server diganti.
func (s *Server) handlePushConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"enabled":   s.push.Enabled(),
		"publicKey": s.push.PublicKey(),
	})
}

func (s *Server) handlePushSubscribe(w http.ResponseWriter, r *http.Request) {
	me := userFrom(r.Context())

	if !s.push.Enabled() {
		writeError(w, http.StatusServiceUnavailable, "notifikasi tidak aktif di server ini")
		return
	}

	var req struct {
		Endpoint string `json:"endpoint"`
		Keys     struct {
			P256dh string `json:"p256dh"`
			Auth   string `json:"auth"`
		} `json:"keys"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "body tidak valid")
		return
	}

	// Endpoint datang dari browser, tapi tetap diperiksa: dia akan jadi alamat
	// yang dihubungi server ini nanti. Tanpa pembatasan skema, sebuah
	// "langganan" bisa dipakai menyuruh server memanggil alamat internal —
	// dan itu bukan lagi notifikasi, melainkan pemindai jaringan gratis.
	if !strings.HasPrefix(req.Endpoint, "https://") || len(req.Endpoint) > 1000 {
		writeError(w, http.StatusBadRequest, "endpoint langganan tidak valid")
		return
	}
	if req.Keys.P256dh == "" || req.Keys.Auth == "" {
		writeError(w, http.StatusBadRequest, "kunci langganan tidak lengkap")
		return
	}

	err := s.store.SaveSubscription(r.Context(), store.PushSubscription{
		Endpoint: req.Endpoint,
		UserID:   me.ID,
		P256dh:   req.Keys.P256dh,
		Auth:     req.Keys.Auth,
	}, truncate(r.UserAgent(), 300))
	if err != nil {
		s.writeStoreError(w, err, "simpan langganan push")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handlePushUnsubscribe(w http.ResponseWriter, r *http.Request) {
	me := userFrom(r.Context())

	var req struct {
		Endpoint string `json:"endpoint"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "body tidak valid")
		return
	}

	// Dibatasi ke langganan milik pemanggil. Endpoint memang praktis tidak bisa
	// ditebak, tapi "praktis tidak bisa ditebak" adalah alasan yang berumur
	// pendek — satu kebocoran log sudah cukup untuk mematikan notifikasi orang
	// lain.
	if err := s.store.DeleteSubscriptionOf(r.Context(), req.Endpoint, me.ID); err != nil {
		s.writeStoreError(w, err, "hapus langganan push")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// notifyOffline menitipkan kabar untuk anggota yang sedang tidak terhubung.
//
// Dipanggil setelah pesan tersimpan DAN setelah disiarkan, dan tidak pernah
// menahan jawaban ke pengirim: paket push menyimpannya di antrean lalu
// mengembalikan kendali seketika. Penyaringan siapa yang benar-benar layak
// dibangunkan terjadi di sana, karena "siapa yang online" baru berarti pada
// detik pengirimannya — bukan pada detik ini.
func (s *Server) notifyOffline(r *http.Request, msg store.Message, sender store.User, members []uuid.UUID) {
	if !s.push.Enabled() {
		return
	}

	recipients := make([]uuid.UUID, 0, len(members))
	for _, id := range members {
		if id != sender.ID {
			recipients = append(recipients, id)
		}
	}
	if len(recipients) == 0 {
		return
	}

	brief, err := s.store.ConversationBrief(r.Context(), msg.ConversationID)
	if err != nil {
		s.log.Warn("keterangan percakapan untuk notifikasi", "err", err)
		return
	}

	title, body := notificationText(brief, sender, msg)
	s.push.Enqueue(push.Notification{
		Recipients:     recipients,
		ConversationID: msg.ConversationID,
		MessageID:      msg.ID,
		Title:          title,
		Body:           body,
	})
}

// notificationText menyusun kalimat yang muncul di layar kunci.
//
// Bentuknya mengikuti pertanyaan yang sebenarnya ada di kepala orang saat
// ponselnya berbunyi: dari siapa, dan tentang apa. Untuk DM, judulnya nama
// pengirim — nama grup tidak ada dan tidak dibutuhkan. Untuk grup, judulnya
// nama grup dan nama pengirim pindah ke awal isi, karena "Tim Produk" tanpa
// "Rina:" tidak memberi tahu apa-apa.
func notificationText(brief store.ConversationBrief, sender store.User, msg store.Message) (string, string) {
	body := strings.TrimSpace(msg.Body)
	if body == "" {
		body = attachmentSummary(msg.Attachments)
	}
	body = truncate(body, 200)

	if brief.Type == "group" {
		title := brief.Title
		if title == "" {
			title = "Grup"
		}
		return title, sender.DisplayName + ": " + body
	}
	return sender.DisplayName, body
}

// attachmentSummary dipakai saat pesannya tidak punya teks sama sekali.
// "Mengirim lampiran" tanpa keterangan membuat orang harus membuka aplikasi
// hanya untuk tahu apakah itu layak dibuka sekarang.
func attachmentSummary(atts []store.Attachment) string {
	switch {
	case len(atts) == 0:
		return "Mengirim pesan"
	case len(atts) == 1 && strings.HasPrefix(atts[0].MIME, "image/"):
		return "📷 Mengirim gambar"
	case len(atts) == 1:
		return "📎 " + atts[0].Name
	default:
		return fmt.Sprintf("📎 Mengirim %d lampiran", len(atts))
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	// Dipotong pada batas rune, bukan byte: memotong di tengah karakter
	// menghasilkan byte rusak yang tampil sebagai kotak di layar orang.
	cut := max
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return strings.TrimSpace(s[:cut]) + "…"
}
