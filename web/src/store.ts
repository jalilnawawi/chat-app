import { create } from 'zustand';
import { ApiError, api } from './api';
import { uuidv7 } from './uuid';
import type { Conversation, Member, Message, PendingMessage, User } from './types';

type TypingEntry = { displayName: string; at: number };

type State = {
  me: User | null;
  connected: boolean;

  conversations: Conversation[];
  activeId: string | null;

  /** Pesan yang sudah dikonfirmasi server, urut menaik berdasarkan seq. */
  messages: Record<string, Message[]>;
  /** Pesan optimistik yang belum dikonfirmasi, ditampilkan di bawah daftar. */
  pending: Record<string, PendingMessage[]>;
  hasMore: Record<string, boolean>;
  members: Record<string, Member[]>;

  online: Set<string>;
  typing: Record<string, Record<string, TypingEntry>>;

  setMe: (u: User | null) => void;
  setConnected: (v: boolean) => void;

  loadConversations: () => Promise<void>;
  openConversation: (id: string) => Promise<void>;
  loadOlder: (id: string) => Promise<void>;

  sendMessage: (conversationId: string, body: string) => Promise<void>;
  retryMessage: (conversationId: string, pendingId: string) => Promise<void>;
  editMessage: (id: string, body: string) => Promise<void>;
  deleteMessage: (id: string) => Promise<void>;

  applyMessage: (m: Message) => void;
  applyConversation: (c: Conversation) => void;
  applyRead: (conversationId: string, userId: string, lastReadSeq: number) => void;
  applyTyping: (conversationId: string, userId: string, displayName: string, typing: boolean) => void;
  setOnline: (ids: string[]) => void;
  setPresence: (userId: string, online: boolean) => void;

  /** Cursor per percakapan untuk resume setelah reconnect. */
  syncCursors: () => Record<string, number>;
  markReadUpTo: (conversationId: string) => void;
  reset: () => void;
};

/** Menyisipkan atau memperbarui pesan sambil menjaga urutan seq. */
function upsert(list: Message[], m: Message): Message[] {
  const idx = list.findIndex(x => x.id === m.id);
  if (idx >= 0) {
    const next = list.slice();
    next[idx] = m;
    return next;
  }
  const next = list.slice();
  let i = next.length;
  while (i > 0 && next[i - 1]!.seq > m.seq) i--;
  next.splice(i, 0, m);
  return next;
}

