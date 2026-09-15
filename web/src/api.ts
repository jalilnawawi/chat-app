import type { Conversation, Member, Message, User } from './types';

// Kosong = satu origin dengan halaman (dev memakai proxy Vite, produksi
// memakai reverse proxy). Isi VITE_API_URL hanya kalau backend memang berada di
// origin lain — dan ingat konsekuensinya: cookie sesi jadi lintas site.
const BASE = import.meta.env.VITE_API_URL ?? '';

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
  sendMessage: (conversationId: string, id: string, body: string) =>
    post<Message>(`/api/conversations/${conversationId}/messages`, { id, body }),

  editMessage: (id: string, body: string) =>
    request<Message>(`/api/messages/${id}`, { method: 'PATCH', body: JSON.stringify({ body }) }),

  deleteMessage: (id: string) => request<Message>(`/api/messages/${id}`, { method: 'DELETE' }),

  markRead: (conversationId: string, seq: number) =>
    post<{ lastReadSeq: number }>(`/api/conversations/${conversationId}/read`, { seq }),
};
