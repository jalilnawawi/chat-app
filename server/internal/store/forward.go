package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Meneruskan adalah MENGIRIM PESAN, bukan fitur tersendiri.
//
// Pesan yang diteruskan lewat jalur SendMessage yang sama persis: id dari
// client (jadi pengiriman ulang tidak menggandakan), kuota pesan, siaran, dan
// notifikasi push. Yang berbeda cuma dari mana isinya datang. Meneruskan ke
// lima percakapan adalah lima pengiriman, masing-masing dengan jawabannya
// sendiri — satu yang gagal tidak membatalkan empat yang berhasil.
//
// Isinya DISALIN, berbeda dari kutipan balasan yang dibaca ulang setiap kali.
// Kelihatannya bertentangan dengan aturan "yang boleh disalin adalah yang tidak
// pernah berubah", padahal justru aturan itu yang dipakai: pesan terusan adalah
// ucapan BARU dari orang yang meneruskan, tentang apa yang dia lihat pada saat
// itu. Kalau penulis aslinya menyunting kalimatnya besok, kalimat yang sudah
// diteruskan tidak boleh ikut berubah di percakapan yang tidak pernah dia
// datangi — dan kutipan yang dibaca ulang lintas percakapan adalah jalur baca
// ke percakapan yang tidak diikuti penerimanya.

// forwarded adalah pesan sumber yang sudah lolos pemeriksaan izin.
type forwarded struct {
	id   uuid.UUID
	body string
	// attachmentIDs dalam urutan yang dilihat penulisnya — urutan dari salinan
	// jsonb, bukan urutan baris di tabel lampiran.
	attachmentIDs []uuid.UUID
}

// forwardSource membaca pesan yang akan diteruskan, sekaligus MEMERIKSA bahwa
// pengirim memang bisa membacanya, dan menguncinya sampai transaksi selesai.
//
// Pemeriksaannya gerbang kebocoran yang sama dengan replyPreview: tanpa dia,
// siapa pun bisa meneruskan id pesan dari percakapan yang tidak dia ikuti ke
// percakapannya sendiri, lalu membaca isinya di sana. Jawabannya sama untuk
// pesan yang tidak ada, yang sudah dihapus, catatan sistem, dan yang bukan
// haknya.
//
// FOR SHARE menahan penghapusan pesan sumber sampai salinannya selesai
// tercatat. Tanpa kunci ini, penghapusan yang jatuh persis di antara "baris
// lampiran sumber dibaca" dan "salinannya di-commit" membuat baris sumber jadi
// yatim — dan penyapu yang kebetulan berjalan pada milidetik itu tidak melihat
// salinan yang belum di-commit, lalu membuang byte yang sebentar lagi ditunjuk
// pesan baru. Lihat TakeOrphanAttachments.
func forwardSource(ctx context.Context, tx pgx.Tx, messageID, senderID uuid.UUID) (*forwarded, error) {
	var (
		src  = forwarded{id: messageID}
		atts []Attachment
	)
	err := tx.QueryRow(ctx, `
		SELECT m.body, m.attachments
		FROM messages m
		JOIN conversation_members cm
		  ON cm.conversation_id = m.conversation_id AND cm.user_id = $2
		WHERE m.id = $1 AND m.kind = 'user' AND m.deleted_at IS NULL
		FOR SHARE OF m`, messageID, senderID,
	).Scan(&src.body, &atts)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("%w: pesan yang diteruskan tidak bisa dibaca", ErrForbidden)
	}
	if err != nil {
		return nil, fmt.Errorf("baca pesan yang diteruskan: %w", err)
	}

	for _, a := range atts {
		src.attachmentIDs = append(src.attachmentIDs, a.ID)
	}
	return &src, nil
}

// copyAttachments membuat baris lampiran BARU untuk pesan terusan, dengan
// kunci penyimpanan yang dipakai bersama.
//
// Barisnya disalin, byte-nya tidak. Baris baru punya pemilik baru dan pesan
// baru, jadi izin bacanya mengikuti percakapan TUJUAN — anggota percakapan itu
// bisa membukanya tanpa pernah menjadi anggota percakapan sumbernya. Itulah
// sebabnya tidak cukup menyalin jsonb-nya saja: alamat lampiran sumber
// diperiksa terhadap keanggotaan percakapan sumber, dan penerima terusan akan
// mendapat 404 untuk setiap gambar.
func copyAttachments(ctx context.Context, tx pgx.Tx, src *forwarded, messageID, ownerID uuid.UUID) ([]Attachment, error) {
	if len(src.attachmentIDs) == 0 {
		return []Attachment{}, nil
	}

	newIDs := make([]uuid.UUID, len(src.attachmentIDs))
	for i := range newIDs {
		newIDs[i] = uuid.New()
	}

	// `a.message_id = $3` bukan pemeriksaan izin — itu sudah terjadi di
	// forwardSource — melainkan pemeriksaan bahwa id di dalam jsonb memang
	// menunjuk lampiran pesan itu. Salinan jsonb adalah salinan-baca; yang
	// berwenang tetap tabelnya.
	rows, err := tx.Query(ctx, `
		INSERT INTO attachments
			(id, owner_id, message_id, storage_key, name, mime, size, width, height, duration_ms,
			 thumb_key, thumb_mime, thumb_size)
		SELECT pair.new_id, $4, $5, a.storage_key, a.name, a.mime, a.size, a.width, a.height, a.duration_ms,
		       a.thumb_key, a.thumb_mime, a.thumb_size
		FROM unnest($1::uuid[], $2::uuid[]) AS pair(src_id, new_id)
		JOIN attachments a ON a.id = pair.src_id AND a.message_id = $3
		RETURNING `+attachmentCols(""),
		src.attachmentIDs, newIDs, src.id, ownerID, messageID)
	if err != nil {
		return nil, fmt.Errorf("salin lampiran terusan: %w", err)
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
	if len(found) != len(newIDs) {
		// Salinan jsonb menyebut lampiran yang tidak lagi terpasang ke pesan
		// itu. Meneruskan sebagian lampiran diam-diam lebih buruk daripada
		// menolak: pengirimnya melihat tiga foto dan penerimanya dua.
		return nil, fmt.Errorf("%w: lampiran pesan yang diteruskan tidak lengkap", ErrConflict)
	}

	out := make([]Attachment, 0, len(newIDs))
	for _, id := range newIDs {
		out = append(out, found[id])
	}
	return out, nil
}
