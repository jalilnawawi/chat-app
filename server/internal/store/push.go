package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// PushSubscription adalah satu pemasangan browser yang bersedia dibangunkan.
//
// Kunci p256dh dan auth dipakai untuk MENGENKRIPSI isi notifikasi. Layanan push
// milik vendor browser cuma meneruskan amplop tertutup: dia tahu ada sesuatu
// untuk perangkat itu, tapi tidak bisa membaca pesannya.
type PushSubscription struct {
	Endpoint string
	UserID   uuid.UUID
	P256dh   string
	Auth     string
}

// SaveSubscription mencatat atau memperbarui langganan.
//
// ON CONFLICT pada endpoint, bukan pada (user, endpoint): browser memberi
// endpoint yang sama kalau izinnya masih sama, dan bila komputer itu dipakai
// orang lain untuk login, langganannya harus BERPINDAH — bukan menghasilkan
// baris kedua yang membuat notifikasi orang sebelumnya ikut muncul.
func (s *Store) SaveSubscription(ctx context.Context, sub PushSubscription, userAgent string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO push_subscriptions (endpoint, user_id, p256dh, auth, user_agent)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (endpoint) DO UPDATE SET
			user_id    = EXCLUDED.user_id,
			p256dh     = EXCLUDED.p256dh,
			auth       = EXCLUDED.auth,
			user_agent = EXCLUDED.user_agent,
			created_at = now()`,
		sub.Endpoint, sub.UserID, sub.P256dh, sub.Auth, userAgent)
	if err != nil {
		return fmt.Errorf("simpan langganan push: %w", err)
	}
	return nil
}

// SubscriptionsOf mengambil semua pemasangan milik sekumpulan user sekaligus.
// Satu query untuk seluruh penerima sebuah pesan, bukan satu query per orang.
func (s *Store) SubscriptionsOf(ctx context.Context, userIDs []uuid.UUID) ([]PushSubscription, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}

	rows, err := s.pool.Query(ctx, `
		SELECT endpoint, user_id, p256dh, auth
		FROM push_subscriptions WHERE user_id = ANY($1)`, userIDs)
	if err != nil {
		return nil, fmt.Errorf("ambil langganan push: %w", err)
	}
	defer rows.Close()

	out := []PushSubscription{}
	for rows.Next() {
		var sub PushSubscription
		if err := rows.Scan(&sub.Endpoint, &sub.UserID, &sub.P256dh, &sub.Auth); err != nil {
			return nil, err
		}
		out = append(out, sub)
	}
	return out, rows.Err()
}

// DeleteSubscription membuang satu pemasangan.
//
// Dipanggil dari dua arah: user mematikan notifikasi di UI, dan layanan push
// menjawab 404/410 yang artinya langganan itu sudah tidak ada di sisi browser.
// Yang kedua lebih sering, dan mengabaikannya berarti mengirim ke alamat mati
// selamanya.
func (s *Store) DeleteSubscription(ctx context.Context, endpoint string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM push_subscriptions WHERE endpoint = $1`, endpoint)
	return err
}

// MarkSubscriptionDelivered mencatat kiriman terakhir yang berhasil — satu-
// satunya cara membedakan langganan yang memang sepi dari yang diam karena
// rusak.
//
// Syarat "sudah lewat sejam" membuat kolom ini tidak jadi jalur tulis panas.
// Yang ingin diketahui adalah apakah langganan masih hidup HARI INI; menulis
// ulang barisnya di setiap notifikasi membayar UPDATE untuk ketelitian sampai
// detik yang tidak pernah ada yang membutuhkannya.
func (s *Store) MarkSubscriptionDelivered(ctx context.Context, endpoint string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE push_subscriptions SET last_ok_at = now()
		WHERE endpoint = $1
		  AND (last_ok_at IS NULL OR last_ok_at < now() - interval '1 hour')`, endpoint)
	return err
}

// ConversationBrief adalah keterangan secukupnya untuk menyusun teks
// notifikasi: judul grup, atau kosong untuk DM (yang memakai nama pengirim).
type ConversationBrief struct {
	Type  string
	Title string
}

func (s *Store) ConversationBrief(ctx context.Context, convID uuid.UUID) (ConversationBrief, error) {
	var (
		b     ConversationBrief
		title *string
	)
	if err := s.pool.QueryRow(ctx,
		`SELECT type, title FROM conversations WHERE id = $1`, convID,
	).Scan(&b.Type, &title); err != nil {
		return ConversationBrief{}, fmt.Errorf("keterangan percakapan: %w", err)
	}
	if title != nil {
		b.Title = *title
	}
	return b, nil
}

// DeleteSubscriptionOf membuang langganan milik user tertentu. Dipakai saat
// orangnya sendiri mematikan notifikasi — berbeda dari DeleteSubscription, yang
// dipakai server saat layanan push memberi tahu bahwa alamatnya sudah mati.
func (s *Store) DeleteSubscriptionOf(ctx context.Context, endpoint string, userID uuid.UUID) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM push_subscriptions WHERE endpoint = $1 AND user_id = $2`, endpoint, userID)
	return err
}
