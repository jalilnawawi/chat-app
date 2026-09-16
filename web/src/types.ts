// Bentuk data ini harus sama persis dengan JSON dari server Go.
// Lihat server/internal/store/models.go dan server/internal/hub/hub.go.

export type User = {
  id: string;
  username: string;
  displayName: string;
  createdAt: string;
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
};

export type Member = {
  userId: string;
  username: string;
  displayName: string;
  role: 'owner' | 'member';
  lastReadSeq: number;
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
  | { type: 'read.updated'; payload: { conversationId: string; userId: string; lastReadSeq: number } }
  | { type: 'typing'; payload: { conversationId: string; userId: string; displayName: string; typing: boolean } }
  | { type: 'presence'; payload: { userId: string; online: boolean } }
  | { type: 'presence.snapshot'; payload: { online: string[] } }
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

/** Jawaban GET /api/push/config. */
export type PushConfig = {
  enabled: boolean;
  publicKey: string;
};
