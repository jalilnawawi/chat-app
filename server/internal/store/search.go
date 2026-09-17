package store

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Pencarian isi pesan.
//
// Satu aturan di atas semua pertimbangan lain: **pencarian tidak pernah boleh
// menemukan apa yang tidak akan ditampilkan riwayat.** Karena itu izinnya
// dievaluasi DI DALAM kueri — join ke keanggotaan pembaca — bukan disaring
// sesudahnya, dan pesan yang dihapus maupun catatan sistem tersaring oleh
// syarat yang sama dengan index-nya.
//
// Keputusan tsvector-vs-trigram dan 'simple'-vs-'indonesian' ditulis di
// 0008_menemukan_pesan.sql, bersama angka yang mendasarinya.

const (
	// maxSearchTerms membatasi berapa kata yang dicocokkan. Tiap kata adalah
	// satu penelusuran index lagi, dan kalimat panjang yang ditempel ke kotak
	// pencarian tidak sedang mencari apa-apa.
	maxSearchTerms = 8

	// minPrefixLen: kata yang lebih pendek dari ini dicocokkan UTUH, bukan
	// sebagai awalan. Awalan satu huruf menyuruh index menggabungkan daftar
	// milik setiap kata yang berawalan huruf itu — pada tabel besar itu
	// sebagian besar isi index. Angkanya ada di docs/menemukan-pesan.md.
	minPrefixLen = 3
)

// SearchQuery adalah permintaan pencarian yang sudah divalidasi bentuknya.
type SearchQuery struct {
	ViewerID uuid.UUID
	Text     string
	// ConversationID nil berarti mencari di SEMUA percakapan milik pembaca.
	ConversationID *uuid.UUID
	// Cursor kosong berarti mulai dari yang terbaru.
	Cursor string
	Limit  int
}

// SearchResult adalah satu halaman hasil.
type SearchResult struct {
	Hits []SearchHit `json:"hits"`

	// Terms adalah kata-kata yang benar-benar dicocokkan, SETELAH diurai
	// Postgres. Client menyorot kata ini, bukan hasil penguraiannya sendiri —
	// dua pengurai yang berbeda pendapat tentang "13.00" atau "e-mail" adalah
	// sorotan yang tidak menunjuk apa pun.
	Terms []string `json:"terms"`

	// NextCursor kosong berarti tidak ada halaman berikutnya.
	NextCursor string `json:"nextCursor,omitempty"`
}

// SearchMessages mencari pesan yang memuat SEMUA kata di kueri.
//
// Hasilnya diurutkan dari yang terbaru, bukan menurut kemiripan. Di chat yang
// dicari hampir selalu "yang kemarin itu"; peringkat kemiripan memindahkan
// pesan tiga tahun lalu yang kebetulan mengulang kata yang sama ke atas pesan
// minggu lalu yang sedang dicari.
func (s *Store) SearchMessages(ctx context.Context, q SearchQuery) (SearchResult, error) {
	terms, err := s.searchTerms(ctx, q.Text)
	if err != nil {
		return SearchResult{}, err
	}

	// Semua penyaring KECUALI keanggotaan ada di dalam CTE. Keanggotaan
	// disaring sesudahnya, lewat hash join, dan bentuk ini bukan selera.
	//
	// Ditulis sebagai satu kueri biasa, planner memilih nested loop yang
	// memindai index pencarian SEKALI PER PERCAKAPAN milik pembaca — enam puluh
	// tiga kali untuk satu kata, 212 ms pada sejuta pesan. MATERIALIZED
	// memaksanya memindai sekali saja: 57 ms untuk kata yang sama, dan tetap
	// begitu setelah Postgres beralih ke rencana umum untuk prepared statement
	// yang dipakai pgx. Angka lengkapnya di docs/menemukan-pesan.md.
	//
	// Kolom pesan yang berat dibaca paling akhir, hanya untuk satu halaman.
	where := []string{
		"m.kind = 'user'", "m.deleted_at IS NULL",
		"to_tsvector('simple', m.body) @@ $2::tsquery",
	}
	args := []any{q.ViewerID, tsquery(terms)}

	if q.ConversationID != nil {
		args = append(args, *q.ConversationID)
		where = append(where, fmt.Sprintf("m.conversation_id = $%d", len(args)))
	}
	if q.Cursor != "" {
		at, id, err := decodeCursor(q.Cursor)
		if err != nil {
			return SearchResult{}, err
		}
		args = append(args, at, id)
		where = append(where, fmt.Sprintf("(m.created_at, m.id) < ($%d, $%d)", len(args)-1, len(args)))
	}
	args = append(args, q.Limit+1)

	rows, err := s.pool.Query(ctx, `
		WITH hit AS MATERIALIZED (
			SELECT m.id, m.conversation_id, m.created_at
			FROM messages m
			WHERE `+strings.Join(where, " AND ")+`
		), page AS (
			SELECT h.id, h.created_at
			FROM hit h
			JOIN conversation_members cm
			  ON cm.conversation_id = h.conversation_id AND cm.user_id = $1
			ORDER BY h.created_at DESC, h.id DESC
			LIMIT $`+strconv.Itoa(len(args))+`
		)
		SELECT `+messageCols("m")+`, `+replyCols("rep")+`, u.display_name, u.avatar_id
		FROM page p
		JOIN messages m ON m.id = p.id
		JOIN users u ON u.id = m.sender_id
		LEFT JOIN messages rep ON rep.id = m.reply_to_id
		ORDER BY p.created_at DESC, p.id DESC`, args...)
	if err != nil {
		return SearchResult{}, fmt.Errorf("cari pesan: %w", err)
	}
	defer rows.Close()

	res := SearchResult{Hits: []SearchHit{}, Terms: terms}
	for rows.Next() {
		var (
			h        SearchHit
			avatarID *uuid.UUID
		)
		dest, finish := messageDest(&h.Message)
		if err := rows.Scan(append(dest, &h.SenderName, &avatarID)...); err != nil {
			return SearchResult{}, err
		}
		finish()
		if avatarID != nil {
			h.SenderAvatarURL = AvatarURL(*avatarID)
		}
		res.Hits = append(res.Hits, h)
	}
	if err := rows.Err(); err != nil {
		return SearchResult{}, err
	}

	// Satu baris lebih dari yang diminta sudah menjawab "masih ada lagi?"
	// tanpa COUNT(*) — yang pada pencarian kata umum berarti menghitung seluruh
	// kecocokan hanya untuk membuang angkanya.
	if len(res.Hits) > q.Limit {
		res.Hits = res.Hits[:q.Limit]
		last := res.Hits[len(res.Hits)-1].Message
		res.NextCursor = encodeCursor(last.CreatedAt, last.ID)
	}

	messages := make([]Message, len(res.Hits))
	for i := range res.Hits {
		messages[i] = res.Hits[i].Message
	}
	if err := s.attachReactions(ctx, messages, q.ViewerID); err != nil {
		return SearchResult{}, err
	}
	for i := range res.Hits {
		res.Hits[i].Message = messages[i]
	}
	return res, nil
}

