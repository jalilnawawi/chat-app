package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Kelola akun & profil: foto, email, password, sesi, dan status.
//
// Tiga aturan menaungi seluruh berkas ini:
//
//  1. **Status bukan presence.** Presence diturunkan dari koneksi yang hidup
//     dan sengaja fana (Fase 6); status adalah pernyataan yang dibuat orang
//     dengan sengaja dan harus bertahan melewati tutup laptop. Yang satu di
//     Redis, yang satu di sini, dan client menampilkan gabungan keduanya.
//
//  2. **Yang kedaluwarsa dengan sendirinya tidak butuh sesuatu yang berjalan.**
//     Tidak ada job yang membersihkan status lewat waktu; penyaringnya ada di
//     userCols dan berlaku pada setiap jalur baca sekaligus.
//
//  3. **Mengubah kredensial menuntut password saat ini, bukan sekadar sesi yang
//     masih hidup.** Sesi bisa saja milik laptop yang ditinggal terbuka.

// Batas panjang yang dipakai bersama handler HTTP dan database.
//
// CHECK di 0007 memasang batas yang sama untuk status_text, dan dua tempat itu
// memang disengaja: handler menjawab 400 dengan kalimat yang bisa dibaca orang,
// CHECK menjaga jalur yang tidak lewat handler sama sekali.
const (
	maxDisplayName = 60
	maxStatusText  = 120
	maxEmailLen    = 254
)

// ---------- profil ----------

// UpdateDisplayName mengganti nama tampilan.
//
// Username TIDAK ikut bisa diganti, dan itu keputusan, bukan kelalaian.
// Username adalah cara orang lain menemukan seseorang, dan nama yang berpindah
// tangan berarti pesan lama yang menyebut "@budi" suatu hari menunjuk orang
// yang berbeda. Nama tampilan tidak punya masalah itu: dia tidak pernah jadi
// kunci apa pun — bahkan sebutan sudah memakai id sejak Fase 9, justru supaya
// siapa yang dipanggil tidak ditentukan oleh cara sebuah string ditulis.
func (s *Store) UpdateDisplayName(ctx context.Context, userID uuid.UUID, name string) (User, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return User{}, fmt.Errorf("%w: nama tampilan wajib diisi", ErrInvalid)
	}
	if utf8.RuneCountInString(name) > maxDisplayName {
		return User{}, fmt.Errorf("%w: nama tampilan maksimal %d karakter", ErrInvalid, maxDisplayName)
	}

	var u User
	err := scanUser(s.pool.QueryRow(ctx, `
		WITH upd AS (
			UPDATE users SET display_name = $1 WHERE id = $2 RETURNING *
		)
		SELECT `+userCols("upd")+` FROM upd`, name, userID), &u)

	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("ganti nama tampilan: %w", err)
	}
	return u, nil
}

// ---------- foto profil ----------

// SetAvatar memasang foto baru dan MENITIPKAN yang lama ke penyapu.
//
// Yang lama tidak dihapus di sini, dan itu disengaja. Penghapusannya adalah satu
// permintaan HTTP ke penyimpanan yang bisa gagal sendiri, dan kegagalan itu di
// tengah jalur "ganti foto" cuma menyisakan dua pilihan sama buruknya:
// menggagalkan penggantian yang sebenarnya sudah berhasil, atau menelannya
// diam-diam — dan dengan itu melupakan kuncinya selamanya, karena tidak ada satu
// baris pun lagi yang menyebutnya.
//
// Barisnya dicatat di blob_garbage dalam transaksi yang sama, lalu penyapu yang
// sudah ada sejak Fase 7 yang membuangnya. Yang gagal tetap tercatat dan dicoba
// lagi putaran berikutnya.
func (s *Store) SetAvatar(ctx context.Context, userID uuid.UUID, av StoredAvatar) (User, error) {
	return s.swapAvatar(ctx, userID, &av)
}

// RemoveAvatar melepas foto profil. Jalur yang sama persis dengan menggantinya —
// yang berbeda cuma tidak ada yang dipasang setelahnya.
func (s *Store) RemoveAvatar(ctx context.Context, userID uuid.UUID) (User, error) {
	return s.swapAvatar(ctx, userID, nil)
}

