package store

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Kelola grup: ganti judul, tambah dan keluarkan anggota, keluar, pindah
// pemilik.
//
// Empat aturan menaungi seluruh berkas ini, dan ketiganya ditulis di sini
// sekali supaya tidak perlu ditebak dari lima fungsi yang mirip:
//
//  1. **DM tidak bisa dikelola.** Setiap operasi di bawah menolak percakapan
//     bertipe 'direct'. Ini bukan kerapian: menambahkan orang ketiga ke sebuah
//     DM akan mempertahankan `direct_key` milik dua orang pertama, sehingga
//     percakapan itu tetap dianggap DM antara mereka berdua — sekaligus berisi
//     orang yang tidak pernah diajak oleh siapa pun. Percakapan pribadi yang
//     diam-diam bertambah pendengarnya adalah bentuk kegagalan terburuk yang
//     bisa dimiliki aplikasi chat.
//
//  2. **Pemilik mengelola, anggota bisa keluar.** Satu kalimat, tanpa kecuali
//     yang perlu dihafal. Mengganti judul, menambah, mengeluarkan, dan
//     memindahkan kepemilikan adalah hak pemilik; keluar adalah hak semua
//     orang atas dirinya sendiri.
//
//  3. **Setiap perubahan meninggalkan catatan.** Lihat 0006_kelola_grup.sql:
//     catatannya adalah baris di tabel messages, jadi dia ikut terbawa oleh
//     riwayat, cursor, dan susulan setelah reconnect tanpa jalur baru.
//
//  4. **Semuanya di dalam satu transaksi dengan baris percakapan terkunci.**
//     Alokasi `seq` untuk catatan sistem memakai kunci yang sama dengan
//     pengiriman pesan biasa, sehingga dua orang yang mengelola grup bersamaan
//     — atau satu yang mengelola sementara yang lain mengirim pesan — tidak
//     pernah menghasilkan `seq` kembar.

// maxGroupMembers membatasi besar sebuah grup.
//
// Angkanya bukan batas teknis melainkan batas akal sehat: dua ratus orang
// adalah jumlah yang masih bisa dibaca daftarnya oleh manusia, dan sekaligus
// jumlah yang sudah dipakai sebagai contoh sejak Fase 9 saat membicarakan
// @semua.
const maxGroupMembers = 200

// GroupChange adalah hasil satu tindakan pengelolaan: catatan yang harus
// disiarkan, dan keadaan grup setelahnya.
type GroupChange struct {
	// Notice adalah pesan sistem yang baru dicatat. Disiarkan seperti pesan
	// biasa, lewat event yang sama persis — client tidak butuh penanganan
	// khusus untuk membuatnya muncul di tempat yang benar.
	Notice Message

	// Title adalah judul grup setelah perubahan.
	Title string

	// Members adalah daftar anggota setelah perubahan, sudah lengkap dengan
	// peran masing-masing.
	Members []Member

	// Departed adalah orang yang BARU SAJA berhenti jadi anggota — dikeluarkan
	// atau keluar sendiri. Dia tidak ada lagi di Members, tapi tetap harus
	// diberi kabar: tanpa itu, percakapan yang sudah bukan miliknya tetap
	// menggantung di sidebar sampai halamannya dimuat ulang.
	Departed *uuid.UUID
}

// lockGroup mengunci baris percakapan dan memastikan dia memang grup.
//
// Mengembalikan `seq` terakhir supaya pemanggil bisa mengalokasikan nomor untuk
// catatan sistemnya di bawah kunci yang sama.
func lockGroup(ctx context.Context, tx pgx.Tx, convID uuid.UUID) (lastSeq int64, title string, err error) {
	var (
		convType string
		t        *string
	)
	err = tx.QueryRow(ctx,
		`SELECT last_seq, type, title FROM conversations WHERE id = $1 FOR UPDATE`, convID,
	).Scan(&lastSeq, &convType, &t)

	if errors.Is(err, pgx.ErrNoRows) {
		return 0, "", ErrNotFound
	}
	if err != nil {
		return 0, "", fmt.Errorf("kunci percakapan: %w", err)
	}
	if convType != "group" {
		// 404, bukan 403: percakapan ini memang tidak punya pengelolaan, dan
		// membedakan "bukan grup" dari "tidak ada" tidak memberi tahu apa pun
		// yang berguna kepada pemanggil yang sah.
		return 0, "", fmt.Errorf("%w: percakapan ini bukan grup", ErrNotFound)
	}
	if t != nil {
		title = *t
	}
	return lastSeq, title, nil
}

