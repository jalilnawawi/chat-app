package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Menyematkan pesan.
//
// Tiga aturan, dan tidak satu pun baru:
//
//  1. **Pemilik mengelola.** Di grup, sematan adalah hak pemilik — aturan yang
//     sama dengan judul dan keanggotaan di Fase 9b. Menyematkan adalah
//     mengatur apa yang dilihat SEMUA orang setiap kali membuka grupnya, dan
//     itu memang pekerjaan mengelola. Di DM tidak ada pemilik, jadi dua orang
//     di dalamnya sama-sama boleh.
//
//  2. **Setiap perubahan meninggalkan catatan.** Menyematkan dan melepas
//     sama-sama menulis pesan sistem. Catatan itu bukan cuma jejak: dia yang
//     memberi tahu client yang sedang menyusul setelah reconnect bahwa daftar
//     sematannya berubah, lewat cursor `seq` yang sudah ada — sematan tidak
//     butuh jam ketiga seperti reaksi.
//
//  3. **Pesan dulu, percakapan kemudian.** Baris pesan dikunci sebelum baris
//     percakapan, urutan yang sama dengan DeleteMessage dan meneruskan pesan.
//     Urutan sebaliknya membuka dua celah sekaligus: deadlock dengan
//     penghapusan, dan sematan yang tercatat untuk pesan yang terhapus pada
//     milidetik yang sama — penghapusan tidak melihat baris sematan yang
//     belum di-commit, dan tidak ada yang akan membersihkannya lagi.

// maxPins membatasi jumlah sematan per percakapan.
//
// Daftar sematan adalah daftar yang DIBACA, bukan dicari. Lebih dari dua puluh,
// dan dia berubah jadi riwayat kedua yang harus digulir — tepat hal yang ingin
// dihindari orang saat menyematkan sesuatu.
const maxPins = 20

// PinChange adalah hasil satu tindakan sematan.
type PinChange struct {
	ConversationID uuid.UUID

	// Changed false berarti keadaannya memang sudah begitu: menyematkan yang
	// sudah tersemat, atau melepas yang tidak tersemat. Tidak ada catatan dan
	// tidak ada siaran — sama dengan penekanan emoji yang kembar.
	Changed bool

	// Notice adalah catatan sistem yang baru ditulis, bila Changed.
	Notice Message
}

// PinMessage menyematkan sebuah pesan.
func (s *Store) PinMessage(ctx context.Context, messageID, actorID uuid.UUID) (PinChange, error) {
	return s.changePin(ctx, messageID, actorID, SystemMessagePinned,
		func(ctx context.Context, tx pgx.Tx, convID uuid.UUID) (bool, error) {
			var count int
			if err := tx.QueryRow(ctx,
				`SELECT count(*) FROM pinned_messages WHERE conversation_id = $1`, convID,
			).Scan(&count); err != nil {
				return false, fmt.Errorf("hitung sematan: %w", err)
			}

			tag, err := tx.Exec(ctx, `
				INSERT INTO pinned_messages (message_id, conversation_id, pinned_by)
				VALUES ($1, $2, $3) ON CONFLICT (message_id) DO NOTHING`,
				messageID, convID, actorID)
			if err != nil {
				return false, fmt.Errorf("sematkan pesan: %w", err)
			}
			if tag.RowsAffected() == 0 {
				return false, nil
			}
			// Diperiksa SETELAH penyisipan, supaya menyematkan ulang pesan yang
			// sudah tersemat di percakapan yang penuh tetap dijawab "tidak ada
			// yang berubah", bukan "penuh". Hitungannya aman dari balapan:
			// baris percakapan terkunci sepanjang transaksi.
			if count >= maxPins {
				return false, fmt.Errorf("%w: paling banyak %d pesan disematkan; lepas salah satu dulu", ErrConflict, maxPins)
			}
			return true, nil
		})
}

// UnpinMessage melepas sematan sebuah pesan.
func (s *Store) UnpinMessage(ctx context.Context, messageID, actorID uuid.UUID) (PinChange, error) {
	return s.changePin(ctx, messageID, actorID, SystemMessageUnpinned,
		func(ctx context.Context, tx pgx.Tx, _ uuid.UUID) (bool, error) {
			tag, err := tx.Exec(ctx, `DELETE FROM pinned_messages WHERE message_id = $1`, messageID)
			if err != nil {
				return false, fmt.Errorf("lepas sematan: %w", err)
			}
			return tag.RowsAffected() > 0, nil
		})
}