func (s *Store) swapAvatar(ctx context.Context, userID uuid.UUID, av *StoredAvatar) (User, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Barisnya dikunci: dua unggahan yang datang hampir bersamaan dari dua tab
	// akan sama-sama membaca kunci lama, dan tanpa kunci ini yang kalah balapan
	// menitipkan kunci yang SUDAH terpasang sebagai sampah — foto yang baru saja
	// dipasang lalu dibuang oleh penyapu beberapa menit kemudian.
	var oldKey *string
	err = tx.QueryRow(ctx,
		`SELECT avatar_key FROM users WHERE id = $1 FOR UPDATE`, userID).Scan(&oldKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("kunci baris pengguna: %w", err)
	}

	var (
		id   *uuid.UUID
		key  *string
		mime *string
		size *int64
	)
	if av != nil {
		id, key, mime, size = &av.ID, &av.Key, &av.MIME, &av.Size
	}

	var u User
	if err := scanUser(tx.QueryRow(ctx, `
		WITH upd AS (
			UPDATE users
			SET avatar_id = $1, avatar_key = $2, avatar_mime = $3, avatar_size = $4
			WHERE id = $5
			RETURNING *
		)
		SELECT `+userCols("upd")+` FROM upd`, id, key, mime, size, userID), &u); err != nil {
		return User{}, fmt.Errorf("pasang avatar: %w", err)
	}

	if oldKey != nil && (key == nil || *oldKey != *key) {
		if _, err := tx.Exec(ctx,
			`INSERT INTO blob_garbage (key) VALUES ($1) ON CONFLICT DO NOTHING`, *oldKey); err != nil {
			return User{}, fmt.Errorf("titipkan avatar lama: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return User{}, err
	}
	return u, nil
}

// AvatarForRead mencari foto berdasarkan ID UNGGAHANNYA.
//
// Izinnya SENGAJA berbeda dari lampiran, dan ditulis eksplisit supaya tidak ada
// yang menyalin aturan lampiran ke sini dan mengira itu kebetulan lebih aman:
// avatar boleh dilihat siapa pun yang sudah login. Orangnya memang sudah bisa
// ditemukan lewat pencarian pengguna, lengkap dengan nama dan username-nya —
// menyembunyikan fotonya di balik "harus satu percakapan dulu" tidak menutup
// apa pun, dan justru membuat hasil pencarian tampil sebagai deretan huruf.
//
// Yang tetap dijaga: tidak ada jalan ke byte-nya selain lewat server ini, jadi
// sesi tetap syarat mutlak.
func (s *Store) AvatarForRead(ctx context.Context, avatarID uuid.UUID) (StoredAvatar, error) {
	av := StoredAvatar{ID: avatarID}
	err := s.pool.QueryRow(ctx,
		`SELECT avatar_key, avatar_mime, avatar_size FROM users WHERE avatar_id = $1`, avatarID,
	).Scan(&av.Key, &av.MIME, &av.Size)

	if errors.Is(err, pgx.ErrNoRows) {
		return StoredAvatar{}, ErrNotFound
	}
	if err != nil {
		return StoredAvatar{}, fmt.Errorf("baca avatar: %w", err)
	}
	return av, nil
}

// TakeBlobGarbage mengambil sekaligus menghapus catatan sampah penyimpanan.
//
// Barisnya dihapus lebih dulu, byte-nya menyusul — urutan yang sama dengan
// TakeOrphanAttachments, dan dengan alasan yang sama: yang tertinggal saat
// proses mati di tengah adalah berkas tanpa baris, bukan baris yang isinya
// sudah hilang.
func (s *Store) TakeBlobGarbage(ctx context.Context, limit int) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
		DELETE FROM blob_garbage
		WHERE key IN (SELECT key FROM blob_garbage ORDER BY created_at LIMIT $1)
		RETURNING key`, limit)
	if err != nil {
		return nil, fmt.Errorf("ambil sampah penyimpanan: %w", err)
	}
	defer rows.Close()

	out := []string{}
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		out = append(out, key)
	}
	return out, rows.Err()
}

// ---------- password ----------

// PasswordHashOf dipakai untuk memverifikasi password SAAT INI sebelum mengubah
// kredensial. Verifikasinya sendiri argon2id dan tidak bisa dikerjakan SQL,
// jadi hash-nya memang harus keluar dari database.
func (s *Store) PasswordHashOf(ctx context.Context, userID uuid.UUID) (string, error) {
	var hash string
	err := s.pool.QueryRow(ctx,
		`SELECT password_hash FROM users WHERE id = $1`, userID).Scan(&hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("ambil hash password: %w", err)
	}
	return hash, nil
}

// ChangePassword menyimpan hash baru dan MENCABUT semua sesi lain.
//
// Keduanya dalam satu transaksi: password yang sudah berganti sementara sesi
// lama masih berlaku adalah keadaan yang tidak boleh pernah ada, sekalipun untuk
// sepersekian detik — justru pada jalur yang dipakai orang yang baru saja sadar
// password-nya dicuri.
//
// Menghapus barisnya BELUM cukup untuk benar-benar memutus koneksi WebSocket
// yang sudah terlanjur terbuka: koneksi itu dipegang di memori proses, dan sejak
// Fase 6 proses itu bisa instance lain. Pemutusannya diurus hub — lihat
// Broadcaster.RevokeSessions.
func (s *Store) ChangePassword(ctx context.Context, userID uuid.UUID, newHash string, keep []byte) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	tag, err := tx.Exec(ctx, `UPDATE users SET password_hash = $1 WHERE id = $2`, newHash, userID)
	if err != nil {
		return fmt.Errorf("ganti password: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}

	if _, err := tx.Exec(ctx,
		`DELETE FROM sessions WHERE user_id = $1 AND token_hash <> $2`, userID, keep); err != nil {
		return fmt.Errorf("cabut sesi lain: %w", err)
	}

	// Token pemulihan yang masih menggantung ikut dibuang. Orang yang baru saja
	// berhasil mengganti password-nya sendiri sudah tidak butuh tautan pemulihan,
	// dan tautan yang masih berlaku di kotak masuk adalah jalan masuk kedua yang
	// tidak diminta siapa pun.
	if _, err := tx.Exec(ctx,
		`DELETE FROM email_tokens WHERE user_id = $1 AND kind = 'reset'`, userID); err != nil {
		return fmt.Errorf("buang token pemulihan: %w", err)
	}

	return tx.Commit(ctx)
}

// ResetPassword memakai token pemulihan lalu mengganti password.
//
// Berbeda dari ChangePassword pada satu hal yang menentukan: di sini SEMUA sesi
// dicabut, tanpa kecuali. Orang yang sampai ke jalur ini tidak sedang memegang
// sesi mana pun yang layak dipercaya — dia baru saja membuktikan bahwa dia
// kehilangan aksesnya.
//
// Token dihapus, bukan ditandai terpakai: satu-satunya pertanyaan yang pernah
// ditanyakan tentangnya adalah "masih berlaku?", dan baris yang tidak ada adalah
// jawaban yang paling sulit disalahtafsirkan.
func (s *Store) ResetPassword(ctx context.Context, tokenHash []byte, newHash string) (uuid.UUID, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var userID uuid.UUID
	err = tx.QueryRow(ctx, `
		DELETE FROM email_tokens
		WHERE token_hash = $1 AND kind = 'reset' AND expires_at > now()
		RETURNING user_id`, tokenHash).Scan(&userID)

	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, fmt.Errorf("%w: tautan pemulihan tidak berlaku lagi", ErrNotFound)
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("pakai token pemulihan: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`UPDATE users SET password_hash = $1 WHERE id = $2`, newHash, userID); err != nil {
		return uuid.Nil, fmt.Errorf("ganti password: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM sessions WHERE user_id = $1`, userID); err != nil {
		return uuid.Nil, fmt.Errorf("cabut semua sesi: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM email_tokens WHERE user_id = $1 AND kind = 'reset'`, userID); err != nil {
		return uuid.Nil, fmt.Errorf("buang token pemulihan lain: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}
	return userID, nil
}

// ---------- email ----------

// NormalizeEmail merapikan alamat dan menolak yang jelas bukan alamat.
//
// Sengaja TIDAK memvalidasi lebih jauh dari ini. Satu-satunya pemeriksaan yang
// benar-benar membuktikan sebuah alamat adalah mengirim surat ke sana dan
// menunggu tautannya diklik — dan itu memang yang dilakukan jalur verifikasi.
// Ekspresi reguler yang lebih ketat hanya menambah cara menolak alamat sah yang
// bentuknya tidak biasa.
func NormalizeEmail(raw string) (string, error) {
	email := strings.TrimSpace(raw)
	if email == "" {
		return "", fmt.Errorf("%w: email wajib diisi", ErrInvalid)
	}
	if len(email) > maxEmailLen {
		return "", fmt.Errorf("%w: email maksimal %d karakter", ErrInvalid, maxEmailLen)
	}

	at := strings.LastIndex(email, "@")
	if at < 1 || at == len(email)-1 ||
		strings.ContainsAny(email, " \t\r\n") ||
		!strings.Contains(email[at+1:], ".") {
		return "", fmt.Errorf("%w: alamat email tidak valid", ErrInvalid)
	}
	return email, nil
}

// SetEmail memasang alamat baru dan MENURUNKAN keadaan verifikasinya.
//
// Baris kedua itu yang penting: alamat yang baru diketik belum dibuktikan
// milik siapa pun. Tanpa penurunan ini, seseorang bisa memverifikasi alamatnya
// sendiri lalu menggantinya dengan alamat orang lain yang ikut terbawa
// "terverifikasi" — dan dengan itu memulihkan akun cukup dengan mengaku
// memiliki sebuah alamat.
func (s *Store) SetEmail(ctx context.Context, userID uuid.UUID, email string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE users SET email = $1, email_verified_at = NULL
		WHERE id = $2 AND (email IS DISTINCT FROM $1 OR email_verified_at IS NOT NULL)`,
		email, userID)

	if isUniqueViolation(err) {
		return fmt.Errorf("%w: email itu sudah dipakai akun lain", ErrConflict)
	}
	if err != nil {
		return fmt.Errorf("simpan email: %w", err)
	}
	if tag.RowsAffected() == 0 {
		// Alamatnya memang sudah persis itu dan memang belum terverifikasi.
		// Bukan kegagalan: jalur pemanggil akan mengirim ulang tautannya, dan
		// itulah yang sebenarnya diminta orang yang menekan tombolnya lagi.
		return nil
	}
	return nil
}

// CreateEmailToken mencatat satu tautan sekali pakai.
//
// Yang disimpan adalah HASH-nya, alasannya sama persis dengan token sesi sejak
// Fase 1: bocornya tabel ini tidak memberi penyerang satu pun tautan yang bisa
// dipakai. Token lama dengan jenis yang sama ikut dibuang — meminta tautan baru
// berarti yang lama sudah tidak dipakai, dan membiarkannya hidup berarti
// menumpuk jalan masuk yang sama-sama sah di kotak masuk orang.
func (s *Store) CreateEmailToken(
	ctx context.Context,
	userID uuid.UUID,
	kind, email string,
	tokenHash []byte,
	expiresAt time.Time,
) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx,
		`DELETE FROM email_tokens WHERE user_id = $1 AND kind = $2`, userID, kind); err != nil {
		return fmt.Errorf("buang token lama: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO email_tokens (token_hash, user_id, kind, email, expires_at)
		VALUES ($1, $2, $3, $4, $5)`,
		tokenHash, userID, kind, email, expiresAt); err != nil {
		return fmt.Errorf("catat token: %w", err)
	}
	return tx.Commit(ctx)
}

// VerifyEmail memakai tautan verifikasi.
//
// Syarat `u.email = t.email` yang menentukan: orang yang meminta verifikasi
// untuk alamat A lalu menggantinya jadi B tidak boleh memverifikasi B dengan
// tautan yang dikirim ke A. Tanpa itu, membuktikan kepemilikan sebuah alamat
// berubah jadi membuktikan kepemilikan alamat apa pun yang diketik sesudahnya.
func (s *Store) VerifyEmail(ctx context.Context, tokenHash []byte) (string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var (
		userID uuid.UUID
		email  string
	)
	err = tx.QueryRow(ctx, `
		DELETE FROM email_tokens
		WHERE token_hash = $1 AND kind = 'verify' AND expires_at > now()
		RETURNING user_id, email`, tokenHash).Scan(&userID, &email)

	if errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("%w: tautan verifikasi tidak berlaku lagi", ErrNotFound)
	}
	if err != nil {
		return "", fmt.Errorf("pakai token verifikasi: %w", err)
	}

	tag, err := tx.Exec(ctx, `
		UPDATE users SET email_verified_at = now()
		WHERE id = $1 AND lower(email) = lower($2)`, userID, email)
	if err != nil {
		return "", fmt.Errorf("tandai email terverifikasi: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return "", fmt.Errorf("%w: alamat ini sudah diganti sejak tautannya dikirim", ErrConflict)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return email, nil
}

// UserByVerifiedEmail mencari pemilik sebuah alamat — dan HANYA yang sudah
// terverifikasi.
//
// Inilah tempat aturan "email yang belum terverifikasi tidak boleh dipakai
// memulihkan akun sama sekali" benar-benar dilaksanakan. Melonggarkannya berarti
// memulihkan akun orang lain cuma butuh mendaftar dengan alamat mereka dan tidak
// pernah membuktikan apa pun.
func (s *Store) UserByVerifiedEmail(ctx context.Context, email string) (uuid.UUID, string, error) {
	var (
		id   uuid.UUID
		name string
	)
	err := s.pool.QueryRow(ctx, `
		SELECT id, display_name FROM users
		WHERE lower(email) = lower($1) AND email_verified_at IS NOT NULL`, email,
	).Scan(&id, &name)

	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, "", ErrNotFound
	}
	if err != nil {
		return uuid.Nil, "", fmt.Errorf("cari pemilik email: %w", err)
	}
	return id, name, nil
}

// DeleteExpiredEmailTokens membuang tautan yang sudah lewat umur, dijalankan
// oleh janitor yang sama dengan pembersih sesi.
func (s *Store) DeleteExpiredEmailTokens(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx, `DELETE FROM email_tokens WHERE expires_at < now()`)
	if err != nil {
		return 0, fmt.Errorf("hapus token kedaluwarsa: %w", err)
	}
	return tag.RowsAffected(), nil
}

// ---------- sesi ----------

// Sessions mengembalikan perangkat yang sedang login, yang terbaru dipakai di
// atas. currentID menandai yang sedang dipakai bertanya.
func (s *Store) Sessions(ctx context.Context, userID, currentID uuid.UUID) ([]Session, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_agent, created_at, last_seen_at, expires_at
		FROM sessions
		WHERE user_id = $1 AND expires_at > now()
		ORDER BY COALESCE(last_seen_at, created_at) DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("daftar sesi: %w", err)
	}
	defer rows.Close()

	out := []Session{}
	for rows.Next() {
		var (
			sess  Session
			agent *string
		)
		if err := rows.Scan(&sess.ID, &agent, &sess.CreatedAt, &sess.LastSeenAt, &sess.ExpiresAt); err != nil {
			return nil, err
		}
		if agent != nil {
			sess.UserAgent = *agent
		}
		sess.Current = sess.ID == currentID
		out = append(out, sess)
	}
	return out, rows.Err()
}

