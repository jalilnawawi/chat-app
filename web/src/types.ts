// Bentuk data ini harus sama persis dengan JSON dari server Go.
// Lihat server/internal/store/models.go dan server/internal/hub/hub.go.

/**
 * Pengguna SEBAGAIMANA DILIHAT ORANG LAIN.
 *
 * Tidak memuat email, dan itu disengaja: bentuk ini ikut di hasil pencarian, di
 * `peer` pada daftar percakapan, dan di setiap siaran yang menyebut seseorang.
 * Bagian pribadi tinggal di `Me`. Lihat server/internal/store/models.go.
 */
export type User = {
  id: string;
  username: string;
  displayName: string;
  createdAt: string;
  /**
   * Kosong berarti belum ada foto; client menampilkan huruf pertama namanya.
   *
   * Alamatnya memuat id UNGGAHAN, bukan id penggunanya, jadi dia berubah setiap
   * fotonya berubah — itu yang membuat cache setahun di sisi server benar.
   */
  avatarUrl?: string;
  /**
   * Status BUKAN presence.
   *
   * Presence (`online`) diturunkan dari koneksi yang hidup dan hilang begitu
   * tabnya ditutup. Status adalah pernyataan yang dibuat orang dengan sengaja
   * dan bertahan melewati tutup laptop, ganti perangkat, dan logout. Titik di
   * sidebar menampilkan GABUNGAN keduanya.
   */
  status: StatusKind;
  statusText?: string;
  /**
   * Instan absolut (ISO-8601). Server tidak pernah membersihkan status yang
   * lewat waktunya — dia menyaringnya saat dibaca — dan client menghitung
   * mundur sendiri dari angka ini, dalam jam LOKAL pembacanya.
   */
  statusExpiresAt?: string;
};

export type StatusKind = 'available' | 'busy' | 'away';

/** Pengguna sebagaimana dilihat DIRINYA SENDIRI. Satu-satunya yang membawa email. */
export type Me = User & {
  email?: string;
  /**
   * Alamat yang belum dibuktikan kepemilikannya TIDAK bisa dipakai memulihkan
   * akun sama sekali — kalau bisa, memulihkan akun orang lain cuma butuh
   * mengaku memiliki sebuah alamat.
   */
  emailVerified: boolean;
};

/** Status seseorang tanpa sisa identitasnya: bentuk yang disiarkan. */
export type UserStatus = {
  userId: string;
  status: StatusKind;
  text?: string;
  expiresAt?: string;
};

/** Satu perangkat yang sedang login. */
export type Session = {
  id: string;
  userAgent: string;
  createdAt: string;
  lastSeenAt: string | null;
  expiresAt: string;
  /** Sesi yang sedang dipakai membaca halaman ini; tidak ditawari tombol cabut. */
  current: boolean;
};

/**
 * Lampiran sebuah pesan.
 *
 * `url` selalu datang dari server dan tidak pernah dirangkai di sini: dia
 * menunjuk ke API ini, bukan ke penyimpanan objek, karena hak baca sebuah
 * lampiran ditentukan oleh keanggotaan percakapan — dan hanya server yang tahu
 * soal itu. Konsekuensinya, <img src> memerlukan cookie sesi, dan itu memang
 * terkirim otomatis untuk permintaan satu origin.
 */
export type Attachment = {
  id: string;
  url: string;
  name: string;
  mime: string;
  size: number;
  width?: number;
  height?: number;
  /**
   * Alamat turunan kecil, kosong bila tidak ada.
   *
   * Tiga hal berakhir tanpa turunan: bukan gambar, sudah cukup kecil untuk
   * dipakai apa adanya, dan pembuatannya gagal di server. Client tidak perlu
   * membedakan ketiganya — jawabannya sama, pakai `url`.
   */
  thumbUrl?: string;
};

/**
 * Secuil pesan yang dibalas, secukupnya untuk gelembung kutipan.
 *
 * Datang dari self-join di server dan TIDAK pernah disalin ke baris
 * pembalasnya, jadi isinya selalu keadaan terbaru: yang sudah diedit tampil
 * versi barunya, yang sudah dihapus tampil dengan `deleted: true`.
 */
