package store

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID          uuid.UUID `json:"id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"displayName"`
	CreatedAt   time.Time `json:"createdAt"`
}

type Message struct {
	ID             uuid.UUID    `json:"id"`
	ConversationID uuid.UUID    `json:"conversationId"`
	Seq            int64        `json:"seq"`
	SenderID       uuid.UUID    `json:"senderId"`
	Body           string       `json:"body"`
	Attachments    []Attachment `json:"attachments"`
	CreatedAt      time.Time    `json:"createdAt"`
	EditedAt       *time.Time   `json:"editedAt"`
	DeletedAt      *time.Time   `json:"deletedAt"`

	// ReplyTo nil berarti pesan ini tidak membalas apa pun. Isinya dibaca ulang
	// dari barisnya sendiri setiap kali riwayat dimuat — lihat ReplyPreview.
	ReplyTo *ReplyPreview `json:"replyTo,omitempty"`

	// Mentions adalah id yang disebut, sudah lolos pemeriksaan keanggotaan di
	// server. Client memakainya untuk menyorot namanya sendiri; dia TIDAK
	// dipakai untuk menentukan siapa yang dibangunkan — itu sudah diputuskan
	// saat pesannya masuk.
	Mentions    []uuid.UUID `json:"mentions"`
	MentionsAll bool        `json:"mentionsAll"`

	// Reactions adalah ringkasan per emoji, dan isinya BERGANTUNG PADA SIAPA
	// YANG MEMBACA (lihat ReactionSummary.Mine). Karena itu dia hanya pernah
	// diisi pada jalur yang tahu pembacanya — riwayat REST dan susulan resume —
	// dan tidak pernah pada siaran yang satu payload untuk semua orang.
	Reactions []ReactionSummary `json:"reactions"`

	// ReactionSeq adalah nilai jam reaksi percakapan saat terakhir kali reaksi
	// pesan ini berubah. Nol berarti belum pernah ada yang bereaksi. Client
	// menyimpan nilai terbesar yang pernah dilihatnya sebagai cursor resume
	// kedua, di samping `seq`.
	ReactionSeq int64 `json:"reactionSeq"`
}

// ReplyPreview adalah secuil pesan yang dibalas, secukupnya untuk gelembung
// kutipan — dan sengaja TIDAK disalin ke baris pembalasnya.
//
// Ini berbeda dari keputusan lampiran di Fase 7, dan perbedaannya disengaja:
// salinan lampiran boleh ada karena lampiran tidak pernah berubah setelah
// terpasang. Isi pesan berubah — diedit dan dihapus — jadi salinannya pasti
// basi. Yang dibaca di sini selalu keadaan terbaru, lewat satu self-join per
// halaman riwayat pada primary key.
type ReplyPreview struct {
	ID       uuid.UUID `json:"id"`
	Seq      int64     `json:"seq"`
	SenderID uuid.UUID `json:"senderId"`
	Body     string    `json:"body"`

	// Deleted true berarti pesan yang dikutip sudah dihapus. Kutipannya tetap
	// tampil sebagai "pesan dihapus", bukan menghilang: gelembung balasan yang
	// tiba-tiba kehilangan konteksnya lebih membingungkan daripada kutipan yang
	// jujur mengatakan isinya sudah tidak ada.
	Deleted bool `json:"deleted"`

	// Kind memberi client satu kata untuk pesan yang isinya hanya lampiran —
	// kutipan kosong terbaca sebagai pesan kosong. "image", "video", "audio",
	// "file", atau kosong untuk pesan teks biasa.
	Kind string `json:"kind,omitempty"`
}

// ReactionSummary adalah satu emoji pada satu pesan, sudah dihitung.
//
// Yang dikirim adalah JUMLAH, bukan daftar orangnya. Sebuah grup dua ratus
// orang yang semuanya menekan emoji yang sama akan menghasilkan dua ratus id
// yang tidak satu pun ditampilkan — dan riwayat yang memuatnya di setiap
// pesan membayar itu untuk seluruh halaman.
type ReactionSummary struct {
	Emoji string `json:"emoji"`
	Count int    `json:"count"`
	// Mine: apakah PEMBACA ikut memberi emoji ini. Inilah yang membuat
	// ringkasan ini tidak pernah boleh ikut di jalur siaran bersama.
	Mine bool `json:"mine"`
}