// requireOwner memastikan pemanggil adalah pemilik grup, sekaligus mengambil
// namanya untuk dicatat sebagai pelaku.
func requireOwner(ctx context.Context, tx pgx.Tx, convID, actorID uuid.UUID) (SystemParty, error) {
	party, role, err := partyOf(ctx, tx, convID, actorID)
	if err != nil {
		return SystemParty{}, err
	}
	if role != "owner" {
		return SystemParty{}, fmt.Errorf("%w: hanya pemilik grup yang bisa melakukan ini", ErrForbidden)
	}
	return party, nil
}

// partyOf mengambil nama dan peran seorang anggota. Bukan anggota = ErrForbidden.
func partyOf(ctx context.Context, tx pgx.Tx, convID, userID uuid.UUID) (SystemParty, string, error) {
	var (
		p    = SystemParty{ID: userID}
		role string
	)
	err := tx.QueryRow(ctx, `
		SELECT u.display_name, cm.role
		FROM conversation_members cm JOIN users u ON u.id = cm.user_id
		WHERE cm.conversation_id = $1 AND cm.user_id = $2`, convID, userID,
	).Scan(&p.Name, &role)

	if errors.Is(err, pgx.ErrNoRows) {
		return SystemParty{}, "", fmt.Errorf("%w: bukan anggota grup ini", ErrForbidden)
	}
	if err != nil {
		return SystemParty{}, "", fmt.Errorf("ambil anggota: %w", err)
	}
	return p, role, nil
}