export type ReplyPreview = {
  id: string;
  seq: number;
  senderId: string;
  body: string;
  deleted: boolean;
  /** Diisi bila pesannya tidak punya teks sama sekali, hanya lampiran. */
  kind?: 'image' | 'video' | 'audio' | 'file';
};

/**
 * Satu emoji pada satu pesan, sudah dihitung di server.
 *
 * `mine` adalah satu-satunya bagian sebuah pesan yang jawabannya berbeda per
 * pembaca — dan itu sebabnya siaran reaksi membawa SELISIH, bukan ringkasan
 * ini. Lihat catatan di store.applyReaction.
 */
export type ReactionSummary = {
  emoji: string;
  count: number;
  mine: boolean;
};

/** Orang yang disebut sebuah catatan sistem, beserta namanya PADA SAAT ITU. */
export type SystemParty = { id: string; name: string };

/**
 * Kejadian yang dicatat sistem: keanggotaan berubah, judul berganti.
 *
 * Yang disimpan server adalah KEJADIANNYA, bukan kalimatnya — kalimatnya
 * disusun di sini, sehingga bahasanya bisa berubah tanpa menulis ulang riwayat
 * siapa pun.
 */
export type SystemEvent = {
  type: 'member.added' | 'member.removed' | 'member.left' | 'title.changed' | 'owner.changed';
  actor: SystemParty;
  targets?: SystemParty[];
  /** Judul BARU, untuk title.changed. */
  title?: string;
};

export type Message = {
  id: string;
  conversationId: string;
  seq: number;
  senderId: string;
  body: string;
  attachments: Attachment[];
  createdAt: string;
  editedAt: string | null;
  deletedAt: string | null;
  /** null berarti pesan ini tidak membalas apa pun. */
  replyTo?: ReplyPreview;
  /** Id yang disebut, sudah lolos pemeriksaan keanggotaan di server. */
  mentions: string[];
  mentionsAll: boolean;
  reactions: ReactionSummary[];
  /**
   * Jam kedua: nilai penghitung reaksi percakapan saat terakhir kali reaksi
   * pesan ini berubah. Nol berarti belum pernah ada yang bereaksi.
   *
   * Dia dipakai untuk dua hal: cursor resume kedua (di samping `seq`), dan
   * gerbang yang membuat satu perubahan tidak pernah terpasang dua kali —
   * event yang sama datang lewat jawaban HTTP DAN lewat siaran WebSocket.
   */
  reactionSeq: number;
  /**
   * 'system' untuk catatan keanggotaan; 'user' untuk yang ditulis orang.
   *
   * Catatan sistem menumpang tabel pesan yang sama supaya dia ikut terbawa oleh
   * riwayat, cursor, dan susulan setelah reconnect tanpa jalur baru. Yang
   * membedakannya cuma kolom ini — dan kolom ini pula yang menutup jalur edit,
   * hapus, balas, dan reaksi untuknya.
   */
  kind: 'user' | 'system';
  systemEvent?: SystemEvent;
};

export type Member = {
  userId: string;
  username: string;
  displayName: string;
  role: 'owner' | 'member';
  lastReadSeq: number;
  /**
   * Status sengaja TIDAK ikut di sini. Anggota sebuah percakapan menurut
   * definisi berbagi percakapan dengan kita, jadi mereka sudah termasuk kontak —
   * dan status kontak datang lewat snapshot saat koneksi dibuka lalu tetap segar
   * lewat siaran. Menyalinnya ke sini berarti dua sumber untuk satu jawaban, dan
   * yang satu ini membeku pada saat daftarnya diambil.
   */
  avatarUrl?: string;
};

export type Conversation = {
  id: string;
  type: 'direct' | 'group';
  title: string | null;
  lastSeq: number;
  lastReadSeq: number;
  unread: number;
  peer: User | null;
  lastMessage: Message | null;
  updatedAt: string;
  /**
   * mentionSeq > mentionAckSeq berarti ada yang menyebut nama kita dan kita
   * belum sampai ke pesannya.
   *
   * Sengaja dua angka, terpisah dari lastReadSeq. "Ada pesan baru" hilang
   * begitu ruangnya dibuka; "ada yang memanggil kamu" tidak boleh hilang
   * sampai pesannya benar-benar terlihat.
   */
  mentionSeq: number;
  mentionAckSeq: number;
};

