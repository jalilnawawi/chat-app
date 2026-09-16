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
}
