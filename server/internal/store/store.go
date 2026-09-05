package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound  = errors.New("not found")
	ErrConflict  = errors.New("conflict")
	ErrForbidden = errors.New("forbidden")
)

type Store struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// ---------- users & sessions ----------

func (s *Store) CreateUser(ctx context.Context, username, displayName, passwordHash string) (User, error) {
	u := User{ID: uuid.New(), Username: username, DisplayName: displayName}
	err := s.pool.QueryRow(ctx, `
		INSERT INTO users (id, username, display_name, password_hash)
		VALUES ($1, $2, $3, $4)
		RETURNING created_at`,
		u.ID, username, displayName, passwordHash,
	).Scan(&u.CreatedAt)

	if isUniqueViolation(err) {
		return User{}, ErrConflict
	}
	if err != nil {
		return User{}, fmt.Errorf("create user: %w", err)
	}
	return u, nil
}

// UserByUsername mengembalikan user beserta hash password-nya untuk verifikasi login.
func (s *Store) UserByUsername(ctx context.Context, username string) (User, string, error) {
	var u User
	var hash string
	err := s.pool.QueryRow(ctx, `
		SELECT id, username, display_name, password_hash, created_at
		FROM users WHERE lower(username) = lower($1)`, username,
	).Scan(&u.ID, &u.Username, &u.DisplayName, &hash, &u.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, "", ErrNotFound
	}
	if err != nil {
		return User{}, "", fmt.Errorf("user by username: %w", err)
	}
	return u, hash, nil
}

func (s *Store) CreateSession(ctx context.Context, tokenHash []byte, userID uuid.UUID, expiresAt time.Time) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO sessions (token_hash, user_id, expires_at) VALUES ($1, $2, $3)`,
		tokenHash, userID, expiresAt)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

func (s *Store) UserBySession(ctx context.Context, tokenHash []byte) (User, error) {
	var u User
	err := s.pool.QueryRow(ctx, `
		SELECT u.id, u.username, u.display_name, u.created_at
		FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = $1 AND s.expires_at > now()`, tokenHash,
	).Scan(&u.ID, &u.Username, &u.DisplayName, &u.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("user by session: %w", err)
	}
	return u, nil
}

func (s *Store) DeleteSession(ctx context.Context, tokenHash []byte) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, tokenHash)
	return err
}

// SearchUsers dipakai untuk memilih lawan bicara saat memulai percakapan baru.
func (s *Store) SearchUsers(ctx context.Context, query string, exclude uuid.UUID, limit int) ([]User, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, username, display_name, created_at
		FROM users
		WHERE id <> $1 AND (username ILIKE $2 OR display_name ILIKE $2)
		ORDER BY username LIMIT $3`,
		exclude, "%"+query+"%", limit)
	if err != nil {
		return nil, fmt.Errorf("search users: %w", err)
	}
	defer rows.Close()

	users := []User{}
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.DisplayName, &u.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// ---------- conversations ----------

// directKey menghasilkan kunci yang sama untuk sepasang user, ke arah mana pun
// percakapan dimulai — dasar dari unique index yang mencegah DM ganda.
func directKey(a, b uuid.UUID) string {
	x, y := a.String(), b.String()
	if x > y {
		x, y = y, x
	}
	return x + ":" + y
}