/** Pesan yang sudah tampil di layar tapi belum dikonfirmasi server. */
export type PendingMessage = {
  id: string;
  conversationId: string;
  body: string;
  attachments: Attachment[];
  createdAt: string;
  status: 'sending' | 'failed';
  replyTo?: ReplyPreview;
  /**
   * Ikut disimpan supaya "coba lagi" mengirim pesan yang SAMA — termasuk siapa
   * yang disebut. Pengiriman ulang yang kehilangan sebutannya adalah pesan
   * yang tidak membangunkan siapa pun, dan itu tidak terlihat oleh
   * pengirimnya.
   */
  mentionedUserIds: string[];
  mentionsAll: boolean;
  /**
   * Sebab kegagalan, ditampilkan di samping tombol coba lagi.
   *
   * Tanpa ini, pesan yang ditolak karena kuota @semua terlihat persis sama
   * dengan pesan yang gagal karena jaringan — dan orang menekan "coba lagi"
   * berulang kali untuk penolakan yang memang belum waktunya berubah.
   */
  error?: string;
};

/**
 * Berkas yang sedang atau sudah diunggah, tapi pesannya belum dikirim.
 *
 * Unggahan dipisah dari pengiriman pesan supaya keduanya bisa gagal sendiri-
 * sendiri: berkas sepuluh megabyte butuh waktu dan bisa putus di tengah,
 * sedangkan mengirim pesan harus tetap satu tindakan cepat yang jawabannya
 * pasti. Yang dikirim bersama pesan hanyalah id dari unggahan yang sudah
 * selesai.
 */
export type Upload = {
  /** Kunci lokal, bukan id lampiran — id baru ada setelah server menerima. */
  key: string;
  name: string;
  size: number;
  mime: string;
  /** objectURL untuk pratinjau gambar; wajib dilepas saat unggahan dibuang. */
  previewUrl: string | null;
  /** 0..1 */
  progress: number;
  status: 'uploading' | 'ready' | 'failed';
  attachment: Attachment | null;
  error: string | null;
  /**
   * Apakah mencoba lagi masih ada gunanya.
   *
   * Unggahan yang putus di tengah jalan layak diulang; yang ditolak karena
   * ukurannya tidak, karena ukurannya tidak akan berubah.
   */
  retriable: boolean;
};

