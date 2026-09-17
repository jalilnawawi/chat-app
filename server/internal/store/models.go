package store

import (
	"time"

	"github.com/google/uuid"
)

// User adalah seorang pengguna SEBAGAIMANA DILIHAT ORANG LAIN.
//
// Apa yang TIDAK ada di sini sama pentingnya dengan apa yang ada: email dan
// keadaan verifikasinya tinggal di Me, dan pemisahan itu bukan kerapian. Bentuk
// ini ikut di dalam hasil pencarian pengguna, di dalam `peer` pada daftar
// percakapan, dan di dalam setiap siaran yang menyebut seseorang — tiga jalur
// yang tidak satu pun punya alasan membawa alamat email siapa pun, dan tiga
// jalur yang akan diam-diam membawanya begitu suatu hari ada yang menambahkan
// satu field ke struct yang salah.
type User struct {
	ID          uuid.UUID `json:"id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"displayName"`
	CreatedAt   time.Time `json:"createdAt"`

	// AvatarURL kosong berarti orang ini belum memasang foto; client
	// menampilkan huruf pertama namanya, seperti sebelum Fase 10.
	//
	// Alamatnya memuat id unggahan, BUKAN id penggunanya — tiap penggantian foto
	// menghasilkan alamat baru. Lihat AvatarURL.
	AvatarURL string `json:"avatarUrl,omitempty"`

	// Status adalah pernyataan yang dibuat orang dengan sengaja, dan dia BUKAN
	// presence. Presence diturunkan dari koneksi yang hidup dan sengaja fana;
	// ini bertahan melewati tutup laptop, ganti perangkat, dan logout.
	//
	// Yang sudah lewat `status_expires_at` tidak pernah sampai ke sini: seluruh
	// pembacaan menyaringnya di dalam SQL — lihat userCols.
	Status          string     `json:"status"`
	StatusText      string     `json:"statusText,omitempty"`
	StatusExpiresAt *time.Time `json:"statusExpiresAt,omitempty"`
}

// Me adalah pengguna sebagaimana dilihat DIRINYA SENDIRI.
//
// Satu-satunya bentuk yang membawa email, dan satu-satunya yang pernah dikirim
// ke pemiliknya saja. Lihat catatan pemisahannya di User.
type Me struct {
	User

	Email string `json:"email,omitempty"`

	// EmailVerified false untuk alamat yang sudah diketik tapi belum dibuktikan
	// kepemilikannya — dan alamat seperti itu tidak bisa dipakai memulihkan akun
	// sama sekali. Kalau bisa, memulihkan akun orang lain cuma butuh mengaku
	// memiliki sebuah alamat.
	EmailVerified bool `json:"emailVerified"`
}

// Status yang boleh dipasang seseorang. 'available' adalah keadaan bawaan, dan
// dia yang dikembalikan untuk status apa pun yang sudah lewat waktunya.
const (
	StatusAvailable = "available"
	StatusBusy      = "busy"
	StatusAway      = "away"
)

// AvatarURL menyusun alamat baca sebuah foto profil dari ID UNGGAHANNYA.
//
// Bukan dari id penggunanya, dan itu seluruh alasan kolom `avatar_id` ada.
// Alamat tetap seperti /api/users/{id}/avatar berarti browser menyimpan foto
// lama selama setahun dan tidak pernah lagi bertanya — orang mengganti fotonya,
// dan tidak seorang pun melihatnya. Dengan id yang berganti tiap unggahan,
// cache setahun kembali menjadi benar, bukan menjadi jebakan.
func AvatarURL(avatarID uuid.UUID) string { return "/api/avatars/" + avatarID.String() }

// StoredAvatar adalah foto profil beserta bagian yang tidak pernah dikirim ke
// client: di mana byte-nya sebenarnya tersimpan. Dipisahkan dengan alasan yang
// sama dengan StoredAttachment.
type StoredAvatar struct {
	ID   uuid.UUID
	Key  string
	MIME string
	Size int64
}

// Session adalah satu perangkat yang sedang login.
//
// Token-nya sendiri tidak pernah ikut — yang tersimpan di database cuma
// hash-nya sejak Fase 1, dan bahkan hash itu tidak dipakai sebagai nama publik.
// Lihat catatan kolom `id` di 0007_akun_profil.sql.
type Session struct {
	ID         uuid.UUID  `json:"id"`
	UserAgent  string     `json:"userAgent"`
	CreatedAt  time.Time  `json:"createdAt"`
	LastSeenAt *time.Time `json:"lastSeenAt"`
	ExpiresAt  time.Time  `json:"expiresAt"`

	// Current menandai sesi yang sedang dipakai untuk bertanya. Client
	// memakainya untuk tidak menawarkan tombol "cabut" yang akan mengeluarkan
	// orangnya dari layar yang sedang dia buka.
	Current bool `json:"current"`
}

