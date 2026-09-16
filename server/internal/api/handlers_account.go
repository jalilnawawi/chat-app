package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/jalilnawawi/chat-app/server/internal/auth"
	"github.com/jalilnawawi/chat-app/server/internal/hub"
	"github.com/jalilnawawi/chat-app/server/internal/mail"
	"github.com/jalilnawawi/chat-app/server/internal/store"
)

// Umur tautan yang dikirim lewat email.
//
// Keduanya berbeda karena taruhannya berbeda. Tautan verifikasi cuma menandai
// sebuah alamat sebagai milik seseorang, dan orang membuka kotak masuknya besok
// pagi. Tautan pemulihan MENGGANTI PASSWORD: dia adalah kunci cadangan sebuah
// akun, dan kunci cadangan yang tergeletak di kotak masuk selama sehari penuh
// adalah kunci cadangan yang terlalu lama tergeletak.
const (
	verifyTokenTTL = 24 * time.Hour
	resetTokenTTL  = time.Hour
)

// minPasswordLen dipakai oleh register, ganti password, dan reset password —
// tiga pintu ke kolom yang sama. Tiga aturan yang berbeda untuk satu kolom
// berarti dua di antaranya bisa dipakai untuk melewati yang paling ketat.
const minPasswordLen = 8

// ---------- profil ----------

// handleUpdateProfile mengganti nama tampilan.
//
// Username sengaja TIDAK bisa diganti lewat sini; alasannya ditulis di
// store.UpdateDisplayName.
func (s *Server) handleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	me := userFrom(r.Context())

	var req struct {
		DisplayName string `json:"displayName"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "body tidak valid")
		return
	}

	user, err := s.store.UpdateDisplayName(r.Context(), me.ID, req.DisplayName)
	if err != nil {
		s.writeStoreError(w, err, "ganti nama tampilan")
		return
	}

	s.broadcastUser(r, user)
	writeJSON(w, http.StatusOK, s.meOf(user, me))
}

// handleSetStatus memasang status yang dinyatakan seseorang dengan sengaja.
//
// `expiresAt` adalah INSTAN ABSOLUT yang dihitung client dari pilihan cepatnya
// ("30 menit", "sampai akhir hari"), bukan durasi yang dihitung server. Client
// yang tahu zona waktu orangnya, dan "sampai akhir hari" adalah pertanyaan yang
// hanya bisa dijawab di sana — pukul 23.59 di Jakarta adalah tengah hari di
// tempat lain.
func (s *Server) handleSetStatus(w http.ResponseWriter, r *http.Request) {
	me := userFrom(r.Context())

	var req struct {
		Status    string     `json:"status"`
		Text      string     `json:"text"`
		ExpiresAt *time.Time `json:"expiresAt"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "body tidak valid")
		return
	}

	st, err := s.store.SetStatus(r.Context(), me.ID, req.Status, req.Text, req.ExpiresAt)
	if err != nil {
		s.writeStoreError(w, err, "pasang status")
		return
	}

	// Jalur fan-out yang PERSIS SAMA dengan presence: siaran ke ContactIDs,
	// tidak ke seluruh pengguna aplikasi. Status seseorang cuma urusan orang
	// yang berbagi percakapan dengannya.
	s.publishToContacts(r, me.ID, hub.Event{Type: hub.EventStatus, Payload: st})
	writeJSON(w, http.StatusOK, st)
}

// ---------- password ----------