/** Event yang datang dari server lewat WebSocket. */
export type ServerEvent =
  | { type: 'message.new'; payload: Message }
  | { type: 'message.updated'; payload: Message }
  | { type: 'conversation.new'; payload: Conversation }
  /**
   * Keadaan grup setelah dikelola. Terpisah dari catatan sistem yang
   * menyertainya, dan keduanya memang datang berpasangan: catatan itu adalah
   * KEJADIAN yang masuk riwayat, ini adalah KEADAAN sekarang yang tidak punya
   * tempat di riwayat.
   */
  | {
      type: 'conversation.updated';
      payload: { conversationId: string; title: string; members: Member[] };
    }
  /** Dikirim HANYA kepada orang yang baru saja berhenti jadi anggota. */
  | { type: 'conversation.removed'; payload: { conversationId: string } }
  | { type: 'read.updated'; payload: { conversationId: string; userId: string; lastReadSeq: number } }
  | { type: 'typing'; payload: { conversationId: string; userId: string; displayName: string; typing: boolean } }
  | { type: 'presence'; payload: { userId: string; online: boolean } }
  | { type: 'presence.snapshot'; payload: { online: string[] } }
  /**
   * Status yang baru dipasang seseorang, dan keadaan awal saat koneksi dibuka.
   *
   * Jalurnya persis sama dengan presence — siaran ke kontak, snapshot ke satu
   * koneksi — supaya tidak ada mekanisme fan-out kedua yang harus ikut benar.
   * Snapshot hanya memuat yang BUKAN bawaan: yang available tanpa teks tidak
   * pernah dikirim, karena client sudah menganggap semua orang begitu.
   */
  | { type: 'status'; payload: UserStatus }
  | { type: 'status.snapshot'; payload: { statuses: UserStatus[] } }
  /**
   * Nama tampilan atau foto seseorang berubah.
   *
   * Tanpa event ini, foto baru memang punya alamat baru — tapi tidak seorang
   * pun tahu alamat itu sampai halamannya dimuat ulang.
   */
  | { type: 'user.updated'; payload: User }
  /**
   * Sesi koneksi ini baru saja dicabut, dan koneksinya ditutup tepat setelah
   * frame ini. Satu-satunya event yang arahnya kebalikan dari yang lain: dia
   * bukan kabar tentang percakapan, melainkan akhir dari sesi ini.
   */
  | { type: 'session.revoked'; payload: { reason: string } }
  /**
   * Pesan susulan setelah reconnect, dikirim berkelompok dalam satu frame.
   * Satu frame per pesan akan meluberkan antrean kirim koneksi saat banyak
   * orang menyusul bersamaan — lihat catatan di server/internal/ws/handler.go.
   */
  | { type: 'sync.batch'; payload: { conversationId: string; messages: Message[] } }
  | { type: 'sync.complete'; payload: Record<string, never> }
  /**
   * Perubahan reaksi, satu penekanan per event.
   *
   * Yang dikirim adalah SELISIH, bukan ringkasan jadi: ringkasan memuat
   * "apakah aku ikut", dan siaran adalah satu payload yang sama untuk semua
   * orang. Client menerapkannya pada hitungan yang sudah dia punya.
   */
  | {
      type: 'reaction.added' | 'reaction.removed';
      payload: {
        conversationId: string;
        messageId: string;
        userId: string;
        emoji: string;
        reactionSeq: number;
      };
    }
  /**
   * Susulan reaksi setelah reconnect, dan ini dikirim ke SATU koneksi — jadi
   * ringkasan lengkap beserta `mine` memang boleh ada di sini.
   *
   * Reaksi tidak pernah muat di cursor `seq`: dia mengubah pesan lama, yang
   * seq-nya sudah berhenti bergerak. Tanpa jalur ini, menutup laptop lalu
   * membukanya lagi menghasilkan riwayat lengkap dengan reaksi yang hilang.
   */
  | {
      type: 'reaction.batch';
      payload: {
        conversationId: string;
        messages: { messageId: string; reactionSeq: number; reactions: ReactionSummary[] }[];
      };
    }
  /**
   * Server pamit terencana (rolling deploy). Bedanya dengan koneksi yang putus
   * begitu saja: instance pengganti SUDAH siap, jadi client boleh menyambung
   * lagi hampir seketika alih-alih mundur bertahap seperti menghadapi gangguan.
   */
  | { type: 'server.shutdown'; payload: { reason: string } }
  | { type: 'error'; payload: { message: string } };

/**
 * Jawaban GET /api/config: apa yang bisa dilakukan server ini.
 *
 * Ada supaya tombol yang PASTI ditolak tidak pernah ditampilkan. Aturan itu
 * sudah dipakai di panel kelola grup — tombol yang bukan hak seseorang tidak
 * ditampilkan kepadanya, bukan ditampilkan lalu ditolak server — dan lampiran,
 * foto profil, serta pemulihan password sekarang mengikutinya juga.
 *
 * Diminta SEKALI saat aplikasi dibuka, sebelum siapa pun login: halaman masuk
 * sudah membutuhkan `mail` untuk memutuskan apakah "Lupa password?" pantas
 * ditawarkan.
 */
export type ServerConfig = {
  /** Unggahan lampiran menyala (`SEAWEED_FILER_URL` diisi). */
  attachments: boolean;
  /** Turunan gambar dibuat (`THUMBNAIL_MAX_DIM` bukan nol). */
  thumbnails: boolean;
  /**
   * Foto profil bisa diunggah.
   *
   * Menuntut KEDUANYA: byte-nya menumpang penyimpanan yang sama dengan
   * lampiran, dan berkas aslinya dibuang setelah diperkecil — jadi tanpa
   * pengolahan gambar tidak ada yang bisa disimpan sama sekali.
   */
  avatars: boolean;
  /** Push notification menyala (kunci VAPID terisi). */
  push: boolean;
  /** Verifikasi email dan pemulihan password menyala (`SMTP_URL` diisi). */
  mail: boolean;
  /** Kunci publik VAPID. Kosong bila push mati. */
  vapidPublicKey: string;
};
