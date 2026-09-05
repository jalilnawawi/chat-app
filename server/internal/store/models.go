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
	ID             uuid.UUID  `json:"id"`
	ConversationID uuid.UUID  `json:"conversationId"`
	Seq            int64      `json:"seq"`
	SenderID       uuid.UUID  `json:"senderId"`
	Body           string     `json:"body"`
	CreatedAt      time.Time  `json:"createdAt"`
	EditedAt       *time.Time `json:"editedAt"`
	DeletedAt      *time.Time `json:"deletedAt"`
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
