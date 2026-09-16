package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Reaksi punya satu persoalan yang tidak dimiliki balasan maupun sebutan:
// dia mengubah pesan LAMA.
//
// Seluruh mesin sinkronisasi aplikasi ini berdiri di atas satu pertanyaan —
// "pesan apa yang seq-nya lebih besar dari punyaku?" — dan reaksi tidak muat di
// sana. Menekan emoji pada pesan kemarin tidak menggerakkan seq apa pun, jadi
// orang yang menutup laptopnya dan membukanya lagi akan melihat riwayat yang
// lengkap dengan reaksi yang hilang semua.
//
// Jawabannya adalah jam kedua: `conversations.reaction_seq` dinaikkan setiap
// kali ada reaksi yang berubah, dan pesan yang bersangkutan mencatat nilai itu
// di `messages.reaction_seq`. Client menyimpan nilai terbesar yang pernah
// dilihatnya sebagai cursor kedua, dan menyusul dengan pertanyaan yang bentuknya
// persis sama — hanya pada jam yang berbeda.
//
// Yang menyusul adalah KEADAAN, bukan kejadian. Dua puluh orang yang menekan
// lalu melepas emoji yang sama menghasilkan satu kiriman berisi jawaban akhir,
// dan pencabutan tidak butuh baris nisan apa pun untuk terlihat: barisnya
// hilang, jam pesannya tetap maju.