// searchTerms mengurai kueri dengan pengurai yang SAMA dengan yang mengisi
// index.
//
// Menguraikannya sendiri di Go terlihat lebih sederhana dan hampir benar — dan
// "hampir" di sini berarti hasil yang hilang tanpa penjelasan. Pengurai
// Postgres menyimpan "13.00" sebagai satu kata, "e-mail" sebagai tiga
// ("e-mail", "e", "mail"), dan alamat email utuh; pengurai buatan sendiri yang
// memecah di setiap tanda baca mencari "13" dan "00", yang tidak pernah ada di
// index.
func (s *Store) searchTerms(ctx context.Context, text string) ([]string, error) {
	var terms []string
	if err := s.pool.QueryRow(ctx, `
		SELECT coalesce(array_agg(lexeme ORDER BY min_pos), '{}')
		FROM (
			SELECT lexeme, positions[1] AS min_pos
			FROM unnest(to_tsvector('simple', $1))
		) t`, text).Scan(&terms); err != nil {
		return nil, fmt.Errorf("urai kueri pencarian: %w", err)
	}
	if len(terms) == 0 {
		return nil, fmt.Errorf("%w: ketik setidaknya satu kata", ErrInvalid)
	}
	if len(terms) > maxSearchTerms {
		terms = terms[:maxSearchTerms]
	}
	return terms, nil
}

// tsquery menyusun kata-kata yang sudah diurai jadi teks tsquery.
//
// Teksnya di-cast langsung ke tsquery, BUKAN dilewatkan ke to_tsquery. Yang
// kedua mengurai ulang setiap kata, dan kata yang sudah diurai tidak selalu
// selamat diurai dua kali: "e-mail" pecah lagi jadi "e", dan awalan "e:*"
// adalah persis penelusuran satu huruf yang dicegah minPrefixLen. Cast tidak
// menyentuh isi kata sama sekali — dan kata-kata ini memang sudah dalam bentuk
// yang tersimpan di index.
//
// Tiap kata DIKUTIP, bukan ditempel apa adanya: kata dari pengguna bisa
// memuat apa saja yang punya arti di sintaks tsquery — titik dua, tanda seru,
// ampersand, kutip. Tanpa kutipan, mencari "O'Brien" atau "rapat:" berakhir
// sebagai kesalahan sintaks di server, bukan sebagai pencarian.
func tsquery(terms []string) string {
	parts := make([]string, len(terms))
	for i, t := range terms {
		quoted := "'" + strings.NewReplacer(`\`, `\\`, `'`, `''`).Replace(t) + "'"
		if utf8.RuneCountInString(t) >= minPrefixLen {
			quoted += ":*"
		}
		parts[i] = quoted
	}
	return strings.Join(parts, " & ")
}

// Cursor halaman berikutnya: waktu dan id pesan terakhir.
//
// Dibuat buram (base64) bukan untuk menyembunyikan apa pun — isinya cuma waktu
// dan id pesan yang sudah dimiliki client — melainkan supaya client tidak
// tergoda menyusunnya sendiri, dan bentuknya bisa berubah tanpa merusak siapa
// pun.
func encodeCursor(at time.Time, id uuid.UUID) string {
	raw := strconv.FormatInt(at.UnixMicro(), 10) + ":" + id.String()
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeCursor(c string) (time.Time, uuid.UUID, error) {
	bad := fmt.Errorf("%w: cursor pencarian tidak dikenali", ErrInvalid)

	raw, err := base64.RawURLEncoding.DecodeString(c)
	if err != nil {
		return time.Time{}, uuid.Nil, bad
	}
	micros, idText, ok := strings.Cut(string(raw), ":")
	if !ok {
		return time.Time{}, uuid.Nil, bad
	}
	n, err := strconv.ParseInt(micros, 10, 64)
	if err != nil {
		return time.Time{}, uuid.Nil, bad
	}
	id, err := uuid.Parse(idText)
	if err != nil {
		return time.Time{}, uuid.Nil, bad
	}
	return time.UnixMicro(n), id, nil
}
