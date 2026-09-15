package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// attachmentCols adalah kolom yang dikirim ke client. storage_key sengaja TIDAK
// ikut: alamat internal penyimpanan bukan urusan browser, dan satu-satunya
// tempat yang membutuhkannya adalah handler unduh — yang memintanya terpisah.
//
// Ditulis sebagai fungsi, bukan konstanta, supaya alias tabel menempel ke SETIAP
// kolom. Menempelkannya di depan daftar ("a." + daftar) hanya mengenai kolom
// pertama; sisanya jadi tak berkualifikasi dan baru meledak saat suatu hari ada
// tabel lain di query yang kebetulan punya kolom bernama sama.
func attachmentCols(alias string) string {
	if alias != "" {
		alias += "."
	}
	cols := []string{"id", "name", "mime", "size", "width", "height", "thumb_key"}
	for i, c := range cols {
		cols[i] = alias + c
	}
	return strings.Join(cols, ", ")
}

// scanAttachment membaca satu baris dan menurunkan alamat-alamatnya.
//
// thumb_key ikut terbaca walau isinya tidak pernah dikirim ke client: yang
// dibutuhkan client cuma TAHU apakah turunannya ada, dan itu yang menentukan
// ThumbURL diisi atau dibiarkan kosong. Alamat penyimpanannya sendiri berhenti
// di sini.
func scanAttachment(row pgx.Row, a *Attachment) error {
	var thumbKey *string
	if err := row.Scan(&a.ID, &a.Name, &a.MIME, &a.Size, &a.Width, &a.Height, &thumbKey); err != nil {
		return err
	}
	a.URL = AttachmentURL(a.ID)
	if thumbKey != nil {
		a.ThumbURL = AttachmentThumbURL(a.ID)
	}
	return nil
}

