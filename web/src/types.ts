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
};

/** Pesan yang sudah tampil di layar tapi belum dikonfirmasi server. */
export type PendingMessage = {
  id: string;
  conversationId: string;
  body: string;
  attachments: Attachment[];
  createdAt: string;
  status: 'sending' | 'failed';
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
