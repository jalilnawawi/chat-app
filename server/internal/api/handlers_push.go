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

// Keadaan notifikasi dan kunci publik VAPID dilaporkan oleh GET /api/config,
// bersama seluruh bagian opsional lain — lihat Server.features. Endpoint
// terpisah untuk satu fitur berarti client harus tahu lebih dulu fitur mana
// yang punya endpoint sendiri, dan itu daftar yang tumbuh tiap fase.

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
		Mentioned:      mentionedAmong(msg, recipients),
	})
}

// mentionedAmong menentukan siapa di antara penerima yang namanya disebut —
// merekalah yang menembus peredam dering.
//
// @semua diterjemahkan jadi "semua penerima" DI SINI, bukan saat disimpan.
// Alasannya sama dengan kenapa mentions_all jadi kolom tersendiri di database:
// keanggotaan grup berubah, dan pesan lama harus tetap berarti "semua orang",
// bukan "semua orang yang kebetulan ada di sana waktu itu".
func mentionedAmong(msg store.Message, recipients []uuid.UUID) []uuid.UUID {
	if msg.MentionsAll {
		return recipients
	}
	if len(msg.Mentions) == 0 {
		return nil
	}

	named := make(map[uuid.UUID]struct{}, len(msg.Mentions))
	for _, id := range msg.Mentions {
		named[id] = struct{}{}
	}

	// Disaring terhadap daftar penerima, bukan dipakai apa adanya: sebutan
	// sudah divalidasi saat pesannya masuk, tapi keanggotaan bisa berubah di
	// antara dua momen itu, dan yang membangunkan orang tidak boleh berjalan di
	// atas daftar yang lebih longgar dari daftar penerimanya sendiri.
	out := make([]uuid.UUID, 0, len(msg.Mentions))
	for _, id := range recipients {
		if _, ok := named[id]; ok {
			out = append(out, id)
		}
	}
	return out
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
	// Terusan diberi tanda juga di layar kunci. Tanpa itu, kalimat orang lain
	// yang diteruskan terbaca sebagai kalimat pengirimnya sendiri — justru di
	// tempat yang tidak punya ruang untuk penjelasan lain.
	if msg.Forwarded {
		body = "Diteruskan: " + body
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