// RevokeSession mencabut satu sesi dan mengembalikan HASH token-nya.
//
// Hash-nya dikembalikan karena menghapus barisnya saja tidak memutus koneksi
// WebSocket yang sudah terbuka — dan hash itulah satu-satunya nama yang dipakai
// hub untuk mengenali koneksi mana yang milik sesi ini. Lihat
// Broadcaster.RevokeSessions.
//
// `user_id = $2` bukan kerapian: tanpa dia, id sesi yang bocor dari mana pun
// bisa dipakai mengeluarkan orang lain dari aplikasinya.
func (s *Store) RevokeSession(ctx context.Context, sessionID, userID uuid.UUID) ([]byte, error) {
	var hash []byte
	err := s.pool.QueryRow(ctx,
		`DELETE FROM sessions WHERE id = $1 AND user_id = $2 RETURNING token_hash`,
		sessionID, userID).Scan(&hash)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("cabut sesi: %w", err)
	}
	return hash, nil
}

// ---------- status ----------

// SetStatus memasang status yang dinyatakan seseorang dengan sengaja.
//
// expiresAt nil berarti tanpa batas waktu. Yang diberi batas TIDAK dijaga timer
// apa pun di server: pembacaan menyaring sendiri (lihat userCols), dan client
// menerima `expiresAt` lalu menghitung mundur sendiri. Alasannya sama dengan
// typing indicator di Fase 3 yang sengaja tidak menyentuh database — keadaan
// yang kedaluwarsa dengan sendirinya tidak butuh sesuatu yang berjalan, dan
// penyapu lintas instance justru menambah masalah: dia butuh penguncian, dan
// tiap instance akan menyiarkan kabar kedaluwarsa yang sama.
func (s *Store) SetStatus(
	ctx context.Context,
	userID uuid.UUID,
	status, text string,
	expiresAt *time.Time,
) (UserStatus, error) {
	switch status {
	case StatusAvailable, StatusBusy, StatusAway:
	default:
		return UserStatus{}, fmt.Errorf("%w: status tidak dikenal", ErrInvalid)
	}

	text = strings.TrimSpace(text)
	if utf8.RuneCountInString(text) > maxStatusText {
		return UserStatus{}, fmt.Errorf("%w: teks status maksimal %d karakter", ErrInvalid, maxStatusText)
	}
	if expiresAt != nil && !expiresAt.After(time.Now()) {
		// Batas waktu yang sudah lewat akan langsung disaring habis oleh
		// pembacaan, jadi menyimpannya berarti menyimpan sesuatu yang tidak akan
		// pernah terlihat — dan orangnya akan mengira fiturnya rusak.
		return UserStatus{}, fmt.Errorf("%w: batas waktu status sudah lewat", ErrInvalid)
	}

	out := UserStatus{UserID: userID}
	err := s.pool.QueryRow(ctx, `
		UPDATE users SET status = $1, status_text = $2, status_expires_at = $3
		WHERE id = $4
		RETURNING status, status_text, status_expires_at`,
		status, text, expiresAt, userID,
	).Scan(&out.Status, &out.Text, &out.ExpiresAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return UserStatus{}, ErrNotFound
	}
	if err != nil {
		return UserStatus{}, fmt.Errorf("pasang status: %w", err)
	}
	return out, nil
}