// changePin memegang bagian yang sama antara menyematkan dan melepas: izin,
// urutan kunci, dan catatan sistemnya.
func (s *Store) changePin(
	ctx context.Context,
	messageID, actorID uuid.UUID,
	eventType string,
	apply func(context.Context, pgx.Tx, uuid.UUID) (bool, error),
) (PinChange, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return PinChange{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Langkah pertama: pesannya, dikunci. Satu jawaban untuk empat keadaan —
	// tidak ada, sudah dihapus, catatan sistem, atau di percakapan yang bukan
	// milik pemanggil — dengan alasan yang sama dengan changeReaction.
	var (
		convID uuid.UUID
		seq    int64
	)
	err = tx.QueryRow(ctx, `
		SELECT m.conversation_id, m.seq
		FROM messages m
		JOIN conversation_members cm
		  ON cm.conversation_id = m.conversation_id AND cm.user_id = $2
		WHERE m.id = $1 AND m.kind = 'user' AND m.deleted_at IS NULL
		FOR SHARE OF m`, messageID, actorID).Scan(&convID, &seq)
	if errors.Is(err, pgx.ErrNoRows) {
		return PinChange{}, ErrNotFound
	}
	if err != nil {
		return PinChange{}, fmt.Errorf("cari pesan untuk sematan: %w", err)
	}

	// Langkah kedua: percakapannya, dikunci — alokasi `seq` untuk catatan
	// sistem memakai kunci yang sama dengan pengiriman pesan biasa.
	var (
		lastSeq  int64
		convType string
	)
	if err := tx.QueryRow(ctx,
		`SELECT last_seq, type FROM conversations WHERE id = $1 FOR UPDATE`, convID,
	).Scan(&lastSeq, &convType); err != nil {
		return PinChange{}, fmt.Errorf("kunci percakapan: %w", err)
	}

	actor, role, err := partyOf(ctx, tx, convID, actorID)
	if err != nil {
		return PinChange{}, err
	}
	if convType == "group" && role != "owner" {
		return PinChange{}, fmt.Errorf("%w: hanya pemilik grup yang bisa menyematkan pesan", ErrForbidden)
	}

	change := PinChange{ConversationID: convID}
	if change.Changed, err = apply(ctx, tx, convID); err != nil {
		return PinChange{}, err
	}
	if !change.Changed {
		return change, tx.Commit(ctx)
	}

	change.Notice, err = appendNotice(ctx, tx, convID, lastSeq, SystemEvent{
		Type: eventType, Actor: actor, MessageID: &messageID, MessageSeq: seq,
	})
	if err != nil {
		return PinChange{}, err
	}
	return change, tx.Commit(ctx)
}

// Pins mengembalikan pesan yang disematkan di sebuah percakapan, terbaru di
// atas. Pemeriksaan keanggotaan adalah urusan pemanggil — daftar ini dibaca
// lewat gerbang authorizedConversation yang sama dengan riwayat.
//
// `deleted_at IS NULL` ditulis walau penghapusan sudah melepas sematannya:
// dua penjaga untuk satu janji lebih murah daripada satu sematan yang
// menampilkan "pesan ini dihapus" di bagian paling atas percakapan.
func (s *Store) Pins(ctx context.Context, convID, viewerID uuid.UUID) ([]Pin, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+messageCols("m")+`, `+replyCols("rep")+`,
		       p.pinned_by, coalesce(u.display_name, ''), p.pinned_at
		FROM pinned_messages p
		JOIN messages m ON m.id = p.message_id
		LEFT JOIN messages rep ON rep.id = m.reply_to_id
		LEFT JOIN users u ON u.id = p.pinned_by
		WHERE p.conversation_id = $1 AND m.deleted_at IS NULL
		ORDER BY p.pinned_at DESC, p.message_id`, convID)
	if err != nil {
		return nil, fmt.Errorf("daftar sematan: %w", err)
	}
	defer rows.Close()

	out := []Pin{}
	for rows.Next() {
		var p Pin
		dest, finish := messageDest(&p.Message)
		if err := rows.Scan(append(dest, &p.PinnedBy, &p.PinnedByName, &p.PinnedAt)...); err != nil {
			return nil, err
		}
		finish()
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Reaksi ikut, supaya pesan yang dibuka dari daftar sematan tampil sama
	// persis dengan yang ada di riwayat.
	messages := make([]Message, len(out))
	for i := range out {
		messages[i] = out[i].Message
	}
	if err := s.attachReactions(ctx, messages, viewerID); err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Message = messages[i]
	}
	return out, nil
}