// handleChangePassword mengganti password, dan MENUNTUT password saat ini.
//
// Sesi yang masih hidup tidak cukup, dan itu seluruh alasan field
// `currentPassword` ada: sesi bisa saja milik laptop yang ditinggal terbuka di
// meja, dan orang yang lewat lalu mengganti password akan mengunci pemiliknya
// keluar dari akunnya sendiri.
func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	me := userFrom(r.Context())
	current := sessionFrom(r.Context())

	var req struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "body tidak valid")
		return
	}
	if len(req.NewPassword) < minPasswordLen {
		writeError(w, http.StatusBadRequest, "password baru minimal 8 karakter")
		return
	}

	if !s.verifyCurrentPassword(w, r, me.ID, req.CurrentPassword) {
		return
	}

	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		s.writeStoreError(w, err, "hash password")
		return
	}

	if err := s.store.ChangePassword(r.Context(), me.ID, hash, current.Hash); err != nil {
		s.writeStoreError(w, err, "ganti password")
		return
	}

	// Barisnya sudah hilang; koneksi yang telanjur terbuka belum. Lihat
	// hub.Broadcaster.RevokeSessions.
	s.hub.RevokeSessionsExcept(me.ID, current.Hash)

	s.notifyByEmail(me, "Password kamu baru saja diganti",
		"Password akun kamu baru saja diganti, dan semua perangkat lain sudah "+
			"dikeluarkan.\n\nKalau bukan kamu yang melakukannya, segera pulihkan "+
			"akunmu lewat \"Lupa password\" di halaman masuk.")

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleForgotPassword mengirim tautan pemulihan.
//
// Jawabannya SELALU sama, apa pun yang terjadi di dalam: alamat yang tidak
// terdaftar, alamat yang terdaftar tapi belum terverifikasi, dan alamat yang
// suratnya benar-benar dikirim menghasilkan 200 yang identik. Membedakannya
// mengubah endpoint ini jadi alat untuk menanyai server "apakah orang ini punya
// akun di sini" — pertanyaan yang tidak seharusnya bisa dijawab siapa pun yang
// cuma menebak alamat.
func (s *Server) handleForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "body tidak valid")
		return
	}

	// Jawaban seragam disiapkan lebih dulu supaya tidak ada satu pun cabang di
	// bawah yang bisa lupa memakainya.
	ok := func() {
		writeJSON(w, http.StatusOK, map[string]string{
			"status": "Kalau alamat itu terdaftar dan sudah terverifikasi, tautannya sudah dikirim.",
		})
	}

	if !s.mail.Enabled() {
		writeError(w, http.StatusServiceUnavailable, "pemulihan lewat email tidak aktif di server ini")
		return
	}

	email, err := store.NormalizeEmail(req.Email)
	if err != nil {
		ok()
		return
	}

	// HANYA yang sudah terverifikasi. Kalau yang belum pun boleh, memulihkan
	// akun orang lain cuma butuh mendaftar dengan alamat mereka dan tidak pernah
	// membuktikan apa pun.
	userID, name, err := s.store.UserByVerifiedEmail(r.Context(), email)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			s.log.Error("cari pemilik email", "err", err)
		}
		ok()
		return
	}

	token, hash, err := auth.NewToken()
	if err != nil {
		s.writeStoreError(w, err, "buat token pemulihan")
		return
	}
	if err := s.store.CreateEmailToken(r.Context(), userID, "reset", email, hash,
		time.Now().Add(resetTokenTTL)); err != nil {
		s.writeStoreError(w, err, "catat token pemulihan")
		return
	}

	s.mail.Enqueue(mail.Message{
		To:      email,
		Subject: "Pulihkan password kamu",
		Body: "Halo " + name + ",\n\n" +
			"Ada yang meminta pemulihan password untuk akun kamu. Buka tautan ini " +
			"untuk memasang password baru:\n\n" +
			s.cfg.AppURL + "/?reset=" + token + "\n\n" +
			"Tautannya berlaku satu jam dan hanya bisa dipakai sekali.\n\n" +
			"Kalau bukan kamu yang meminta, abaikan surat ini — password kamu tidak berubah.",
	})
	ok()
}

// handleResetPassword memasang password baru lewat tautan pemulihan.
//
// Berbeda dari ganti password biasa pada satu hal: di sini SEMUA sesi dicabut,
// tanpa kecuali. Orang yang sampai ke jalur ini tidak sedang memegang sesi mana
// pun yang layak dipercaya — dia baru saja membuktikan bahwa dia kehilangan
// aksesnya.
func (s *Server) handleResetPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "body tidak valid")
		return
	}
	if len(req.Password) < minPasswordLen {
		writeError(w, http.StatusBadRequest, "password minimal 8 karakter")
		return
	}
	if req.Token == "" {
		writeError(w, http.StatusBadRequest, "tautan pemulihan tidak lengkap")
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		s.writeStoreError(w, err, "hash password")
		return
	}

	userID, err := s.store.ResetPassword(r.Context(), auth.HashToken(req.Token), hash)
	if err != nil {
		s.writeStoreError(w, err, "pulihkan password")
		return
	}

	s.hub.RevokeSessionsExcept(userID, nil)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ---------- email ----------