// appendNotice mencatat kejadian sebagai pesan sistem, memakai jalur alokasi
// `seq` yang sama persis dengan pesan biasa.
//
// Penanda baca PELAKU ikut dimajukan, sama seperti saat dia mengirim pesan:
// orang yang baru saja menekan tombolnya sendiri tidak perlu melihat badge
// "1 belum dibaca" untuk perbuatannya sendiri.
func appendNotice(ctx context.Context, tx pgx.Tx, convID uuid.UUID, lastSeq int64, ev SystemEvent) (Message, error) {
	seq := lastSeq + 1
	m := Message{
		ID: uuid.New(), ConversationID: convID, Seq: seq, SenderID: ev.Actor.ID,
		Attachments: []Attachment{}, Mentions: []uuid.UUID{},
		Reactions: []ReactionSummary{}, Kind: "system", SystemEvent: &ev,
	}

	if err := tx.QueryRow(ctx, `
		INSERT INTO messages (id, conversation_id, seq, sender_id, body, kind, system_event)
		VALUES ($1, $2, $3, $4, '', 'system', $5) RETURNING created_at`,
		m.ID, convID, seq, ev.Actor.ID, ev,
	).Scan(&m.CreatedAt); err != nil {
		return Message{}, fmt.Errorf("catat kejadian: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`UPDATE conversations SET last_seq = $1 WHERE id = $2`, seq, convID); err != nil {
		return Message{}, fmt.Errorf("bump last_seq: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE conversation_members SET last_read_seq = $1
		WHERE conversation_id = $2 AND user_id = $3 AND last_read_seq < $1`,
		seq, convID, ev.Actor.ID); err != nil {
		return Message{}, fmt.Errorf("self read: %w", err)
	}

	return m, nil
}

// membersIn membaca daftar anggota di dalam transaksi yang sedang berjalan.
// Dipisahkan dari Store.Members karena yang itu memakai pool, dan keadaan
// setelah perubahan belum terlihat dari sana sebelum commit.
func membersIn(ctx context.Context, tx pgx.Tx, convID uuid.UUID) ([]Member, error) {
	rows, err := tx.Query(ctx, `
		SELECT u.id, u.username, u.display_name, cm.role, cm.last_read_seq
		FROM conversation_members cm JOIN users u ON u.id = cm.user_id
		WHERE cm.conversation_id = $1
		ORDER BY u.display_name`, convID)
	if err != nil {
		return nil, fmt.Errorf("anggota: %w", err)
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

// ---------- tindakan ----------

// RenameGroup mengganti judul grup.
func (s *Store) RenameGroup(ctx context.Context, convID, actorID uuid.UUID, title string) (GroupChange, error) {
	return s.manage(ctx, convID, func(ctx context.Context, tx pgx.Tx, lastSeq int64, current string) (SystemEvent, error) {
		actor, err := requireOwner(ctx, tx, convID, actorID)
		if err != nil {
			return SystemEvent{}, err
		}
		if title == current {
			// Tidak ada yang berubah berarti tidak ada yang layak dicatat.
			// Menyimpannya akan menaruh baris "judul diganti jadi X" di tengah
			// percakapan padahal judulnya memang sudah X.
			return SystemEvent{}, errNoChange
		}
		if _, err := tx.Exec(ctx,
			`UPDATE conversations SET title = $1 WHERE id = $2`, title, convID); err != nil {
			return SystemEvent{}, fmt.Errorf("ganti judul: %w", err)
		}
		return SystemEvent{Type: SystemTitleChanged, Actor: actor, Title: title}, nil
	})
}

// AddMembers menambahkan orang ke grup.
//
// Yang sudah jadi anggota dilewati diam-diam, bukan ditolak: dua orang yang
// menambahkan orang yang sama bersamaan adalah kejadian biasa, dan kegagalan di
// sana tidak memberi tahu apa pun yang berguna. Yang benar-benar tidak ada
// sebagai pengguna DITOLAK — itu bukan balapan, itu id yang dikarang.
func (s *Store) AddMembers(ctx context.Context, convID, actorID uuid.UUID, userIDs []uuid.UUID) (GroupChange, error) {
	return s.manage(ctx, convID, func(ctx context.Context, tx pgx.Tx, lastSeq int64, _ string) (SystemEvent, error) {
		actor, err := requireOwner(ctx, tx, convID, actorID)
		if err != nil {
			return SystemEvent{}, err
		}

		wanted := withoutDuplicates(userIDs, uuid.Nil)
		if len(wanted) == 0 {
			return SystemEvent{}, errNoChange
		}

		var count int
		if err := tx.QueryRow(ctx,
			`SELECT count(*) FROM conversation_members WHERE conversation_id = $1`, convID,
		).Scan(&count); err != nil {
			return SystemEvent{}, fmt.Errorf("hitung anggota: %w", err)
		}

		// INSERT ... SELECT dari users: id yang tidak menunjuk pengguna mana pun
		// tidak menghasilkan baris, dan selisih jumlahnya yang menolaknya. Satu
		// query untuk "ada orangnya" dan "belum jadi anggota" sekaligus.
		rows, err := tx.Query(ctx, `
			INSERT INTO conversation_members (conversation_id, user_id, role)
			SELECT $1, u.id, 'member' FROM users u WHERE u.id = ANY($2)
			ON CONFLICT (conversation_id, user_id) DO NOTHING
			RETURNING user_id`, convID, wanted)
		if err != nil {
			return SystemEvent{}, fmt.Errorf("tambah anggota: %w", err)
		}

		added := []uuid.UUID{}
		for rows.Next() {
			var id uuid.UUID
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return SystemEvent{}, err
			}
			added = append(added, id)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return SystemEvent{}, err
		}

		if count+len(added) > maxGroupMembers {
			return SystemEvent{}, fmt.Errorf("%w: grup maksimal %d anggota", ErrConflict, maxGroupMembers)
		}
		if len(added) == 0 {
			// Semuanya sudah jadi anggota, atau tidak satu pun id-nya nyata.
			// Dibedakan di bawah lewat pemeriksaan keberadaan penggunanya.
			if err := ensureUsersExist(ctx, tx, wanted); err != nil {
				return SystemEvent{}, err
			}
			return SystemEvent{}, errNoChange
		}
		if err := ensureUsersExist(ctx, tx, wanted); err != nil {
			return SystemEvent{}, err
		}

		targets, err := partiesOf(ctx, tx, added)
		if err != nil {
			return SystemEvent{}, err
		}
		return SystemEvent{Type: SystemMemberAdded, Actor: actor, Targets: targets}, nil
	})
}

// RemoveMember mengeluarkan seorang anggota.
func (s *Store) RemoveMember(ctx context.Context, convID, actorID, targetID uuid.UUID) (GroupChange, error) {
	change, err := s.manage(ctx, convID, func(ctx context.Context, tx pgx.Tx, lastSeq int64, _ string) (SystemEvent, error) {
		actor, err := requireOwner(ctx, tx, convID, actorID)
		if err != nil {
			return SystemEvent{}, err
		}
		if targetID == actorID {
			// Pemilik yang ingin pergi memakai jalur keluar, yang tahu cara
			// memindahkan kepemilikan. Mengeluarkan diri sendiri lewat sini
			// akan meninggalkan grup tanpa pemilik.
			return SystemEvent{}, fmt.Errorf("%w: pakai jalur keluar untuk diri sendiri", ErrConflict)
		}

		target, role, err := partyOf(ctx, tx, convID, targetID)
		if err != nil {
			return SystemEvent{}, err
		}
		if role == "owner" {
			return SystemEvent{}, fmt.Errorf("%w: pemilik grup tidak bisa dikeluarkan", ErrForbidden)
		}

		if _, err := tx.Exec(ctx,
			`DELETE FROM conversation_members WHERE conversation_id = $1 AND user_id = $2`,
			convID, targetID); err != nil {
			return SystemEvent{}, fmt.Errorf("keluarkan anggota: %w", err)
		}

		return SystemEvent{Type: SystemMemberRemoved, Actor: actor, Targets: []SystemParty{target}}, nil
	})
	if err != nil {
		return GroupChange{}, err
	}
	change.Departed = &targetID
	return change, nil
}

// LeaveGroup mengeluarkan pemanggil dari grup atas kehendaknya sendiri.
//
// Pemilik yang keluar TIDAK ditahan. Kepemilikan berpindah sendiri ke anggota
// yang paling lama bergabung.
//
// Alternatifnya adalah menuntut pemilik memindahkan kepemilikan lebih dulu, dan
// itu menukar satu langkah tambahan dengan sebuah keadaan yang jauh lebih
// buruk: grup yang pemiliknya berhenti memakai aplikasi ini adalah grup yang
// tidak bisa dikelola siapa pun, selamanya, tanpa cara memperbaikinya dari
// dalam.
func (s *Store) LeaveGroup(ctx context.Context, convID, actorID uuid.UUID) (GroupChange, error) {
	change, err := s.manage(ctx, convID, func(ctx context.Context, tx pgx.Tx, lastSeq int64, _ string) (SystemEvent, error) {
		actor, role, err := partyOf(ctx, tx, convID, actorID)
		if err != nil {
			return SystemEvent{}, err
		}

		if _, err := tx.Exec(ctx,
			`DELETE FROM conversation_members WHERE conversation_id = $1 AND user_id = $2`,
			convID, actorID); err != nil {
			return SystemEvent{}, fmt.Errorf("keluar grup: %w", err)
		}

		if role == "owner" {
			// Anggota terlama, bukan yang pertama menurut abjad: yang paling
			// lama di dalam grup adalah yang paling mungkin mengenali isinya.
			// Kalau tidak ada siapa-siapa lagi, grupnya memang habis — barisnya
			// tetap ada supaya riwayatnya tidak ikut terhapus, dan tidak ada
			// seorang pun yang masih bisa membukanya.
			if _, err := tx.Exec(ctx, `
				UPDATE conversation_members SET role = 'owner'
				WHERE conversation_id = $1 AND user_id = (
					SELECT user_id FROM conversation_members
					WHERE conversation_id = $1
					ORDER BY joined_at, user_id LIMIT 1
				)`, convID); err != nil {
				return SystemEvent{}, fmt.Errorf("pindahkan kepemilikan: %w", err)
			}
		}

		return SystemEvent{Type: SystemMemberLeft, Actor: actor}, nil
	})
	if err != nil {
		return GroupChange{}, err
	}
	change.Departed = &actorID
	return change, nil
}

// TransferOwnership memindahkan kepemilikan grup ke anggota lain.
func (s *Store) TransferOwnership(ctx context.Context, convID, actorID, targetID uuid.UUID) (GroupChange, error) {
	return s.manage(ctx, convID, func(ctx context.Context, tx pgx.Tx, lastSeq int64, _ string) (SystemEvent, error) {
		actor, err := requireOwner(ctx, tx, convID, actorID)
		if err != nil {
			return SystemEvent{}, err
		}
		if targetID == actorID {
			return SystemEvent{}, errNoChange
		}

		target, _, err := partyOf(ctx, tx, convID, targetID)
		if err != nil {
			return SystemEvent{}, err
		}

		// Dua UPDATE, bukan satu dengan CASE: yang lama turun lebih dulu supaya
		// tidak pernah ada sesaat pun dengan dua pemilik. Keduanya di dalam satu
		// transaksi, jadi tidak ada yang bisa melihat keadaan di antaranya.
		if _, err := tx.Exec(ctx, `
			UPDATE conversation_members SET role = 'member'
			WHERE conversation_id = $1 AND user_id = $2`, convID, actorID); err != nil {
			return SystemEvent{}, fmt.Errorf("turunkan pemilik lama: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE conversation_members SET role = 'owner'
			WHERE conversation_id = $1 AND user_id = $2`, convID, targetID); err != nil {
			return SystemEvent{}, fmt.Errorf("angkat pemilik baru: %w", err)
		}

		return SystemEvent{Type: SystemOwnerChanged, Actor: actor, Targets: []SystemParty{target}}, nil
	})
}