// StatusesOf mengambil status sekumpulan orang — dan HANYA yang punya sesuatu
// untuk diceritakan.
//
// Yang 'available' tanpa teks adalah keadaan bawaan, dan client sudah
// menganggap semua orang begitu sampai ada kabar lain. Mengirimkannya berarti
// snapshot sebuah akun dengan dua ratus kontak berisi dua ratus baris yang
// tidak mengubah apa pun di layar. Index parsial di 0007 ditulis untuk persis
// syarat ini.
func (s *Store) StatusesOf(ctx context.Context, ids []uuid.UUID) ([]UserStatus, error) {
	if len(ids) == 0 {
		return []UserStatus{}, nil
	}

	rows, err := s.pool.Query(ctx, `
		SELECT id, status, status_text, status_expires_at
		FROM users
		WHERE id = ANY($1)
		  AND (status <> '`+StatusAvailable+`' OR status_text <> '')
		  AND (status_expires_at IS NULL OR status_expires_at > now())`, ids)
	if err != nil {
		return nil, fmt.Errorf("ambil status: %w", err)
	}
	defer rows.Close()

	out := []UserStatus{}
	for rows.Next() {
		var st UserStatus
		if err := rows.Scan(&st.UserID, &st.Status, &st.Text, &st.ExpiresAt); err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

// BusyAmong menyaring daftar orang, mengembalikan yang sedang menyatakan diri
// sibuk PADA SAAT INI.
//
// Inilah yang membuat status bukan sekadar hiasan: dia menyambung ke peredam
// dering Fase 7. Penyaringan waktunya ada di dalam query, bukan di pemanggil —
// status yang sudah lewat jamnya tidak boleh meredam apa pun, dan jam yang
// dipakai harus jam yang sama dengan seluruh jalur baca lain.
func (s *Store) BusyAmong(ctx context.Context, ids []uuid.UUID) ([]uuid.UUID, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	rows, err := s.pool.Query(ctx, `
		SELECT id FROM users
		WHERE id = ANY($1) AND status = '`+StatusBusy+`'
		  AND (status_expires_at IS NULL OR status_expires_at > now())`, ids)
	if err != nil {
		return nil, fmt.Errorf("saring yang sibuk: %w", err)
	}
	defer rows.Close()

	out := []uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