// handleSetEmail memasang alamat email dan mengirim tautan verifikasinya.
//
// Menuntut password saat ini, dengan alasan yang sama dengan ganti password:
// alamat email adalah jalan masuk kedua ke sebuah akun, jadi memasangnya adalah
// perubahan kredensial — bukan sekadar mengisi kolom profil.
func (s *Server) handleSetEmail(w http.ResponseWriter, r *http.Request) {
	me := userFrom(r.Context())

	var req struct {
		Email           string `json:"email"`
		CurrentPassword string `json:"currentPassword"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "body tidak valid")
		return
	}

	email, err := store.NormalizeEmail(req.Email)
	if err != nil {
		s.writeStoreError(w, err, "email")
		return
	}
	if !s.verifyCurrentPassword(w, r, me.ID, req.CurrentPassword) {
		return
	}

	if err := s.store.SetEmail(r.Context(), me.ID, email); err != nil {
		s.writeStoreError(w, err, "simpan email")
		return
	}

	me.Email, me.EmailVerified = email, false
	s.sendVerification(r, me)
	writeJSON(w, http.StatusOK, me)
}

// handleResendVerification mengirim ulang tautan verifikasi untuk alamat yang
// SUDAH tersimpan. Tidak menuntut password: dia tidak mengubah apa pun, dan
// alamat tujuannya adalah alamat yang sudah tercatat — bukan yang diketik pada
// permintaan ini.
func (s *Server) handleResendVerification(w http.ResponseWriter, r *http.Request) {
	me := userFrom(r.Context())

	if !s.mail.Enabled() {
		writeError(w, http.StatusServiceUnavailable, "email tidak aktif di server ini")
		return
	}
	if me.Email == "" {
		writeError(w, http.StatusBadRequest, "belum ada alamat email di akun ini")
		return
	}
	if me.EmailVerified {
		writeError(w, http.StatusConflict, "alamat ini sudah terverifikasi")
		return
	}

	if !s.sendVerification(r, me) {
		writeError(w, http.StatusInternalServerError, "gagal menyiapkan tautan verifikasi")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleVerifyEmail memakai tautan verifikasi.
//
// TIDAK menuntut sesi. Tautannya diklik dari kotak masuk, dan kotak masuk itu
// sering dibuka di perangkat yang berbeda dari tempat akunnya dipakai — menuntut
// login lebih dulu berarti menuntut orang memasukkan password justru pada jalur
// yang ada untuk membuktikan sesuatu yang lain. Token itu sendiri sudah rahasia,
// sekali pakai, dan berumur pendek.
func (s *Server) handleVerifyEmail(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "body tidak valid")
		return
	}
	if req.Token == "" {
		writeError(w, http.StatusBadRequest, "tautan verifikasi tidak lengkap")
		return
	}

	email, err := s.store.VerifyEmail(r.Context(), auth.HashToken(req.Token))
	if err != nil {
		s.writeStoreError(w, err, "verifikasi email")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"email": email})
}

// ---------- sesi ----------

func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	me := userFrom(r.Context())

	sessions, err := s.store.Sessions(r.Context(), me.ID, sessionFrom(r.Context()).ID)
	if err != nil {
		s.writeStoreError(w, err, "daftar sesi")
		return
	}
	writeJSON(w, http.StatusOK, sessions)
}

// handleRevokeSession mengeluarkan satu perangkat.
//
// Dua langkah yang keduanya wajib: barisnya dihapus supaya permintaan
// berikutnya ditolak, DAN koneksi WebSocket-nya ditutup — di instance mana pun
// dia kebetulan dipegang. Tanpa langkah kedua, perangkat yang baru saja dicabut
// tetap menerima setiap pesan yang masuk sampai orangnya sendiri menutup tab.
func (s *Server) handleRevokeSession(w http.ResponseWriter, r *http.Request) {
	me := userFrom(r.Context())

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "id sesi tidak valid")
		return
	}

	hash, err := s.store.RevokeSession(r.Context(), id, me.ID)
	if err != nil {
		s.writeStoreError(w, err, "cabut sesi")
		return
	}

	// Penyaring yang berlawanan dari ganti password: di sana semua ditutup
	// kecuali satu, di sini satu ditutup dan sisanya tidak disentuh.
	s.hub.RevokeSession(me.ID, hash)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ---------- pembantu ----------

// verifyCurrentPassword adalah gerbang yang dipakai bersama oleh ganti password
// dan ganti email. Menulis jawabannya sendiri ke w dan mengembalikan false bila
// tidak lolos.
func (s *Server) verifyCurrentPassword(w http.ResponseWriter, r *http.Request, userID uuid.UUID, password string) bool {
	hash, err := s.store.PasswordHashOf(r.Context(), userID)
	if err != nil {
		s.writeStoreError(w, err, "ambil hash password")
		return false
	}
	if err := auth.VerifyPassword(hash, password); err != nil {
		writeError(w, http.StatusForbidden, "password saat ini salah")
		return false
	}
	return true
}

// sendVerification menyiapkan token lalu menitipkan suratnya. Mengembalikan
// false hanya bila tokennya gagal dibuat — surat yang gagal DIKIRIM bukan
// urusan pemanggil, karena pengirimannya memang tidak sinkron.
func (s *Server) sendVerification(r *http.Request, me store.Me) bool {
	if !s.mail.Enabled() || me.Email == "" {
		return false
	}

	token, hash, err := auth.NewToken()
	if err != nil {
		s.log.Error("buat token verifikasi", "err", err)
		return false
	}
	if err := s.store.CreateEmailToken(r.Context(), me.ID, "verify", me.Email, hash,
		time.Now().Add(verifyTokenTTL)); err != nil {
		s.log.Error("catat token verifikasi", "err", err)
		return false
	}

	s.mail.Enqueue(mail.Message{
		To:      me.Email,
		Subject: "Verifikasi alamat email kamu",
		Body: "Halo " + me.DisplayName + ",\n\n" +
			"Buka tautan ini untuk membuktikan bahwa alamat ini memang milik kamu:\n\n" +
			s.cfg.AppURL + "/?verify=" + token + "\n\n" +
			"Tautannya berlaku 24 jam.\n\n" +
			"Sampai alamat ini terverifikasi, dia tidak bisa dipakai memulihkan akun.\n\n" +
			"Kalau bukan kamu yang memasang alamat ini, abaikan surat ini.",
	})
	return true
}

// notifyByEmail mengabari pemilik akun tentang perubahan yang BARU SAJA terjadi
// pada kredensialnya.
//
// Suratnya tidak meminta orangnya melakukan apa pun, dan itu justru gunanya:
// dia satu-satunya cara pemilik akun tahu bahwa password-nya diganti oleh orang
// lain, pada saat itu masih bisa dibatalkan lewat pemulihan.
//
// Hanya ke alamat yang SUDAH terverifikasi: alamat yang belum dibuktikan bisa
// saja milik orang lain, dan mengirimi mereka kabar tentang akun yang bukan
// milik mereka adalah kebocoran kecil yang dibuat oleh fitur keamanan.
func (s *Server) notifyByEmail(me store.Me, subject, body string) {
	if !s.mail.Enabled() || !me.EmailVerified {
		return
	}
	s.mail.Enqueue(mail.Message{
		To:      me.Email,
		Subject: subject,
		Body:    "Halo " + me.DisplayName + ",\n\n" + body,
	})
}

// meOf menyusun jawaban untuk pemilik akun dari bentuk publik yang baru saja
// berubah, ditambah bagian pribadi yang tidak ikut berubah.
func (s *Server) meOf(user store.User, previous store.Me) store.Me {
	return store.Me{User: user, Email: previous.Email, EmailVerified: previous.EmailVerified}
}

// broadcastUser memberi tahu kontak bahwa nama atau foto seseorang berubah.
//
// Tanpa ini, foto baru memang punya alamat baru — tapi tidak seorang pun tahu
// alamat itu sampai mereka memuat ulang halamannya. Cache setahun yang dipasang
// pada byte avatar jadi benar dan tidak berguna sekaligus: byte-nya tidak basi,
// penunjuknya yang basi.
func (s *Server) broadcastUser(r *http.Request, user store.User) {
	s.publishToContacts(r, user.ID, hub.Event{Type: hub.EventUserUpdated, Payload: user})
}

// publishToContacts menyiarkan ke orang yang berbagi percakapan dengan userID,
// DAN ke userID itu sendiri.
//
// Dirinya ikut dengan sengaja: seseorang bisa membuka aplikasi ini di dua tab,
// dan status yang diganti di satu tab harus ikut terlihat di tab lainnya. Ini
// yang membedakannya dari siaran presence, yang memang tidak perlu kembali ke
// pemiliknya — browser sudah tahu sendiri kalau dirinya online.
func (s *Server) publishToContacts(r *http.Request, userID uuid.UUID, ev hub.Event) {
	contacts, err := s.store.ContactIDs(r.Context(), userID)
	if err != nil {
		s.log.Error("ambil kontak untuk siaran", "user", userID, "err", err)
		return
	}
	s.hub.Publish(append(contacts, userID), ev)
}