// bumpReactionSeq mengambil nomor jam reaksi berikutnya dan menempelkannya ke
// sebuah pesan. Dipanggil untuk penambahan MAUPUN pencabutan.
func bumpReactionSeq(ctx context.Context, tx pgx.Tx, convID, messageID uuid.UUID) (int64, error) {
	var seq int64
	if err := tx.QueryRow(ctx, `
		UPDATE conversations SET reaction_seq = reaction_seq + 1
		WHERE id = $1 RETURNING reaction_seq`, convID).Scan(&seq); err != nil {
		return 0, fmt.Errorf("alokasi jam reaksi: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`UPDATE messages SET reaction_seq = $1 WHERE id = $2`, seq, messageID); err != nil {
		return 0, fmt.Errorf("tandai jam reaksi pesan: %w", err)
	}
	return seq, nil
}

// ReactionChange adalah hasil satu penekanan: apa yang berubah, di mana, dan
// pada nomor jam berapa.
type ReactionChange struct {
	ConversationID uuid.UUID
	MessageID      uuid.UUID
	UserID         uuid.UUID
	Emoji          string
	ReactionSeq    int64

	// Changed false berarti keadaannya memang sudah seperti itu — emoji yang
	// sama ditekan dua kali, atau dilepas padahal tidak pernah dipasang.
	// Jam reaksi TIDAK dinaikkan untuk itu, dan tidak ada yang perlu disiarkan.
	Changed bool
}

// AddReaction memasang satu emoji pada satu pesan.
//
// Izinnya dievaluasi di dalam query, bukan di handler: sebuah id pesan saja
// tidak memberi tahu percakapan mana yang memuatnya, dan menanyakannya lebih
// dulu berarti dua perjalanan ke database untuk satu jawaban yang sama. INSERT
// ... SELECT di bawah hanya menghasilkan baris bila pemanggil benar-benar
// anggota percakapan tempat pesan itu berada.
//
// Pesan yang sudah dihapus tidak bisa direaksikan. Isinya sudah tidak ada, dan
// emoji yang menempel pada "pesan ini dihapus" tidak berarti apa pun.
func (s *Store) AddReaction(ctx context.Context, messageID, userID uuid.UUID, emoji string) (ReactionChange, error) {
	return s.changeReaction(ctx, messageID, userID, emoji, func(ctx context.Context, tx pgx.Tx, convID uuid.UUID) (bool, error) {
		tag, err := tx.Exec(ctx, `
			INSERT INTO message_reactions (message_id, user_id, emoji)
			VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`, messageID, userID, emoji)
		if err != nil {
			return false, fmt.Errorf("pasang reaksi: %w", err)
		}
		return tag.RowsAffected() > 0, nil
	})
}

// RemoveReaction mencabut satu emoji milik pemanggil sendiri. Emoji orang lain
// tidak bisa disentuh: baris yang dihapus selalu dibatasi user_id pemanggil.
func (s *Store) RemoveReaction(ctx context.Context, messageID, userID uuid.UUID, emoji string) (ReactionChange, error) {
	return s.changeReaction(ctx, messageID, userID, emoji, func(ctx context.Context, tx pgx.Tx, convID uuid.UUID) (bool, error) {
		tag, err := tx.Exec(ctx, `
			DELETE FROM message_reactions
			WHERE message_id = $1 AND user_id = $2 AND emoji = $3`, messageID, userID, emoji)
		if err != nil {
			return false, fmt.Errorf("cabut reaksi: %w", err)
		}
		return tag.RowsAffected() > 0, nil
	})
}

// changeReaction memegang bagian yang sama persis antara memasang dan mencabut:
// menemukan percakapannya sambil memeriksa izin, menjalankan perubahannya, lalu
// menaikkan jam HANYA bila memang ada yang berubah.
func (s *Store) changeReaction(
	ctx context.Context,
	messageID, userID uuid.UUID,
	emoji string,
	apply func(context.Context, pgx.Tx, uuid.UUID) (bool, error),
) (ReactionChange, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ReactionChange{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var convID uuid.UUID
	err = tx.QueryRow(ctx, `
		SELECT m.conversation_id
		FROM messages m
		JOIN conversation_members cm
		  ON cm.conversation_id = m.conversation_id AND cm.user_id = $2
		WHERE m.id = $1 AND m.deleted_at IS NULL AND m.kind = 'user'`, messageID, userID).Scan(&convID)

	if errors.Is(err, pgx.ErrNoRows) {
		// Satu jawaban untuk empat keadaan: pesannya tidak ada, sudah dihapus,
		// bukan ucapan siapa pun (catatan sistem), atau ada di percakapan yang
		// bukan milik pemanggil. Membedakan yang terakhir berarti memberi tahu
		// orang asing bahwa pesan itu eksis.
		return ReactionChange{}, ErrNotFound
	}
	if err != nil {
		return ReactionChange{}, fmt.Errorf("cari pesan untuk reaksi: %w", err)
	}

	changed, err := apply(ctx, tx, convID)
	if err != nil {
		return ReactionChange{}, err
	}

	change := ReactionChange{
		ConversationID: convID, MessageID: messageID,
		UserID: userID, Emoji: emoji, Changed: changed,
	}
	if !changed {
		// Tidak ada yang berubah berarti tidak ada yang perlu disiarkan, dan
		// jam reaksi tidak boleh maju — memajukannya akan membuat setiap client
		// menyusul sesuatu yang isinya sama persis dengan yang sudah dia punya.
		return change, tx.Commit(ctx)
	}

	if change.ReactionSeq, err = bumpReactionSeq(ctx, tx, convID, messageID); err != nil {
		return ReactionChange{}, err
	}
	return change, tx.Commit(ctx)
}

// clearReactions membuang reaksi sebuah pesan yang baru saja dihapus, dan
// memajukan jamnya supaya client yang sedang offline ikut kehilangan hitungan
// yang sudah tidak punya pesan lagi.
func clearReactions(ctx context.Context, tx pgx.Tx, convID, messageID uuid.UUID) error {
	tag, err := tx.Exec(ctx, `DELETE FROM message_reactions WHERE message_id = $1`, messageID)
	if err != nil {
		return fmt.Errorf("buang reaksi pesan: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil
	}
	_, err = bumpReactionSeq(ctx, tx, convID, messageID)
	return err
}

// attachReactions mengisi ringkasan reaksi untuk SEHALAMAN pesan sekaligus.
//
// Satu query beragregasi untuk seluruh halaman, bukan satu query per pesan:
// riwayat lima puluh pesan yang membayar lima puluh perjalanan ke database
// adalah persis bentuk kegagalan yang tidak terlihat di dua browser di meja dan
// langsung terlihat pada percakapan sungguhan.
//
// `Mine` dihitung di dalam SQL dan itu sebabnya viewerID harus ikut: ini satu-
// satunya bagian sebuah pesan yang jawabannya berbeda untuk tiap pembaca.
func (s *Store) attachReactions(ctx context.Context, messages []Message, viewerID uuid.UUID) error {
	ids := make([]uuid.UUID, 0, len(messages))
	for _, m := range messages {
		// Pesan yang reaction_seq-nya nol tidak pernah menerima satu reaksi
		// pun. Menyaringnya di sini membuat halaman percakapan biasa — yang
		// sebagian besar pesannya memang begitu — tidak menanyakan apa pun.
		if m.ReactionSeq > 0 {
			ids = append(ids, m.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}

	rows, err := s.pool.Query(ctx, `
		SELECT message_id, emoji, count(*), bool_or(user_id = $2)
		FROM message_reactions
		WHERE message_id = ANY($1)
		GROUP BY message_id, emoji
		ORDER BY message_id, min(created_at)`, ids, viewerID)
	if err != nil {
		return fmt.Errorf("ringkas reaksi: %w", err)
	}
	defer rows.Close()

	byMessage := map[uuid.UUID][]ReactionSummary{}
	for rows.Next() {
		var (
			id uuid.UUID
			r  ReactionSummary
		)
		if err := rows.Scan(&id, &r.Emoji, &r.Count, &r.Mine); err != nil {
			return err
		}
		byMessage[id] = append(byMessage[id], r)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for i := range messages {
		if got, ok := byMessage[messages[i].ID]; ok {
			messages[i].Reactions = got
		}
	}
	return nil
}

// ReactionsSince mengembalikan keadaan reaksi setiap pesan yang berubah setelah
// nomor jam tertentu — jalur menyusul setelah reconnect.
//
// LEFT JOIN, bukan JOIN: pesan yang SELURUH reaksinya dicabut selagi client
// offline tetap harus ikut, justru karena jawabannya sekarang kosong. Dengan
// INNER JOIN dia akan lenyap dari hasil, dan client menyimpan hitungan lama
// selamanya — tepat kasus yang membuat "state, bukan event" dipilih di sini.
func (s *Store) ReactionsSince(ctx context.Context, convID, viewerID uuid.UUID, afterSeq int64, limit int) ([]MessageReactions, error) {
	rows, err := s.pool.Query(ctx, `
		WITH changed AS (
			SELECT id, reaction_seq FROM messages
			WHERE conversation_id = $1 AND reaction_seq > $2
			ORDER BY reaction_seq LIMIT $4
		)
		SELECT c.id, c.reaction_seq, r.emoji, count(r.user_id),
		       coalesce(bool_or(r.user_id = $3), false)
		FROM changed c
		LEFT JOIN message_reactions r ON r.message_id = c.id
		GROUP BY c.id, c.reaction_seq, r.emoji
		ORDER BY c.reaction_seq, min(r.created_at)`, convID, afterSeq, viewerID, limit)
	if err != nil {
		return nil, fmt.Errorf("reaksi susulan: %w", err)
	}
	defer rows.Close()

	out := []MessageReactions{}
	index := map[uuid.UUID]int{}
	for rows.Next() {
		var (
			id    uuid.UUID
			seq   int64
			emoji *string
			r     ReactionSummary
		)
		if err := rows.Scan(&id, &seq, &emoji, &r.Count, &r.Mine); err != nil {
			return nil, err
		}

		at, ok := index[id]
		if !ok {
			at = len(out)
			index[id] = at
			out = append(out, MessageReactions{
				MessageID: id, ReactionSeq: seq, Reactions: []ReactionSummary{},
			})
		}
		// emoji NULL adalah baris dari LEFT JOIN yang tidak menemukan pasangan:
		// pesan yang reaksinya sudah habis. Entri kosongnya tetap dikirim.
		if emoji != nil {
			r.Emoji = *emoji
			out[at].Reactions = append(out[at].Reactions, r)
		}
	}
	return out, rows.Err()
}