export const useStore = create<State>((set, get) => ({
  me: null,
  connected: false,
  conversations: [],
  activeId: null,
  messages: {},
  pending: {},
  hasMore: {},
  members: {},
  online: new Set(),
  typing: {},

  setMe: me => set({ me }),
  setConnected: connected => set({ connected }),

  reset: () =>
    set({
      me: null,
      conversations: [],
      activeId: null,
      messages: {},
      pending: {},
      hasMore: {},
      members: {},
      online: new Set(),
      typing: {},
    }),

  loadConversations: async () => {
    set({ conversations: await api.conversations() });
  },

  openConversation: async id => {
    set({ activeId: id });

    if (!get().messages[id]) {
      const [page, members] = await Promise.all([api.messages(id), api.members(id)]);
      set(s => ({
        messages: { ...s.messages, [id]: page.messages },
        hasMore: { ...s.hasMore, [id]: page.hasMore },
        members: { ...s.members, [id]: members },
      }));
    }
    get().markReadUpTo(id);
  },

  loadOlder: async id => {
    const list = get().messages[id] ?? [];
    const oldest = list[0]?.seq;
    if (!oldest || !get().hasMore[id]) return;

    const page = await api.messages(id, oldest);
    set(s => ({
      messages: { ...s.messages, [id]: [...page.messages, ...(s.messages[id] ?? [])] },
      hasMore: { ...s.hasMore, [id]: page.hasMore },
    }));
  },

  // Alur kirim optimistik: pesan langsung muncul dengan id final buatan client,
  // lalu dikonfirmasi (atau ditandai gagal) setelah server menjawab. Karena id
  // sudah final, percobaan ulang tidak akan menghasilkan pesan ganda.
  sendMessage: async (conversationId, body) => {
    const id = uuidv7();
    const optimistic: PendingMessage = {
      id,
      conversationId,
      body,
      createdAt: new Date().toISOString(),
      status: 'sending',
    };
    set(s => ({
      pending: { ...s.pending, [conversationId]: [...(s.pending[conversationId] ?? []), optimistic] },
    }));

    await deliver(set, get, conversationId, id, body);
  },

  retryMessage: async (conversationId, pendingId) => {
    const entry = (get().pending[conversationId] ?? []).find(p => p.id === pendingId);
    if (!entry) return;

    set(s => ({
      pending: {
        ...s.pending,
        [conversationId]: (s.pending[conversationId] ?? []).map(p =>
          p.id === pendingId ? { ...p, status: 'sending' as const } : p,
        ),
      },
    }));
    await deliver(set, get, conversationId, pendingId, entry.body);
  },

  editMessage: async (id, body) => {
    get().applyMessage(await api.editMessage(id, body));
  },

  deleteMessage: async id => {
    get().applyMessage(await api.deleteMessage(id));
  },

  applyMessage: m =>
    set(s => {
      const list = upsert(s.messages[m.conversationId] ?? [], m);
      const conversations = s.conversations.map(c =>
        c.id === m.conversationId
          ? {
              ...c,
              lastMessage: m,
              lastSeq: Math.max(c.lastSeq, m.seq),
              updatedAt: m.createdAt,
              unread:
                c.id === s.activeId || m.senderId === s.me?.id
                  ? 0
                  : Math.max(c.lastSeq, m.seq) - c.lastReadSeq,
            }
          : c,
      );
      conversations.sort((a, b) => b.updatedAt.localeCompare(a.updatedAt));

      return {
        messages: { ...s.messages, [m.conversationId]: list },
        // Konfirmasi server menggantikan versi optimistiknya (id-nya sama).
        pending: {
          ...s.pending,
          [m.conversationId]: (s.pending[m.conversationId] ?? []).filter(p => p.id !== m.id),
        },
        conversations,
      };
    }),

  applyConversation: c =>
    set(s => (s.conversations.some(x => x.id === c.id) ? s : { conversations: [c, ...s.conversations] })),

  applyRead: (conversationId, userId, lastReadSeq) =>
    set(s => ({
      members: {
        ...s.members,
        [conversationId]: (s.members[conversationId] ?? []).map(m =>
          m.userId === userId ? { ...m, lastReadSeq } : m,
        ),
      },
      conversations: s.conversations.map(c =>
        c.id === conversationId && userId === s.me?.id
          ? { ...c, lastReadSeq, unread: Math.max(0, c.lastSeq - lastReadSeq) }
          : c,
      ),
    })),

  applyTyping: (conversationId, userId, displayName, typing) =>
    set(s => {
      const room = { ...(s.typing[conversationId] ?? {}) };
      if (typing) room[userId] = { displayName, at: Date.now() };
      else delete room[userId];
      return { typing: { ...s.typing, [conversationId]: room } };
    }),

  setOnline: ids => set({ online: new Set(ids) }),

  setPresence: (userId, online) =>
    set(s => {
      const next = new Set(s.online);
      if (online) next.add(userId);
      else next.delete(userId);
      return { online: next };
    }),

  syncCursors: () => {
    const { messages } = get();
    const cursors: Record<string, number> = {};
    for (const [id, list] of Object.entries(messages)) {
      cursors[id] = list.length ? list[list.length - 1]!.seq : 0;
    }
    return cursors;
  },

  markReadUpTo: conversationId => {
    const list = get().messages[conversationId] ?? [];
    const last = list[list.length - 1];
    const conv = get().conversations.find(c => c.id === conversationId);
    if (!last || !conv || conv.lastReadSeq >= last.seq) return;

    set(s => ({
      conversations: s.conversations.map(c =>
        c.id === conversationId ? { ...c, lastReadSeq: last.seq, unread: 0 } : c,
      ),
    }));
    void api.markRead(conversationId, last.seq).catch(() => {});
  },
}));

/** Satu percobaan kirim; dipakai baik oleh kiriman pertama maupun retry. */
async function deliver(
  set: (fn: (s: State) => Partial<State>) => void,
  get: () => State,
  conversationId: string,
  id: string,
  body: string,
) {
  try {
    const saved = await api.sendMessage(conversationId, id, body);
    get().applyMessage(saved);
  } catch (err) {
    // Kena kuota: tunggu selama yang diminta server, lalu coba sekali lagi
    // diam-diam. Aman justru karena `id` dibuat di client — pengiriman ulang
    // menghasilkan pesan yang sama, bukan pesan kedua. Menampilkan "gagal" di
    // sini akan menyuruh orang menekan tombol kirim lagi, dan tekanan tepat
    // saat server sedang minta pelan-pelan adalah kebalikan dari yang berguna.
    if (err instanceof ApiError && err.status === 429) {
      await new Promise(r => setTimeout(r, Math.min(err.retryAfterMs || 1000, 10_000)));
      try {
        get().applyMessage(await api.sendMessage(conversationId, id, body));
        return;
      } catch {
        // Jatuh ke penandaan gagal di bawah.
      }
    }
    set(s => ({
      pending: {
        ...s.pending,
        [conversationId]: (s.pending[conversationId] ?? []).map(p =>
          p.id === id ? { ...p, status: 'failed' as const } : p,
        ),
      },
    }));
  }
}