// nullable menerjemahkan string kosong jadi NULL.
//
// Kolom turunan dijaga CHECK yang menuntut ketiganya terisi atau ketiganya
// kosong, dan string kosong BUKAN kosong di mata CHECK itu. Tanpa ini, lampiran
// tanpa turunan akan ditolak database — tepat di jalur yang paling sering
// dilewati.
func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// CreateAttachment mencatat lampiran yang isinya SUDAH tersimpan di blob store.
//
// Urutannya disengaja: tulis byte dulu, baru catat barisnya. Kalau prosesnya
// mati di antara keduanya, yang tertinggal adalah file tanpa baris — sampah
// diam yang tidak dilihat siapa pun. Urutan sebaliknya meninggalkan baris tanpa
// file, dan itu tampil di layar orang sebagai lampiran yang rusak.
func (s *Store) CreateAttachment(ctx context.Context, ownerID uuid.UUID, sa StoredAttachment) error {
	var thumbSize *int64
	if sa.ThumbKey != "" {
		thumbSize = &sa.ThumbSize
	}

	_, err := s.pool.Exec(ctx, `
		INSERT INTO attachments
			(id, owner_id, storage_key, name, mime, size, width, height,
			 thumb_key, thumb_mime, thumb_size)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		sa.ID, ownerID, sa.Key, sa.Name, sa.MIME, sa.Size, sa.Width, sa.Height,
		nullable(sa.ThumbKey), nullable(sa.ThumbMIME), thumbSize)
	if isUniqueViolation(err) {
		return ErrConflict
	}
	if err != nil {
		return fmt.Errorf("catat lampiran: %w", err)
	}
	return nil
}

// AttachmentForRead mengembalikan lampiran beserta kunci penyimpanannya, tapi
// HANYA kalau viewer berhak membacanya.
//
// Izinnya dievaluasi di dalam query, bukan di handler, karena inilah satu-
// satunya jalan menuju byte lampiran — dan aturan "boleh dibaca" persis sama
// dengan aturan percakapan: orang yang ada di dalamnya. Yang belum terpasang ke
// pesan mana pun hanya bisa dibaca pengunggahnya sendiri, supaya pratinjau
// sebelum kirim tetap jalan tanpa membuka file itu untuk orang lain.
func (s *Store) AttachmentForRead(ctx context.Context, id, viewerID uuid.UUID) (StoredAttachment, error) {
	var (
		sa        StoredAttachment
		thumbKey  *string
		thumbMIME *string
		thumbSize *int64
	)
	err := s.pool.QueryRow(ctx, `
		SELECT a.storage_key, a.thumb_key, a.thumb_mime, a.thumb_size,
		       a.id, a.name, a.mime, a.size, a.width, a.height
		FROM attachments a
		LEFT JOIN messages m ON m.id = a.message_id
		WHERE a.id = $1
		  AND (
		        a.owner_id = $2
		     OR (m.deleted_at IS NULL AND EXISTS (
		           SELECT 1 FROM conversation_members cm
		           WHERE cm.conversation_id = m.conversation_id AND cm.user_id = $2))
		  )`, id, viewerID,
	).Scan(&sa.Key, &thumbKey, &thumbMIME, &thumbSize,
		&sa.ID, &sa.Name, &sa.MIME, &sa.Size, &sa.Width, &sa.Height)

	if errors.Is(err, pgx.ErrNoRows) {
		// 404 juga untuk lampiran yang ADA tapi bukan haknya: membedakan
		// keduanya berarti memberi tahu orang asing bahwa file itu eksis.
		//
		// Turunan mengikuti aturan yang sama persis, lewat query yang sama
		// persis. Jalur baca kedua yang izinnya diperiksa dengan cara sendiri
		// adalah jalur kedua yang bisa salah sendiri.
		return StoredAttachment{}, ErrNotFound
	}
	if err != nil {
		return StoredAttachment{}, fmt.Errorf("baca lampiran: %w", err)
	}

	sa.URL = AttachmentURL(sa.ID)
	if thumbKey != nil {
		sa.ThumbKey, sa.ThumbMIME, sa.ThumbSize = *thumbKey, *thumbMIME, *thumbSize
		sa.ThumbURL = AttachmentThumbURL(sa.ID)
	}
	return sa, nil
}

// claimAttachments memasang lampiran ke sebuah pesan, di dalam transaksi
// pengiriman pesan itu.
//
// Tiga syarat dipaksakan sekaligus oleh satu UPDATE: lampiran itu milik
// pengirim, belum pernah dipakai pesan lain, dan memang ada. Jumlah baris yang
// kembali lebih sedikit dari yang diminta berarti salah satu syarat gagal —
// tanpa perlu tahu yang mana, karena jawabannya sama: tolak.
func claimAttachments(ctx context.Context, tx pgx.Tx, messageID, ownerID uuid.UUID, ids []uuid.UUID) ([]Attachment, error) {
	if len(ids) == 0 {
		return []Attachment{}, nil
	}

	rows, err := tx.Query(ctx, `
		UPDATE attachments SET message_id = $1
		WHERE id = ANY($2) AND owner_id = $3 AND message_id IS NULL
		RETURNING `+attachmentCols(""), messageID, ids, ownerID)
	if err != nil {
		return nil, fmt.Errorf("pasang lampiran: %w", err)
	}
	defer rows.Close()

	found := map[uuid.UUID]Attachment{}
	for rows.Next() {
		var a Attachment
		if err := scanAttachment(rows, &a); err != nil {
			return nil, err
		}
		found[a.ID] = a
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(found) != len(ids) {
		return nil, ErrForbidden
	}

	// RETURNING tidak menjanjikan urutan. Urutan yang dipakai adalah urutan
	// yang dipilih pengirim di layarnya — itu yang harus dilihat penerima.
	out := make([]Attachment, 0, len(ids))
	for _, id := range ids {
		out = append(out, found[id])
	}
	return out, nil
}

// Orphan adalah lampiran yang diunggah tapi tidak pernah jadi dikirim.
type Orphan struct {
	ID         uuid.UUID
	StorageKey string
	// ThumbKey kosong bila lampiran ini tidak punya turunan. Ikut dibawa
	// karena penyapu adalah SATU-SATUNYA yang akan pernah menyentuh berkas ini
	// lagi — kalau turunannya tidak ikut disebut di sini, dia tinggal di
	// penyimpanan selamanya tanpa satu baris pun yang mengingatnya.
	ThumbKey string
}

// TakeOrphanAttachments mengambil sekaligus menghapus catatan lampiran yatim
// yang sudah lewat umur, dan mengembalikan kunci penyimpanannya supaya isinya
// bisa ikut dibuang.
//
// Barisnya dihapus lebih dulu, byte-nya menyusul. Kalau proses mati di tengah,
// yang tertinggal adalah file tanpa baris — sisa yang tidak terlihat siapa pun
// dan tidak memakan apa pun selain ruang disk. Urutan sebaliknya bisa membuat
// baris yang isinya sudah hilang tetap tampil sebagai lampiran di layar.
func (s *Store) TakeOrphanAttachments(ctx context.Context, olderThan time.Duration, limit int) ([]Orphan, error) {
	rows, err := s.pool.Query(ctx, `
		DELETE FROM attachments
		WHERE id IN (
			SELECT id FROM attachments
			WHERE message_id IS NULL AND created_at < now() - $1::interval
			ORDER BY created_at LIMIT $2
		)
		RETURNING id, storage_key, thumb_key`, olderThan.String(), limit)
	if err != nil {
		return nil, fmt.Errorf("ambil lampiran yatim: %w", err)
	}
	defer rows.Close()

	out := []Orphan{}
	for rows.Next() {
		var (
			o        Orphan
			thumbKey *string
		)
		if err := rows.Scan(&o.ID, &o.StorageKey, &thumbKey); err != nil {
			return nil, err
		}
		if thumbKey != nil {
			o.ThumbKey = *thumbKey
		}
		out = append(out, o)
	}
	return out, rows.Err()
}