// UserStatus adalah status seseorang tanpa sisa identitasnya — bentuk yang
// disiarkan dan yang dikirim sebagai snapshot saat client menyambung.
type UserStatus struct {
	UserID    uuid.UUID  `json:"userId"`
	Status    string     `json:"status"`
	Text      string     `json:"text,omitempty"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
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

	// Kind memisahkan apa yang ditulis orang ("user") dari apa yang dicatat
	// sistem ("system") — bergabung, keluar, dikeluarkan, judul berganti.
	//
	// Pesan sistem menumpang tabel yang sama supaya dia ikut terbawa oleh
	// seluruh mesin yang sudah ada: riwayat, cursor, dan susulan setelah
	// reconnect. Yang membedakannya cuma satu kolom, dan kolom itulah yang
	// menutup jalur edit, hapus, balas, dan reaksi untuknya.
	Kind string `json:"kind"`

	// SystemEvent nil untuk pesan biasa.
	SystemEvent *SystemEvent `json:"systemEvent,omitempty"`

	// Forwarded menandai pesan yang isinya disalin dari pesan lain.
	//
	// Cuma penanda, tanpa asal-usul: siapa penulis aslinya dan di percakapan
	// mana dia menulisnya bukan sesuatu yang dibagikan oleh penulis itu. Lihat
	// catatannya di 0008_menemukan_pesan.sql.
	Forwarded bool `json:"forwarded"`
}

// SystemParty adalah orang yang disebut sebuah catatan sistem, beserta namanya
// PADA SAAT ITU.
//
// Namanya ikut disalin, dan itu disengaja: orang yang dikeluarkan tidak lagi
// ada di daftar anggota, jadi client tidak punya tempat untuk mencarinya.
// Sejalan dengan aturan salinan yang sama sejak Fase 7 — yang boleh disalin
// adalah yang tidak pernah berubah, dan catatan sejarah memang dibekukan pada
// saat kejadiannya.
type SystemParty struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// Jenis catatan sistem. Yang disimpan adalah KEJADIANNYA, bukan kalimatnya —
// kalimatnya disusun client, sehingga bahasanya bisa berubah tanpa menulis
// ulang riwayat siapa pun.
const (
	SystemMemberAdded   = "member.added"
	SystemMemberRemoved = "member.removed"
	SystemMemberLeft    = "member.left"
	SystemTitleChanged  = "title.changed"
	SystemOwnerChanged  = "owner.changed"

	// Sematan dicatat sebagai kejadian seperti pengelolaan grup, dan catatan
	// itulah yang memberi tahu client yang sedang menyusul bahwa daftar
	// sematannya perlu dibaca ulang. Lihat store/pins.go.
	SystemMessagePinned   = "message.pinned"
	SystemMessageUnpinned = "message.unpinned"
)

type SystemEvent struct {
	Type    string        `json:"type"`
	Actor   SystemParty   `json:"actor"`
	Targets []SystemParty `json:"targets,omitempty"`
	// Title diisi untuk title.changed: judul BARU-nya.
	Title string `json:"title,omitempty"`

	// MessageID dan MessageSeq diisi untuk sematan: pesan mana yang disematkan
	// atau dilepas.
	//
	// Yang disalin cuma penunjuknya, TIDAK ada cuplikan isinya. Isi pesan bisa
	// diedit dan dihapus; cuplikan yang dibekukan di dalam catatan ini akan
	// tetap menampilkan kalimat yang sudah dihapus penulisnya, selamanya, di
	// tengah riwayat semua orang. `seq` boleh disalin karena dia memang tidak
	// pernah berubah — dan dia yang membuat catatan ini bisa diklik untuk
	// melompat ke pesannya walau pesan itu jauh di belakang riwayat.
	MessageID  *uuid.UUID `json:"messageId,omitempty"`
	MessageSeq int64      `json:"messageSeq,omitempty"`
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

// Pin adalah satu pesan yang disematkan, beserta siapa yang menyematkannya.
//
// Isi pesannya dibaca lewat join setiap kali, bukan disalin — aturan yang sama
// dengan kutipan balasan: yang diedit tampil versi barunya.
type Pin struct {
	Message Message `json:"message"`
	// PinnedBy nil bila akun yang menyematkannya sudah tidak ada.
	PinnedBy     *uuid.UUID `json:"pinnedBy"`
	PinnedByName string     `json:"pinnedByName"`
	PinnedAt     time.Time  `json:"pinnedAt"`
}

// SearchHit adalah satu pesan yang cocok dengan pencarian.
//
// Nama dan foto pengirim ikut, berbeda dari riwayat biasa: hasil pencarian
// lintas percakapan datang dari ruang-ruang yang daftar anggotanya belum tentu
// pernah dimuat client, dan hasil yang semuanya bertuliskan "Seseorang" tidak
// bisa dipakai memilih.
type SearchHit struct {
	Message         Message `json:"message"`
	SenderName      string  `json:"senderName"`
	SenderAvatarURL string  `json:"senderAvatarUrl,omitempty"`
}

type Member struct {
	UserID      uuid.UUID `json:"userId"`
	Username    string    `json:"username"`
	DisplayName string    `json:"displayName"`
	Role        string    `json:"role"`
	LastReadSeq int64     `json:"lastReadSeq"`

	// AvatarURL ikut, status TIDAK. Anggota sebuah percakapan menurut definisi
	// berbagi percakapan dengan pembacanya, jadi mereka sudah termasuk kontak —
	// dan status kontak sudah datang lewat snapshot saat koneksi dibuka, lalu
	// tetap segar lewat siaran. Menyalinnya ke sini juga berarti dua sumber
	// untuk satu jawaban, dan yang satu ini membeku pada saat daftarnya diambil.
	AvatarURL string `json:"avatarUrl,omitempty"`
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