// ---------- kerangka bersama ----------

// errNoChange menandai tindakan yang ternyata tidak mengubah apa pun. Bukan
// kesalahan, dan tidak pernah sampai ke pemanggil: yang terjadi cuma tidak ada
// catatan yang dibuat dan tidak ada siaran yang dikirim.
var errNoChange = errors.New("tidak ada yang berubah")

// manage menjalankan satu tindakan pengelolaan di dalam transaksi, dengan baris
// percakapan terkunci, lalu mencatat kejadiannya dan membaca keadaan akhirnya.
//
// Seluruh bagian yang sama untuk kelima tindakan ada di sini — kunci,
// pemeriksaan tipe, alokasi seq, pembacaan keadaan akhir, commit — sehingga
// tiap tindakan tinggal berisi bagian yang memang khas miliknya.
func (s *Store) manage(
	ctx context.Context,
	convID uuid.UUID,
	act func(context.Context, pgx.Tx, int64, string) (SystemEvent, error),
) (GroupChange, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return GroupChange{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	lastSeq, title, err := lockGroup(ctx, tx, convID)
	if err != nil {
		return GroupChange{}, err
	}

	ev, err := act(ctx, tx, lastSeq, title)
	if errors.Is(err, errNoChange) {
		members, err := membersIn(ctx, tx, convID)
		if err != nil {
			return GroupChange{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return GroupChange{}, err
		}
		// Notice kosong (ID nil) adalah tanda bagi pemanggil bahwa tidak ada
		// yang perlu disiarkan.
		return GroupChange{Title: title, Members: members}, nil
	}
	if err != nil {
		return GroupChange{}, err
	}

	notice, err := appendNotice(ctx, tx, convID, lastSeq, ev)
	if err != nil {
		return GroupChange{}, err
	}

	// Judul dibaca ULANG, bukan diambil dari ev: hanya satu dari lima tindakan
	// yang mengubahnya, dan mengembalikan judul lama untuk empat sisanya berarti
	// empat jalur yang harus ingat mengisinya sendiri.
	var newTitle *string
	if err := tx.QueryRow(ctx,
		`SELECT title FROM conversations WHERE id = $1`, convID).Scan(&newTitle); err != nil {
		return GroupChange{}, fmt.Errorf("baca judul: %w", err)
	}

	members, err := membersIn(ctx, tx, convID)
	if err != nil {
		return GroupChange{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return GroupChange{}, err
	}

	change := GroupChange{Notice: notice, Members: members}
	if newTitle != nil {
		change.Title = *newTitle
	}
	return change, nil
}

// ensureUsersExist menolak id yang tidak menunjuk pengguna mana pun.
//
// Dipisahkan dari penyisipannya karena ON CONFLICT DO NOTHING membuat "sudah
// jadi anggota" dan "orangnya tidak ada" menghasilkan jumlah baris yang sama
// persis — nol — dan keduanya menuntut jawaban yang berbeda.
func ensureUsersExist(ctx context.Context, tx pgx.Tx, ids []uuid.UUID) error {
	var found int
	if err := tx.QueryRow(ctx,
		`SELECT count(*) FROM users WHERE id = ANY($1)`, ids).Scan(&found); err != nil {
		return fmt.Errorf("periksa pengguna: %w", err)
	}
	if found != len(ids) {
		return fmt.Errorf("%w: ada id yang bukan pengguna", ErrNotFound)
	}
	return nil
}

// partiesOf mengambil nama sekumpulan orang untuk dicatat di dalam kejadian.
func partiesOf(ctx context.Context, tx pgx.Tx, ids []uuid.UUID) ([]SystemParty, error) {
	rows, err := tx.Query(ctx,
		`SELECT id, display_name FROM users WHERE id = ANY($1) ORDER BY display_name`, ids)
	if err != nil {
		return nil, fmt.Errorf("ambil nama: %w", err)
	}
	defer rows.Close()

	out := []SystemParty{}
	for rows.Next() {
		var p SystemParty
		if err := rows.Scan(&p.ID, &p.Name); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// NormalizeGroupTitle merapikan judul yang diketik orang dan menolak yang tidak
// masuk akal. Dipakai saat membuat grup maupun saat menggantinya, supaya
// keduanya tidak pernah punya aturan yang berbeda.
func NormalizeGroupTitle(raw string) (string, error) {
	title := strings.TrimSpace(raw)
	if title == "" {
		return "", fmt.Errorf("%w: judul grup wajib diisi", ErrInvalid)
	}
	if len([]rune(title)) > 100 {
		return "", fmt.Errorf("%w: judul grup maksimal 100 karakter", ErrInvalid)
	}
	return title, nil
}
