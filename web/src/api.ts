import type {
  Attachment,
  Conversation,
  Me,
  Member,
  Message,
  Pin,
  SearchResult,
  ServerConfig,
  Session,
  StatusKind,
  User,
  UserStatus,
} from './types';

/**
 * Bagian pesan yang bukan teks maupun lampiran.
 *
 * `mentionedUserIds` dikirim EKSPLISIT dan tidak diurai server dari "@nama".
 * Nama tampilan boleh mengandung spasi, tapi alasan utamanya bukan itu: siapa
 * yang dibangunkan tidak boleh ditentukan oleh cara sebuah string kebetulan
 * ditulis. Server tetap memeriksa tiap id — client tidak pernah dipercaya soal
 * siapa yang berhak dibangunkan.
 */
export type SendExtras = {
  attachmentIds?: string[];
  replyToId?: string;
  mentionedUserIds?: string[];
  mentionsAll?: boolean;
};

// Kosong = satu origin dengan halaman (dev memakai proxy Vite, produksi
// memakai reverse proxy). Isi VITE_API_URL hanya kalau backend memang berada di
// origin lain — dan ingat konsekuensinya: cookie sesi jadi lintas site.
const BASE = import.meta.env.VITE_API_URL ?? '';

/**
 * Alamat lengkap sebuah lampiran.
 *
 * Server mengirim URL relatif (`/api/attachments/<id>`), dan itu benar — bentuk
 * alamatnya boleh berubah tanpa menulis ulang pesan lama. Tapi `<img src>` dan
 * `<a href>` tidak melewati `request()`, jadi awalan BASE harus ditempelkan di
 * sini. Tanpa ini, menyetel VITE_API_URL membuat semua lampiran 404 sementara
 * seluruh bagian aplikasi lain tetap jalan — kegagalan yang hanya muncul di satu
 * konfigurasi dan tidak terlihat sama sekali di konfigurasi lainnya.
 */
export const attachmentURL = (url: string) => (BASE ? BASE + url : url);

/**
 * Alamat lengkap sebuah foto profil.
 *
 * Fungsi yang sama bentuknya dengan attachmentURL, dan sengaja tidak digabung:
 * keduanya kebetulan sama HARI INI, tapi izin bacanya berbeda — avatar boleh
 * dilihat siapa pun yang sudah login, lampiran hanya oleh anggota percakapannya.
 * Dua aturan yang berbeda layak punya dua nama, supaya yang satu tidak ikut
 * berubah saat yang lain diubah.
 */
export const avatarURL = (url: string) => (BASE ? BASE + url : url);

export const wsURL = () => {
  if (BASE) return BASE.replace(/^http/, 'ws') + '/ws';
  const scheme = location.protocol === 'https:' ? 'wss:' : 'ws:';
  return `${scheme}//${location.host}/ws`;
};

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
    /**
     * Diisi dari header Retry-After saat server menolak karena kuota (429).
     * Server tahu persis kapan token berikutnya tersedia; menebak sendiri
     * berarti mencoba terlalu cepat dan ditolak lagi.
     */
    public retryAfterMs = 0,
  ) {
    super(message);
  }
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const res = await fetch(BASE + path, {
    ...init,
    // Wajib: sesi memakai cookie httpOnly, dan ini permintaan lintas origin
    // (5174 -> 8090). Tanpa ini cookie tidak ikut terkirim.
    credentials: 'include',
    headers: { 'Content-Type': 'application/json', ...init.headers },
  });

  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: res.statusText }));
    const retryAfter = Number(res.headers.get('Retry-After')) * 1000;
    throw new ApiError(
      res.status,
      body.error ?? 'Terjadi kesalahan',
      Number.isFinite(retryAfter) ? retryAfter : 0,
    );
  }
  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}

const post = <T,>(path: string, body?: unknown) =>
  request<T>(path, { method: 'POST', body: body === undefined ? undefined : JSON.stringify(body) });

