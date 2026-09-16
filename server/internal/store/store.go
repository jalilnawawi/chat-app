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
	// ErrInvalid adalah masukan yang bentuknya salah — bukan bentrok, bukan
	// soal izin. Dipisahkan supaya "judul grup kosong" tidak dijawab 409
	// "sudah ada atau bentrok", yang mengirim orang mencari bentrokan yang
	// tidak pernah ada.
	ErrInvalid = errors.New("invalid")
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
	// Batas yang sama dengan jalur tambah anggota. Dua pintu masuk ke keadaan
	// yang sama dengan batas yang berbeda berarti salah satunya bisa dipakai
	// untuk melewati yang lain.
	if len(withoutDuplicates(members, creator))+1 > maxGroupMembers {
		return uuid.Nil, fmt.Errorf("%w: grup maksimal %d anggota", ErrConflict, maxGroupMembers)
	}

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
		       cm.mention_seq, cm.mention_ack_seq,
		       lm.id, lm.seq, lm.sender_id, lm.body, lm.attachments,
		       lm.created_at, lm.edited_at, lm.deleted_at, lm.kind, lm.system_event,
		       peer.id, peer.username, peer.display_name, peer.created_at
		FROM conversation_members cm
		JOIN conversations c ON c.id = cm.conversation_id
		LEFT JOIN LATERAL (
			SELECT id, seq, sender_id, body, attachments, created_at, edited_at,
			       deleted_at, kind, system_event
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
			msgAtt   []Attachment
			msgAt    *time.Time
			msgEdit  *time.Time
			msgDel   *time.Time
			msgKind  *string
			msgSys   *SystemEvent
			peerID   *uuid.UUID
			peerUser *string
			peerName *string
			peerAt   *time.Time
		)
		if err := rows.Scan(
			&c.ID, &c.Type, &c.Title, &c.LastSeq, &c.LastReadSeq, &created,
			&c.MentionSeq, &c.MentionAckSeq,
			&msgID, &msgSeq, &msgFrom, &msgBody, &msgAtt, &msgAt, &msgEdit, &msgDel,
			&msgKind, &msgSys,
			&peerID, &peerUser, &peerName, &peerAt,
		); err != nil {
			return nil, err
		}

		c.Unread = c.LastSeq - c.LastReadSeq
		c.UpdatedAt = created

		if msgID != nil {
			// Sengaja tanpa kutipan, sebutan, maupun reaksi: ini cuma baris
			// pratinjau di sidebar, dan tiga metadata tambahan untuk satu baris
			// teks yang dipotong berarti membayar join dan agregasi untuk
			// SETIAP percakapan yang dimiliki seseorang.
			c.LastMessage = &Message{
				ID: *msgID, ConversationID: c.ID, Seq: *msgSeq, SenderID: *msgFrom,
				Body: *msgBody, Attachments: msgAtt,
				CreatedAt: *msgAt, EditedAt: msgEdit, DeletedAt: msgDel,
				Mentions: []uuid.UUID{}, Reactions: []ReactionSummary{},
				Kind: deref(msgKind, "user"), SystemEvent: msgSys,
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

// messageCols adalah satu-satunya tempat daftar kolom pesan ditulis.
//
// Sebelumnya daftar ini diulang di enam query, dan menambah satu kolom berarti
// mengubah enam tempat sekaligus enam urutan Scan yang harus tetap cocok.
// Menambahkan `attachments` di Fase 7 adalah kali pertama itu benar-benar
// diuji — dan yang membuatnya aman adalah daftar kolom dan urutan Scan hidup
// bersebelahan di bawah ini.
//
// Sejak Fase 9 daftarnya butuh alias tabel: query pesan sekarang selalu
// menyertakan self-join ke tabel yang sama, dan `id` tanpa kualifikasi menjadi
// ambigu. Alasan yang sama dengan attachmentCols di attachments.go — aliasnya
// ditempelkan ke SETIAP kolom, bukan hanya ke yang pertama.
func messageCols(alias string) string {
	return prefix(alias, "id", "conversation_id", "seq", "sender_id", "body",
		"attachments", "created_at", "edited_at", "deleted_at",
		"mentions", "mentions_all", "reaction_seq", "kind", "system_event")
}

// replyCols adalah kolom pesan YANG DIBALAS, dibaca lewat self-join.
//
// Isinya tidak pernah disalin ke baris pembalasnya, jadi kutipan selalu
// menampilkan keadaan terbaru: yang sudah diedit tampil versi barunya, yang
// sudah dihapus tampil sebagai "pesan dihapus".
func replyCols(alias string) string {
	return prefix(alias, "id", "seq", "sender_id", "body", "attachments", "deleted_at")
}

func prefix(alias string, cols ...string) string {
	if alias != "" {
		alias += "."
	}
	out := make([]string, len(cols))
	for i, c := range cols {
		out[i] = alias + c
	}
	return strings.Join(out, ", ")
}

// messageSelect menyusun pembacaan pesan lengkap dengan kutipannya.
//
// Self-join-nya SATU per query, bukan satu per pesan: dia menempel pada primary
// key tabel yang sama, dan Postgres menjawabnya dengan satu lookup index per
// baris yang memang punya `reply_to_id`. Halaman berisi seratus pesan yang tak
// satu pun membalas apa pun tidak membayar apa-apa untuk join ini.
func messageSelect(where string) string {
	return `SELECT ` + messageCols("m") + `, ` + replyCols("rep") + `
		FROM messages m
		LEFT JOIN messages rep ON rep.id = m.reply_to_id
		WHERE ` + where
}

// scanMessage menerima pgx.Row maupun pgx.Rows — keduanya punya Scan yang sama.
func scanMessage(row pgx.Row, m *Message) error {
	var (
		repID      *uuid.UUID
		repSeq     *int64
		repSender  *uuid.UUID
		repBody    *string
		repAtts    []Attachment
		repDeleted *time.Time
	)
	if err := row.Scan(&m.ID, &m.ConversationID, &m.Seq, &m.SenderID, &m.Body,
		&m.Attachments, &m.CreatedAt, &m.EditedAt, &m.DeletedAt,
		&m.Mentions, &m.MentionsAll, &m.ReactionSeq, &m.Kind, &m.SystemEvent,
		&repID, &repSeq, &repSender, &repBody, &repAtts, &repDeleted,
	); err != nil {
		return err
	}

	if m.Mentions == nil {
		m.Mentions = []uuid.UUID{}
	}
	// Reaksi diisi terpisah oleh pemanggil yang tahu siapa pembacanya; yang
	// tidak tahu tetap mengirim larik kosong, bukan null.
	m.Reactions = []ReactionSummary{}

	if repID != nil {
		m.ReplyTo = &ReplyPreview{
			ID: *repID, Seq: *repSeq, SenderID: *repSender,
			Body: *repBody, Deleted: repDeleted != nil,
		}
		if !m.ReplyTo.Deleted && m.ReplyTo.Body == "" {
			m.ReplyTo.Kind = attachmentKind(repAtts)
		}
	}
	return nil
}

// attachmentKind memberi satu kata untuk pesan yang isinya hanya lampiran.
// Kutipan tanpa ini tampil sebagai baris kosong — yang terbaca seperti pesan
// kosong, bukan seperti foto yang sedang dibalas.
func attachmentKind(atts []Attachment) string {
	if len(atts) == 0 {
		return ""
	}
	switch mime := atts[0].MIME; {
	case strings.HasPrefix(mime, "image/"):
		return "image"
	case strings.HasPrefix(mime, "video/"):
		return "video"
	case strings.HasPrefix(mime, "audio/"):
		return "audio"
	default:
		return "file"
	}
}

// SendParams mengumpulkan segala yang menentukan sebuah pesan.
//
// Ditulis sebagai struct sejak Fase 9 karena daftarnya tumbuh dari lima jadi
// delapan, dan tiga di antaranya adalah id yang tipenya sama persis — `id`,
// `convID`, `senderID`, dan sekarang `replyToID`. Argumen posisional yang
// tipenya seragam adalah tempat di mana dua parameter tertukar tanpa satu pun
// peringatan compiler.
type SendParams struct {
	ID             uuid.UUID
	ConversationID uuid.UUID
	SenderID       uuid.UUID
	Body           string

	// AttachmentIDs menunjuk lampiran yang sudah diunggah lebih dulu.
	AttachmentIDs []uuid.UUID

	// ReplyToID nil berarti bukan balasan. Bila diisi, pesannya WAJIB berada di
	// percakapan yang sama — lihat pemeriksaannya di bawah.
	ReplyToID *uuid.UUID

	// Mentions datang dari client dan TIDAK dipercaya: tiap id diperiksa
	// keanggotaannya di dalam transaksi ini.
	Mentions    []uuid.UUID
	MentionsAll bool
}

// SendMessage menyisipkan pesan dan mengalokasikan `seq` berikutnya.
//
// Idempoten: `id` datang dari client, jadi pengiriman ulang setelah timeout
// jaringan mengembalikan pesan yang sudah tersimpan (created=false) alih-alih
// membuat duplikat.
//
// SELECT ... FOR UPDATE mengunci baris percakapan sehingga pengiriman serentak
// di ruang yang sama diserialisasi — `seq` dijamin berurutan tanpa lompatan,
// dan pengecekan duplikat di dalam kunci selalu akurat.
//
// Lampiran, kutipan, dan sebutan semuanya diselesaikan di dalam transaksi yang
// sama: sebuah pesan tidak boleh pernah terlihat tanpa salah satunya, sekalipun
// untuk sepersekian detik.
func (s *Store) SendMessage(ctx context.Context, p SendParams) (Message, bool, error) {
	convID, senderID := p.ConversationID, p.SenderID

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Message{}, false, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var (
		lastSeq  int64
		convType string
	)
	// Keanggotaan diperiksa DI SINI, di bawah kunci yang sama yang
	// mengalokasikan `seq` — bukan hanya di handler HTTP sebelum transaksi ini
	// dibuka.
	//
	// Alasannya sama persis dengan alasan pemeriksaan sebutan menyatu dengan
	// penulisannya di markMentioned: pemeriksaan yang terpisah dari tindakannya
	// menyisakan celah di antara keduanya. Di sini celah itu berarti orang yang
	// baru saja dikeluarkan dari grup masih bisa menyelipkan satu pesan, karena
	// pengeluarannya terjadi persis setelah handler memastikan dia anggota.
	//
	// JOIN, bukan EXISTS: yang bukan anggota tidak menghasilkan baris sama
	// sekali, jadi jawabannya jatuh ke cabang yang sama dengan percakapan yang
	// memang tidak ada — dan itu memang jawaban yang benar untuknya.
	err = tx.QueryRow(ctx, `
		SELECT c.last_seq, c.type
		FROM conversations c
		JOIN conversation_members cm
		  ON cm.conversation_id = c.id AND cm.user_id = $2
		WHERE c.id = $1
		FOR UPDATE OF c`, convID, senderID,
	).Scan(&lastSeq, &convType)
	if errors.Is(err, pgx.ErrNoRows) {
		return Message{}, false, ErrNotFound
	}
	if err != nil {
		return Message{}, false, fmt.Errorf("lock conversation: %w", err)
	}

	var existing Message
	err = scanMessage(tx.QueryRow(ctx, messageSelect(`m.id = $1`), p.ID), &existing)
	if err == nil {
		if existing.ConversationID != convID || existing.SenderID != senderID {
			return Message{}, false, ErrConflict
		}
		return existing, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Message{}, false, fmt.Errorf("check duplicate: %w", err)
	}

	// @semua hanya punya arti di grup. Di DM dia cuma cara lain untuk menembus
	// peredam dering, dan menembusnya tanpa menyebut siapa pun.
	mentionsAll := p.MentionsAll && convType == "group"

	// Pengirim dikeluarkan dari daftar: yang disimpan di sini adalah daftar
	// orang yang perlu DIBANGUNKAN, dan tidak ada yang perlu dibangunkan oleh
	// dirinya sendiri.
	mentions := withoutDuplicates(p.Mentions, senderID)

	replyTo, err := replyPreview(ctx, tx, p.ReplyToID, convID)
	if err != nil {
		return Message{}, false, err
	}

	seq := lastSeq + 1
	m := Message{
		ID: p.ID, ConversationID: convID, Seq: seq, SenderID: senderID,
		Body: p.Body, Attachments: []Attachment{},
		ReplyTo: replyTo, Mentions: mentions, MentionsAll: mentionsAll,
		Reactions: []ReactionSummary{}, Kind: "user",
	}
	if err := tx.QueryRow(ctx, `
		INSERT INTO messages (id, conversation_id, seq, sender_id, body,
		                      reply_to_id, mentions, mentions_all)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING created_at`,
		p.ID, convID, seq, senderID, p.Body, p.ReplyToID, mentions, mentionsAll,
	).Scan(&m.CreatedAt); err != nil {
		return Message{}, false, fmt.Errorf("insert message: %w", err)
	}

	if err := markMentioned(ctx, tx, convID, senderID, seq, mentions, mentionsAll); err != nil {
		return Message{}, false, err
	}

	// Lampiran dipasang setelah baris pesan ada — foreign key-nya menuntut itu —
	// lalu hasilnya disalin ke kolom jsonb milik pesan. Salinan itu yang dibaca
	// saat menampilkan riwayat, sehingga memuat seratus pesan tetap satu query.
	if m.Attachments, err = claimAttachments(ctx, tx, p.ID, senderID, p.AttachmentIDs); err != nil {
		return Message{}, false, err
	}
	if len(m.Attachments) > 0 {
		if _, err := tx.Exec(ctx,
			`UPDATE messages SET attachments = $1 WHERE id = $2`, m.Attachments, p.ID); err != nil {
			return Message{}, false, fmt.Errorf("simpan lampiran pesan: %w", err)
		}
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

// withoutDuplicates membersihkan daftar id: buang yang kembar dan buang
// `drop`, dengan urutan aslinya tetap terjaga.
func withoutDuplicates(ids []uuid.UUID, drop uuid.UUID) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(ids))
	seen := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		if id == drop {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

// replyPreview membaca pesan yang dibalas, sekaligus MEMERIKSA bahwa pesan itu
// berada di percakapan yang sama.
//
// Pemeriksaan itu bukan kerapian, melainkan gerbang kebocoran. Tanpanya, siapa
// pun bisa mengirim pesan ke percakapannya sendiri sambil mengutip id pesan
// dari percakapan yang tidak dia ikuti — dan isi pesan itu akan tampil rapi di
// dalam gelembung kutipan. Bentuknya persis seperti fitur; akibatnya adalah
// membaca percakapan orang lain satu pesan pada satu waktu.
//
// `conversation_id = $2` di WHERE membuat jawabannya sama untuk pesan yang
// tidak ada dan pesan yang tidak boleh dilihat: tidak ditemukan.
func replyPreview(ctx context.Context, tx pgx.Tx, replyToID *uuid.UUID, convID uuid.UUID) (*ReplyPreview, error) {
	if replyToID == nil {
		return nil, nil
	}

	var (
		rp        ReplyPreview
		atts      []Attachment
		deletedAt *time.Time
	)
	// `kind = 'user'` menutup satu jalur yang tidak masuk akal: membalas catatan
	// sistem. Catatan itu bukan ucapan siapa pun, dan mengutipnya menghasilkan
	// gelembung yang mengaku dikatakan oleh orang yang cuma jadi pelakunya.
	err := tx.QueryRow(ctx, `
		SELECT `+replyCols("")+`
		FROM messages
		WHERE id = $1 AND conversation_id = $2 AND kind = 'user'`, *replyToID, convID,
	).Scan(&rp.ID, &rp.Seq, &rp.SenderID, &rp.Body, &atts, &deletedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("%w: pesan yang dibalas tidak ada di percakapan ini", ErrForbidden)
	}
	if err != nil {
		return nil, fmt.Errorf("baca pesan yang dibalas: %w", err)
	}

	rp.Deleted = deletedAt != nil
	if !rp.Deleted && rp.Body == "" {
		rp.Kind = attachmentKind(atts)
	}
	return &rp, nil
}

// markMentioned memajukan penanda "ada yang menyebut kamu" pada anggota yang
// disebut, sekaligus MEMVALIDASI bahwa mereka memang anggota.
//
// Validasi dan penulisan terjadi dalam satu UPDATE, bukan dua langkah: jumlah
// baris yang tersentuh lebih sedikit dari yang diminta berarti ada id yang
// bukan anggota percakapan ini — dan pesannya dibatalkan seluruhnya. Memisahkan
// keduanya berarti ada celah di antara "sudah diperiksa" dan "sudah ditulis",
// tepat pada pemeriksaan yang menentukan siapa boleh dibangunkan.
func markMentioned(ctx context.Context, tx pgx.Tx, convID, senderID uuid.UUID, seq int64, mentions []uuid.UUID, all bool) error {
	if len(mentions) > 0 {
		tag, err := tx.Exec(ctx, `
			UPDATE conversation_members SET mention_seq = GREATEST(mention_seq, $1)
			WHERE conversation_id = $2 AND user_id = ANY($3)`, seq, convID, mentions)
		if err != nil {
			return fmt.Errorf("tandai sebutan: %w", err)
		}
		if tag.RowsAffected() != int64(len(mentions)) {
			return fmt.Errorf("%w: menyebut orang yang bukan anggota percakapan ini", ErrForbidden)
		}
	}

	if all {
		if _, err := tx.Exec(ctx, `
			UPDATE conversation_members SET mention_seq = GREATEST(mention_seq, $1)
			WHERE conversation_id = $2 AND user_id <> $3`, seq, convID, senderID); err != nil {
			return fmt.Errorf("tandai sebutan semua: %w", err)
		}
	}
	return nil
}

// AckMentions mencatat bahwa sebutan sudah benar-benar sampai ke mata orangnya.
//
// Terpisah dari MarkRead dengan sengaja, dan itu seluruh alasan kolom ini ada.
// MarkRead bergerak saat percakapannya dibuka; ini baru bergerak saat pesan
// yang menyebut namanya benar-benar terlihat di layar. Membuka ruang sebentar
// untuk mengintip tidak boleh menghapus panggilan yang belum dibaca.
func (s *Store) AckMentions(ctx context.Context, convID, userID uuid.UUID, seq int64) (int64, error) {
	var current int64
	err := s.pool.QueryRow(ctx, `
		UPDATE conversation_members SET mention_ack_seq = GREATEST(mention_ack_seq, $1)
		WHERE conversation_id = $2 AND user_id = $3
		RETURNING mention_ack_seq`, seq, convID, userID).Scan(&current)

	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrForbidden
	}
	if err != nil {
		return 0, fmt.Errorf("ack sebutan: %w", err)
	}
	return current, nil
}

// ListMessages mengambil riwayat dengan cursor pagination.
// beforeSeq = 0 berarti mulai dari pesan terbaru. Hasil dikembalikan menaik
// (lama -> baru) supaya client bisa langsung menempelkannya ke atas daftar.
// viewerID dipakai untuk mengisi kolom "Mine" pada ringkasan reaksi — satu-
// satunya bagian dari sebuah pesan yang jawabannya berbeda per pembaca.
func (s *Store) ListMessages(ctx context.Context, convID, viewerID uuid.UUID, beforeSeq int64, limit int) ([]Message, error) {
	where := `m.conversation_id = $1`
	args := []any{convID}
	if beforeSeq > 0 {
		where += ` AND m.seq < $2`
		args = append(args, beforeSeq)
	}
	query := messageSelect(where) + fmt.Sprintf(` ORDER BY m.seq DESC LIMIT $%d`, len(args)+1)
	args = append(args, limit)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}
	defer rows.Close()

	out := []Message{}
	for rows.Next() {
		var m Message
		if err := scanMessage(rows, &m); err != nil {
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

	if err := s.attachReactions(ctx, out, viewerID); err != nil {
		return nil, err
	}
	return out, nil
}

// MessagesSince dipakai saat client reconnect: kirim semua yang terlewat.
func (s *Store) MessagesSince(ctx context.Context, convID, viewerID uuid.UUID, afterSeq int64, limit int) ([]Message, error) {
	rows, err := s.pool.Query(ctx,
		messageSelect(`m.conversation_id = $1 AND m.seq > $2`)+` ORDER BY m.seq ASC LIMIT $3`,
		convID, afterSeq, limit)
	if err != nil {
		return nil, fmt.Errorf("messages since: %w", err)
	}
	defer rows.Close()

	out := []Message{}
	for rows.Next() {
		var m Message
		if err := scanMessage(rows, &m); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if err := s.attachReactions(ctx, out, viewerID); err != nil {
		return nil, err
	}
	return out, nil
}

// EditMessage mengubah isi pesan dan mengembalikannya lengkap dengan kutipannya.
//
// UPDATE-nya dibungkus CTE supaya self-join kutipan tetap bisa dipasang: klausa
// RETURNING tidak bisa menjangkau tabel lain, termasuk tabel yang sama lewat
// alias lain. Tanpa ini, pesan hasil edit akan kembali ke client TANPA kutipan
// yang tadinya ada — dan gelembung kutipannya lenyap begitu pengirimnya
// memperbaiki satu salah ketik.
func (s *Store) EditMessage(ctx context.Context, id, senderID uuid.UUID, body string) (Message, error) {
	var m Message
	err := scanMessage(s.pool.QueryRow(ctx, `
		WITH upd AS (
			UPDATE messages SET body = $1, edited_at = now()
			WHERE id = $2 AND sender_id = $3 AND deleted_at IS NULL AND kind = 'user'
			RETURNING *
		)
		SELECT `+messageCols("m")+`, `+replyCols("rep")+`
		FROM upd m LEFT JOIN messages rep ON rep.id = m.reply_to_id`,
		body, id, senderID), &m)

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
//
// Lampirannya tidak ikut soft delete. Menghapus pesan berarti isinya benar-benar
// pergi, dan berkas yang tertinggal di penyimpanan tidak akan pernah bisa
// dibuang lewat UI mana pun — tidak ada layar yang bisa menampilkannya lagi.
// Melepas `message_id` mengembalikannya ke keadaan yatim, dan penyapu yang sudah
// ada akan membuangnya setelah lewat umur.
func (s *Store) DeleteMessage(ctx context.Context, id, senderID uuid.UUID) (Message, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Message{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var m Message
	err = scanMessage(tx.QueryRow(ctx, `
		WITH upd AS (
			UPDATE messages SET body = '', attachments = '[]'::jsonb, deleted_at = now()
			WHERE id = $1 AND sender_id = $2 AND deleted_at IS NULL AND kind = 'user'
			RETURNING *
		)
		SELECT `+messageCols("m")+`, `+replyCols("rep")+`
		FROM upd m LEFT JOIN messages rep ON rep.id = m.reply_to_id`, id, senderID), &m)

	if errors.Is(err, pgx.ErrNoRows) {
		return Message{}, ErrForbidden
	}
	if err != nil {
		return Message{}, fmt.Errorf("delete message: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`UPDATE attachments SET message_id = NULL WHERE message_id = $1`, id); err != nil {
		return Message{}, fmt.Errorf("lepas lampiran pesan: %w", err)
	}

	// Reaksi ikut pergi, dengan alasan yang sama dengan lampiran: emoji yang
	// menempel pada "pesan ini dihapus" tidak menunjuk apa pun lagi. Jamnya
	// dimajukan supaya yang sedang offline ikut kehilangan hitungannya, bukan
	// menyimpan angka yang sudah tidak punya pesan.
	if err := clearReactions(ctx, tx, m.ConversationID, id); err != nil {
		return Message{}, err
	}
	m.Reactions = []ReactionSummary{}

	if err := tx.Commit(ctx); err != nil {
		return Message{}, err
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

// deref membaca pointer dengan nilai cadangan. Dipakai untuk kolom yang NULL
// bukan karena kosong, melainkan karena LEFT JOIN-nya tidak menemukan baris.
func deref(p *string, fallback string) string {
	if p == nil {
		return fallback
	}
	return *p
}

// Ping dipakai probe kesiapan. Sengaja lewat store, bukan pool langsung, supaya
// paket api tidak perlu tahu apa pun tentang pgx.
func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

// DeleteExpiredSessions membuang sesi yang sudah lewat masa berlakunya dan
// mengembalikan jumlah baris yang terhapus.
func (s *Store) DeleteExpiredSessions(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE expires_at < now()`)
	if err != nil {
		return 0, fmt.Errorf("hapus sesi kedaluwarsa: %w", err)
	}
	return tag.RowsAffected(), nil
}
