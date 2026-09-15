import type { Attachment, Conversation, Member, Message, PushConfig, User } from './types';

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
    post<User>('/api/auth/register', { username, displayName, password }),

  login: (username: string, password: string) =>
    post<User>('/api/auth/login', { username, password }),

  logout: () => post<{ status: string }>('/api/auth/logout'),

  me: () => request<User>('/api/auth/me'),

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

  /** id dibuat di client supaya pengiriman ulang tidak menghasilkan duplikat. */
  sendMessage: (conversationId: string, id: string, body: string, attachmentIds: string[] = []) =>
    post<Message>(`/api/conversations/${conversationId}/messages`, { id, body, attachmentIds }),

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
    dimensions: { width: number; height: number } | null,
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
      const query = dimensions ? `?w=${dimensions.width}&h=${dimensions.height}` : '';

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

  pushConfig: () => request<PushConfig>('/api/push/config'),

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

  markRead: (conversationId: string, seq: number) =>
    post<{ lastReadSeq: number }>(`/api/conversations/${conversationId}/read`, { seq }),
};