export const api = {
  register: (username: string, displayName: string, password: string) =>
    post<Me>('/api/auth/register', { username, displayName, password }),

  login: (username: string, password: string) => post<Me>('/api/auth/login', { username, password }),

  logout: () => post<{ status: string }>('/api/auth/logout'),

  me: () => request<Me>('/api/auth/me'),

  // ---- kelola akun & profil ----

  updateProfile: (displayName: string) =>
    request<Me>('/api/account', { method: 'PATCH', body: JSON.stringify({ displayName }) }),

  /**
   * Memasang status.
   *
   * `expiresAt` adalah INSTAN ABSOLUT yang dihitung DI SINI dari pilihan cepat
   * orangnya, bukan durasi yang dikirim ke server. Hanya client yang tahu zona
   * waktu pemakainya, dan "sampai akhir hari" adalah pertanyaan yang cuma bisa
   * dijawab di sini — pukul 23.59 di Jakarta adalah tengah hari di tempat lain.
   */
  setStatus: (status: StatusKind, text: string, expiresAt: string | null) =>
    request<UserStatus>('/api/account/status', {
      method: 'PUT',
      body: JSON.stringify({ status, text, expiresAt }),
    }),

  /** Menuntut password saat ini: sesi bisa saja milik laptop yang ditinggal terbuka. */
  changePassword: (currentPassword: string, newPassword: string) =>
    post<{ status: string }>('/api/account/password', { currentPassword, newPassword }),

  setEmail: (email: string, currentPassword: string) =>
    post<Me>('/api/account/email', { email, currentPassword }),

  resendVerification: () => post<{ status: string }>('/api/account/email/verify'),

  verifyEmail: (token: string) => post<{ email: string }>('/api/auth/verify-email', { token }),

  /**
   * Jawabannya selalu sama, terdaftar atau tidak — membedakannya mengubah
   * endpoint ini jadi alat untuk menanyai server siapa saja yang punya akun.
   */
  forgotPassword: (email: string) => post<{ status: string }>('/api/auth/forgot-password', { email }),

  resetPassword: (token: string, password: string) =>
    post<{ status: string }>('/api/auth/reset-password', { token, password }),

  sessions: () => request<Session[]>('/api/account/sessions'),

  revokeSession: (id: string) =>
    request<{ status: string }>(`/api/account/sessions/${id}`, { method: 'DELETE' }),

  removeAvatar: () => request<Me>('/api/account/avatar', { method: 'DELETE' }),

  /**
   * Mengunggah foto profil.
   *
   * Memakai XMLHttpRequest dengan alasan yang sama dengan uploadAttachment:
   * fetch tidak punya cara melaporkan kemajuan unggahan sama sekali.
   */
  uploadAvatar: (file: File, onProgress: (fraction: number) => void): Promise<Me> =>
    new Promise((resolve, reject) => {
      const form = new FormData();
      form.append('file', file, file.name);

      const xhr = new XMLHttpRequest();
      xhr.open('POST', `${BASE}/api/account/avatar`);
      xhr.withCredentials = true;
      xhr.responseType = 'json';

      xhr.upload.onprogress = e => {
        if (e.lengthComputable) onProgress(e.loaded / e.total);
      };
      xhr.onload = () => {
        if (xhr.status >= 200 && xhr.status < 300) {
          onProgress(1);
          resolve(xhr.response as Me);
          return;
        }
        const body = xhr.response as { error?: string } | null;
        reject(new ApiError(xhr.status, body?.error ?? 'Unggahan gagal'));
      };
      xhr.onerror = () => reject(new ApiError(0, 'Jaringan bermasalah saat mengunggah'));
      xhr.send(form);
    }),

  searchUsers: (q: string) => request<User[]>(`/api/users?q=${encodeURIComponent(q)}`),

  conversations: () => request<Conversation[]>('/api/conversations'),

  openDirect: (peerId: string) =>
    post<{ conversationId: string }>('/api/conversations/direct', { peerId }),

  createGroup: (title: string, memberIds: string[]) =>
    post<{ conversationId: string }>('/api/conversations/group', { title, memberIds }),

  members: (conversationId: string) =>
    request<Member[]>(`/api/conversations/${conversationId}/members`),

  /** before = 0 berarti mulai dari pesan terbaru (cursor pagination). */
  messages: (conversationId: string, before = 0, limit = 50) =>
    request<{ messages: Message[]; hasMore: boolean }>(
      `/api/conversations/${conversationId}/messages?before=${before}&limit=${limit}`,
    ),

  /**
   * Jendela riwayat yang MEMUAT pesan `seq`, untuk melompat ke pesan yang jauh
   * di belakang. Jendelanya belum tentu menyentuh ujung terbaru — `hasNewer`
   * yang mengatakannya.
   */
  messagesAround: (conversationId: string, seq: number, limit = 50) =>
    request<{ messages: Message[]; hasMore: boolean; hasNewer: boolean }>(
      `/api/conversations/${conversationId}/messages?around=${seq}&limit=${limit}`,
    ),

  /** Lanjutan jendela ke arah yang lebih baru. */
  messagesAfter: (conversationId: string, after: number, limit = 50) =>
    request<{ messages: Message[]; hasNewer: boolean }>(
      `/api/conversations/${conversationId}/messages?after=${after}&limit=${limit}`,
    ),

  /**
   * Mencari isi pesan. Tanpa `conversationId` berarti di semua percakapan.
   *
   * Server mencocokkan kata utuh dan AWALANNYA (untuk kata tiga huruf ke
   * atas): "kirim" menemukan "kirimkan", tapi tidak "dikirim".
   */
  search: (q: string, opts: { conversationId?: string; cursor?: string } = {}) => {
    const params = new URLSearchParams({ q });
    if (opts.conversationId) params.set('conversationId', opts.conversationId);
    if (opts.cursor) params.set('cursor', opts.cursor);
    return request<SearchResult>(`/api/search?${params}`);
  },

  pins: (conversationId: string) => request<Pin[]>(`/api/conversations/${conversationId}/pins`),

  pin: (messageId: string) =>
    request<PinResult>(`/api/messages/${messageId}/pin`, { method: 'PUT' }),

  unpin: (messageId: string) =>
    request<PinResult>(`/api/messages/${messageId}/pin`, { method: 'DELETE' }),

  /**
   * Meneruskan satu pesan ke satu percakapan.
   *
   * Meneruskan adalah MENGIRIM PESAN — jalur yang sama, id dari client yang
   * sama — yang isinya disalin server. Meneruskan ke lima percakapan adalah
   * lima panggilan ini, dan satu yang gagal tidak membatalkan yang lain.
   */
  forwardMessage: (conversationId: string, id: string, fromMessageId: string) =>
    post<Message>(`/api/conversations/${conversationId}/messages`, {
      id,
      body: '',
      forwardFromId: fromMessageId,
    }),

  /** id dibuat di client supaya pengiriman ulang tidak menghasilkan duplikat. */
  sendMessage: (conversationId: string, id: string, body: string, extras: SendExtras = {}) =>
    post<Message>(`/api/conversations/${conversationId}/messages`, {
      id,
      body,
      attachmentIds: extras.attachmentIds ?? [],
      replyToId: extras.replyToId ?? null,
      mentionedUserIds: extras.mentionedUserIds ?? [],
      mentionsAll: extras.mentionsAll ?? false,
    }),

  /**
   * Mengunggah satu berkas, dengan laporan kemajuan.
   *
   * Memakai XMLHttpRequest, bukan fetch. Itu bukan pilihan gaya: fetch tidak
   * punya cara melaporkan kemajuan UNGGAHAN sama sekali. Untuk berkas sepuluh
   * megabyte, bedanya adalah antara bilah yang bergerak dan layar yang diam
   * selama setengah menit — dan layar yang diam membuat orang menekan tombol
   * kirim berkali-kali.
   */
  uploadAttachment: (
    file: File,
    meta: { width?: number; height?: number; durationMs?: number },
    onProgress: (fraction: number) => void,
    signal?: AbortSignal,
  ): Promise<Attachment> =>
    new Promise((resolve, reject) => {
      const form = new FormData();
      form.append('file', file, file.name);

      // Ukuran gambar lewat query, bukan field form. Server memproses unggahan
      // sebagai aliran dan meneruskannya ke penyimpanan tanpa menyangganya,
      // jadi field yang datang SETELAH berkas tidak akan pernah terbaca tepat
      // waktu. Query string sudah lengkap sebelum byte pertama dikirim.
      const params = new URLSearchParams();
      if (meta.width && meta.height) {
        params.set('w', String(meta.width));
        params.set('h', String(meta.height));
      }
      if (meta.durationMs) params.set('d', String(Math.round(meta.durationMs)));
      const query = params.toString() ? `?${params}` : '';

      const xhr = new XMLHttpRequest();
      xhr.open('POST', `${BASE}/api/attachments${query}`);
      xhr.withCredentials = true;
      xhr.responseType = 'json';

      xhr.upload.onprogress = e => {
        if (e.lengthComputable) onProgress(e.loaded / e.total);
      };

      xhr.onload = () => {
        if (xhr.status >= 200 && xhr.status < 300) {
          onProgress(1);
          resolve(xhr.response as Attachment);
          return;
        }
        const body = xhr.response as { error?: string } | null;
        const retryAfter = Number(xhr.getResponseHeader('Retry-After')) * 1000;
        reject(
          new ApiError(
            xhr.status,
            body?.error ?? 'Unggahan gagal',
            Number.isFinite(retryAfter) ? retryAfter : 0,
          ),
        );
      };

      xhr.onerror = () => reject(new ApiError(0, 'Jaringan bermasalah saat mengunggah'));
      xhr.onabort = () => reject(new ApiError(0, 'Unggahan dibatalkan'));

      signal?.addEventListener('abort', () => xhr.abort(), { once: true });
      xhr.send(form);
    }),

  /**
   * Apa yang bisa dilakukan server ini. Tanpa sesi: halaman masuk sudah
   * membutuhkannya untuk memutuskan apakah "Lupa password?" pantas ditawarkan.
   */
  serverConfig: () => request<ServerConfig>('/api/config'),

  pushSubscribe: (sub: PushSubscriptionJSON) =>
    post<{ status: string }>('/api/push/subscribe', {
      endpoint: sub.endpoint,
      keys: { p256dh: sub.keys?.p256dh, auth: sub.keys?.auth },
    }),

  pushUnsubscribe: (endpoint: string) =>
    post<{ status: string }>('/api/push/unsubscribe', { endpoint }),

  editMessage: (id: string, body: string) =>
    request<Message>(`/api/messages/${id}`, { method: 'PATCH', body: JSON.stringify({ body }) }),

  deleteMessage: (id: string) => request<Message>(`/api/messages/${id}`, { method: 'DELETE' }),

  /**
   * Hapus yang harus tetap sampai walau halamannya sedang ditutup.
   *
   * `keepalive` membuat browser menyelesaikan permintaan ini setelah tab
   * hilang; jawabannya tidak ditunggu siapa pun, jadi tidak ada yang dibaca.
   */
  deleteMessageOnExit: (id: string) => {
    void fetch(`${BASE}/api/messages/${id}`, {
      method: 'DELETE',
      credentials: 'include',
      keepalive: true,
    }).catch(() => {});
  },

  markRead: (conversationId: string, seq: number) =>
    post<{ lastReadSeq: number }>(`/api/conversations/${conversationId}/read`, { seq }),

  /**
   * Menurunkan penanda "ada yang menyebut kamu".
   *
   * Endpoint terpisah dari markRead, dan itu seluruh gunanya: terbaca bergerak
   * saat percakapannya dibuka, sebutan baru bergerak setelah pesan yang
   * memanggil namanya benar-benar terlihat di layar.
   */
  ackMentions: (conversationId: string, seq: number) =>
    post<{ mentionAckSeq: number }>(`/api/conversations/${conversationId}/mentions/ack`, { seq }),

  /**
   * Pengelolaan grup.
   *
   * Kelimanya menjawab dengan bentuk yang sama — judul dan daftar anggota
   * setelah perubahan — supaya client tidak perlu tahu tindakan mana yang
   * mengubah apa. Siaran ke anggota lain diurus server.
   */
  renameGroup: (conversationId: string, title: string) =>
    request<GroupState>(`/api/conversations/${conversationId}`, {
      method: 'PATCH',
      body: JSON.stringify({ title }),
    }),

  addMembers: (conversationId: string, userIds: string[]) =>
    post<GroupState>(`/api/conversations/${conversationId}/members`, { userIds }),

  removeMember: (conversationId: string, userId: string) =>
    request<GroupState>(`/api/conversations/${conversationId}/members/${userId}`, {
      method: 'DELETE',
    }),

  leaveGroup: (conversationId: string) =>
    post<GroupState>(`/api/conversations/${conversationId}/leave`),

  transferOwnership: (conversationId: string, userId: string) =>
    post<GroupState>(`/api/conversations/${conversationId}/owner`, { userId }),

  addReaction: (messageId: string, emoji: string) =>
    post<ReactionResult>(`/api/messages/${messageId}/reactions`, { emoji }),

  removeReaction: (messageId: string, emoji: string) =>
    request<ReactionResult>(`/api/messages/${messageId}/reactions`, {
      method: 'DELETE',
      body: JSON.stringify({ emoji }),
    }),
};

/**
 * Jawaban atas satu penekanan reaksi.
 *
 * `changed: false` berarti keadaannya memang sudah begitu — emoji yang sama
 * ditekan dua kali karena jaringan lambat. `reactionSeq` adalah nomor jam
 * perubahan itu, dan dialah yang membuat satu perubahan tidak pernah terpasang
 * dua kali walau datang lewat dua jalan sekaligus (jawaban ini dan siaran
 * WebSocket).
 */
/** Keadaan grup setelah satu tindakan pengelolaan. */
export type GroupState = {
  conversationId: string;
  title: string;
  members: Member[];
};

/** Jawaban atas menyematkan atau melepas. `changed: false` = memang sudah begitu. */
export type PinResult = {
  conversationId: string;
  messageId: string;
  pinned: boolean;
  changed: boolean;
  notice?: Message;
};

export type ReactionResult = {
  messageId: string;
  emoji: string;
  changed: boolean;
  reactionSeq: number;
};