// GetOrCreateDirect mengembalikan DM antara dua user, membuatnya bila belum ada.
// Aman dipanggil bersamaan dari dua sisi: unique index pada direct_key membuat
// pemenang balapan menang, yang kalah membaca ulang baris yang sudah ada.
func (s *Store) GetOrCreateDirect(ctx context.Context, me, peer uuid.UUID) (uuid.UUID, error) {
	if me == peer {
		return uuid.Nil, fmt.Errorf("%w: tidak bisa membuat DM dengan diri sendiri", ErrConflict)
	}
	key := directKey(me, peer)

	var id uuid.UUID
	err := s.pool.QueryRow(ctx,
		`SELECT id FROM conversations WHERE direct_key = $1`, key).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, fmt.Errorf("lookup direct: %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op setelah commit

	id = uuid.New()
	_, err = tx.Exec(ctx, `
		INSERT INTO conversations (id, type, created_by, direct_key)
		VALUES ($1, 'direct', $2, $3)`, id, me, key)
	if isUniqueViolation(err) {
		// Sisi lain menang balapan; pakai punya mereka.
		var existing uuid.UUID
		if err := s.pool.QueryRow(ctx,
			`SELECT id FROM conversations WHERE direct_key = $1`, key).Scan(&existing); err != nil {
			return uuid.Nil, fmt.Errorf("reread direct: %w", err)
		}
		return existing, nil
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("insert direct: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO conversation_members (conversation_id, user_id, role)
		VALUES ($1, $2, 'member'), ($1, $3, 'member')`, id, me, peer); err != nil {
		return uuid.Nil, fmt.Errorf("insert direct members: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

func (s *Store) CreateGroup(ctx context.Context, creator uuid.UUID, title string, members []uuid.UUID) (uuid.UUID, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	id := uuid.New()
	if _, err := tx.Exec(ctx, `
		INSERT INTO conversations (id, type, title, created_by)
		VALUES ($1, 'group', $2, $3)`, id, title, creator); err != nil {
		return uuid.Nil, fmt.Errorf("insert group: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO conversation_members (conversation_id, user_id, role)
		VALUES ($1, $2, 'owner')`, id, creator); err != nil {
		return uuid.Nil, fmt.Errorf("insert owner: %w", err)
	}

	for _, m := range members {
		if m == creator {
			continue
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO conversation_members (conversation_id, user_id, role)
			VALUES ($1, $2, 'member')
			ON CONFLICT DO NOTHING`, id, m); err != nil {
			return uuid.Nil, fmt.Errorf("insert member: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

// ListConversations mengembalikan percakapan milik user, terbaru di atas,
// lengkap dengan pesan terakhir dan jumlah belum dibaca dalam satu query.
func (s *Store) ListConversations(ctx context.Context, userID uuid.UUID) ([]Conversation, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT c.id, c.type, c.title, c.last_seq, cm.last_read_seq, c.created_at,
		       lm.id, lm.seq, lm.sender_id, lm.body, lm.created_at, lm.edited_at, lm.deleted_at,
		       peer.id, peer.username, peer.display_name, peer.created_at
		FROM conversation_members cm
		JOIN conversations c ON c.id = cm.conversation_id
		LEFT JOIN LATERAL (
			SELECT id, seq, sender_id, body, created_at, edited_at, deleted_at
			FROM messages WHERE conversation_id = c.id
			ORDER BY seq DESC LIMIT 1
		) lm ON true
		LEFT JOIN LATERAL (
			SELECT u.id, u.username, u.display_name, u.created_at
			FROM conversation_members other
			JOIN users u ON u.id = other.user_id
			WHERE other.conversation_id = c.id AND other.user_id <> cm.user_id
			LIMIT 1
		) peer ON c.type = 'direct'
		WHERE cm.user_id = $1
		ORDER BY COALESCE(lm.created_at, c.created_at) DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list conversations: %w", err)
	}
	defer rows.Close()

	out := []Conversation{}
	for rows.Next() {
		var (
			c        Conversation
			created  time.Time
			msgID    *uuid.UUID
			msgSeq   *int64
			msgFrom  *uuid.UUID
			msgBody  *string
			msgAt    *time.Time
			msgEdit  *time.Time
			msgDel   *time.Time
			peerID   *uuid.UUID
			peerUser *string
			peerName *string
			peerAt   *time.Time
		)
		if err := rows.Scan(
			&c.ID, &c.Type, &c.Title, &c.LastSeq, &c.LastReadSeq, &created,
			&msgID, &msgSeq, &msgFrom, &msgBody, &msgAt, &msgEdit, &msgDel,
			&peerID, &peerUser, &peerName, &peerAt,
		); err != nil {
			return nil, err
		}

		c.Unread = c.LastSeq - c.LastReadSeq
		c.UpdatedAt = created

		if msgID != nil {
			c.LastMessage = &Message{
				ID: *msgID, ConversationID: c.ID, Seq: *msgSeq, SenderID: *msgFrom,
				Body: *msgBody, CreatedAt: *msgAt, EditedAt: msgEdit, DeletedAt: msgDel,
			}
			c.UpdatedAt = *msgAt
		}
		if peerID != nil {
			c.Peer = &User{ID: *peerID, Username: *peerUser, DisplayName: *peerName, CreatedAt: *peerAt}
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) IsMember(ctx context.Context, convID, userID uuid.UUID) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM conversation_members
			WHERE conversation_id = $1 AND user_id = $2
		)`, convID, userID).Scan(&ok)
	return ok, err
}

// MemberIDs dipakai hub untuk menentukan siapa saja penerima siaran.
func (s *Store) MemberIDs(ctx context.Context, convID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT user_id FROM conversation_members WHERE conversation_id = $1`, convID)
	if err != nil {
		return nil, fmt.Errorf("member ids: %w", err)
	}
	defer rows.Close()

	ids := []uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *Store) Members(ctx context.Context, convID uuid.UUID) ([]Member, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT u.id, u.username, u.display_name, cm.role, cm.last_read_seq
		FROM conversation_members cm JOIN users u ON u.id = cm.user_id
		WHERE cm.conversation_id = $1
		ORDER BY u.display_name`, convID)
	if err != nil {
		return nil, fmt.Errorf("members: %w", err)
	}
	defer rows.Close()

	out := []Member{}
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.UserID, &m.Username, &m.DisplayName, &m.Role, &m.LastReadSeq); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ConversationsOf mengembalikan semua percakapan milik user — dipakai saat
// koneksi WebSocket dibuka untuk mendaftarkan langganan siaran.
func (s *Store) ConversationsOf(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT conversation_id FROM conversation_members WHERE user_id = $1`, userID)
	if err != nil {
		return nil, fmt.Errorf("conversations of: %w", err)
	}
	defer rows.Close()

	ids := []uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ---------- messages ----------

// SendMessage menyisipkan pesan dan mengalokasikan `seq` berikutnya.
//
// Idempoten: `id` datang dari client, jadi pengiriman ulang setelah timeout
// jaringan mengembalikan pesan yang sudah tersimpan (created=false) alih-alih
// membuat duplikat.
//
// SELECT ... FOR UPDATE mengunci baris percakapan sehingga pengiriman serentak
// di ruang yang sama diserialisasi — `seq` dijamin berurutan tanpa lompatan,
// dan pengecekan duplikat di dalam kunci selalu akurat.
func (s *Store) SendMessage(ctx context.Context, id, convID, senderID uuid.UUID, body string) (Message, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Message{}, false, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var lastSeq int64
	err = tx.QueryRow(ctx,
		`SELECT last_seq FROM conversations WHERE id = $1 FOR UPDATE`, convID).Scan(&lastSeq)
	if errors.Is(err, pgx.ErrNoRows) {
		return Message{}, false, ErrNotFound
	}
	if err != nil {
		return Message{}, false, fmt.Errorf("lock conversation: %w", err)
	}

	var existing Message
	err = tx.QueryRow(ctx, `
		SELECT id, conversation_id, seq, sender_id, body, created_at, edited_at, deleted_at
		FROM messages WHERE id = $1`, id,
	).Scan(&existing.ID, &existing.ConversationID, &existing.Seq, &existing.SenderID,
		&existing.Body, &existing.CreatedAt, &existing.EditedAt, &existing.DeletedAt)
	if err == nil {
		if existing.ConversationID != convID || existing.SenderID != senderID {
			return Message{}, false, ErrConflict
		}
		return existing, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Message{}, false, fmt.Errorf("check duplicate: %w", err)
	}

	seq := lastSeq + 1
	m := Message{ID: id, ConversationID: convID, Seq: seq, SenderID: senderID, Body: body}
	if err := tx.QueryRow(ctx, `
		INSERT INTO messages (id, conversation_id, seq, sender_id, body)
		VALUES ($1, $2, $3, $4, $5) RETURNING created_at`,
		id, convID, seq, senderID, body,
	).Scan(&m.CreatedAt); err != nil {
		return Message{}, false, fmt.Errorf("insert message: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`UPDATE conversations SET last_seq = $1 WHERE id = $2`, seq, convID); err != nil {
		return Message{}, false, fmt.Errorf("bump last_seq: %w", err)
	}

	// Pengirim otomatis sudah membaca pesannya sendiri.
	if _, err := tx.Exec(ctx, `
		UPDATE conversation_members SET last_read_seq = $1
		WHERE conversation_id = $2 AND user_id = $3 AND last_read_seq < $1`,
		seq, convID, senderID); err != nil {
		return Message{}, false, fmt.Errorf("self read: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return Message{}, false, err
	}
	return m, true, nil
}

// ListMessages mengambil riwayat dengan cursor pagination.
// beforeSeq = 0 berarti mulai dari pesan terbaru. Hasil dikembalikan menaik
// (lama -> baru) supaya client bisa langsung menempelkannya ke atas daftar.
func (s *Store) ListMessages(ctx context.Context, convID uuid.UUID, beforeSeq int64, limit int) ([]Message, error) {
	var sb strings.Builder
	sb.WriteString(`
		SELECT id, conversation_id, seq, sender_id, body, created_at, edited_at, deleted_at
		FROM messages WHERE conversation_id = $1`)
	args := []any{convID}
	if beforeSeq > 0 {
		sb.WriteString(` AND seq < $2`)
		args = append(args, beforeSeq)
	}
	sb.WriteString(fmt.Sprintf(` ORDER BY seq DESC LIMIT $%d`, len(args)+1))
	args = append(args, limit)

	rows, err := s.pool.Query(ctx, sb.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}
	defer rows.Close()

	out := []Message{}
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.Seq, &m.SenderID,
			&m.Body, &m.CreatedAt, &m.EditedAt, &m.DeletedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

// MessagesSince dipakai saat client reconnect: kirim semua yang terlewat.
func (s *Store) MessagesSince(ctx context.Context, convID uuid.UUID, afterSeq int64, limit int) ([]Message, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, conversation_id, seq, sender_id, body, created_at, edited_at, deleted_at
		FROM messages WHERE conversation_id = $1 AND seq > $2
		ORDER BY seq ASC LIMIT $3`, convID, afterSeq, limit)
	if err != nil {
		return nil, fmt.Errorf("messages since: %w", err)
	}
	defer rows.Close()

	out := []Message{}
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.Seq, &m.SenderID,
			&m.Body, &m.CreatedAt, &m.EditedAt, &m.DeletedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) EditMessage(ctx context.Context, id, senderID uuid.UUID, body string) (Message, error) {
	var m Message
	err := s.pool.QueryRow(ctx, `
		UPDATE messages SET body = $1, edited_at = now()
		WHERE id = $2 AND sender_id = $3 AND deleted_at IS NULL
		RETURNING id, conversation_id, seq, sender_id, body, created_at, edited_at, deleted_at`,
		body, id, senderID,
	).Scan(&m.ID, &m.ConversationID, &m.Seq, &m.SenderID, &m.Body, &m.CreatedAt, &m.EditedAt, &m.DeletedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return Message{}, ErrForbidden
	}
	if err != nil {
		return Message{}, fmt.Errorf("edit message: %w", err)
	}
	return m, nil
}

// DeleteMessage adalah soft delete: baris tetap ada agar `seq` tidak bolong dan
// client yang sedang offline tetap bisa menyinkronkan status "dihapus".
func (s *Store) DeleteMessage(ctx context.Context, id, senderID uuid.UUID) (Message, error) {
	var m Message
	err := s.pool.QueryRow(ctx, `
		UPDATE messages SET body = '', deleted_at = now()
		WHERE id = $1 AND sender_id = $2 AND deleted_at IS NULL
		RETURNING id, conversation_id, seq, sender_id, body, created_at, edited_at, deleted_at`,
		id, senderID,
	).Scan(&m.ID, &m.ConversationID, &m.Seq, &m.SenderID, &m.Body, &m.CreatedAt, &m.EditedAt, &m.DeletedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return Message{}, ErrForbidden
	}
	if err != nil {
		return Message{}, fmt.Errorf("delete message: %w", err)
	}
	return m, nil
}

// MarkRead memajukan penanda baca. Tidak pernah mundur, supaya pesan yang
// datang tidak berurutan tidak membuat badge unread muncul lagi.
func (s *Store) MarkRead(ctx context.Context, convID, userID uuid.UUID, seq int64) (int64, error) {
	var current int64
	err := s.pool.QueryRow(ctx, `
		UPDATE conversation_members SET last_read_seq = GREATEST(last_read_seq, $1)
		WHERE conversation_id = $2 AND user_id = $3
		RETURNING last_read_seq`, seq, convID, userID).Scan(&current)

	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrForbidden
	}
	if err != nil {
		return 0, fmt.Errorf("mark read: %w", err)
	}
	return current, nil
}

// ContactIDs mengembalikan user yang berbagi minimal satu percakapan dengan
// userID. Ini himpunan orang yang perlu tahu saat user online/offline —
// presence tidak disiarkan ke seluruh pengguna aplikasi.
func (s *Store) ContactIDs(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT other.user_id
		FROM conversation_members mine
		JOIN conversation_members other ON other.conversation_id = mine.conversation_id
		WHERE mine.user_id = $1 AND other.user_id <> $1`, userID)
	if err != nil {
		return nil, fmt.Errorf("contact ids: %w", err)
	}
	defer rows.Close()

	ids := []uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