// MessageReactions adalah keadaan reaksi satu pesan pada satu titik waktu —
// bentuk yang dikirim saat menyusul setelah reconnect.
type MessageReactions struct {
	MessageID   uuid.UUID         `json:"messageId"`
	ReactionSeq int64             `json:"reactionSeq"`
	Reactions   []ReactionSummary `json:"reactions"`
}

// Attachment adalah bentuk lampiran yang dilihat client — dan sekaligus bentuk
// yang disalin ke kolom messages.attachments.
//
// URL-nya menunjuk ke server ini, bukan ke penyimpanan objek. Itu keputusan
// utama soal lampiran: SeaweedFS tidak tahu apa-apa tentang keanggotaan
// percakapan, jadi satu-satunya tempat yang bisa menjawab "boleh tidak orang
// ini membaca file ini" adalah server yang menyimpan keanggotaannya.
type Attachment struct {
	ID   uuid.UUID `json:"id"`
	URL  string    `json:"url"`
	Name string    `json:"name"`
	MIME string    `json:"mime"`
	Size int64     `json:"size"`

	// Ukuran gambar sebagaimana AKAN TERLIHAT — hasil membaca header berkasnya
	// di server, bukan angka yang diakui client. Dipakai memesan ruang di layar
	// sebelum gambarnya termuat.
	Width  *int `json:"width,omitempty"`
	Height *int `json:"height,omitempty"`

	// ThumbURL kosong berarti lampiran ini tidak punya turunan kecil, dan
	// client memakai URL aslinya. Tiga hal berakhir di keadaan itu: bukan
	// gambar, sudah cukup kecil untuk dipakai apa adanya, dan pembuatan
	// turunannya gagal. Client tidak perlu membedakan ketiganya — jawabannya
	// sama.
	ThumbURL string `json:"thumbUrl,omitempty"`
}

// AttachmentURL menyusun alamat unduh sebuah lampiran. Client tidak pernah
// merangkainya sendiri, supaya bentuk alamatnya bisa berubah tanpa memaksa
// semua pesan lama ditulis ulang.
func AttachmentURL(id uuid.UUID) string { return "/api/attachments/" + id.String() }

// AttachmentThumbURL menyusun alamat turunan kecil. Jalur terpisah, bukan
// parameter query pada alamat aslinya: turunan dan aslinya adalah dua byte yang
// berbeda dan keduanya disimpan selamanya, jadi keduanya layak punya alamat
// sendiri yang bisa di-cache sendiri.
func AttachmentThumbURL(id uuid.UUID) string {
	return "/api/attachments/" + id.String() + "/thumb"
}

// StoredAttachment adalah lampiran beserta bagian yang tidak pernah dikirim ke
// client: di mana byte-nya sebenarnya tersimpan.
//
// Dipisahkan dari Attachment supaya alamat internal penyimpanan tidak bisa
// ikut terbawa ke JSON hanya karena suatu hari ada yang menambahkan field.
type StoredAttachment struct {
	Attachment

	Key string

	// Kosong bila tidak ada turunan. Ketiganya lahir dan mati bersama —
	// dijaga juga oleh CHECK di sisi database.
	ThumbKey  string
	ThumbMIME string
	ThumbSize int64
}

type Member struct {
	UserID      uuid.UUID `json:"userId"`
	Username    string    `json:"username"`
	DisplayName string    `json:"displayName"`
	Role        string    `json:"role"`
	LastReadSeq int64     `json:"lastReadSeq"`
}

// Conversation adalah bentuk yang dikirim ke client untuk daftar percakapan.
// Untuk tipe "direct", Title dikosongkan dan Peer diisi lawan bicara — client
// menampilkan nama peer sebagai judul.
type Conversation struct {
	ID          uuid.UUID `json:"id"`
	Type        string    `json:"type"`
	Title       *string   `json:"title"`
	LastSeq     int64     `json:"lastSeq"`
	LastReadSeq int64     `json:"lastReadSeq"`
	Unread      int64     `json:"unread"`
	Peer        *User     `json:"peer"`
	LastMessage *Message  `json:"lastMessage"`
	UpdatedAt   time.Time `json:"updatedAt"`

	// MentionSeq > MentionAckSeq berarti ada yang menyebut nama pembaca dan dia
	// belum sampai ke pesannya. Sengaja dua angka, bukan satu boolean: sebutan
	// yang datang SAAT percakapannya sedang terbuka harus tetap menyalakan
	// penanda sampai pesannya benar-benar terlihat.
	MentionSeq    int64 `json:"mentionSeq"`
	MentionAckSeq int64 `json:"mentionAckSeq"`
}
