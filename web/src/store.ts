import { create } from 'zustand';
import { ApiError, api } from './api';
import { uuidv7 } from './uuid';
import type { Attachment, Conversation, Member, Message, PendingMessage, Upload, User } from './types';

/**
 * Batas ukuran berkas di sisi client.
 *
 * Harus sama dengan MAX_UPLOAD_BYTES di server, dan yang MENGIKAT tetap yang di
 * server — ini cuma supaya orang tidak menunggu unggahan sepuluh menit yang
 * sudah pasti ditolak di ujungnya.
 */
const MAX_UPLOAD_BYTES = 10 * 1024 * 1024;

/** Sama dengan maxAttachmentsPerMessage di server. */
const MAX_ATTACHMENTS = 10;

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
  /**
   * Berkas yang sudah dipilih tapi pesannya belum dikirim, per percakapan.
   *
   * Disimpan per percakapan, bukan di komponen, supaya berpindah ruang sebentar
   * lalu kembali tidak menghanguskan unggahan yang sedang berjalan — dan
   * unggahan itu bisa saja sudah separuh jalan.
   */
  uploads: Record<string, Upload[]>;
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
  addFiles: (conversationId: string, files: File[]) => void;
  retryUpload: (conversationId: string, key: string) => void;
  removeUpload: (conversationId: string, key: string) => void;
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

/**
 * Berkas asli disimpan di luar state.
 *
 * Objek File tidak punya tempat di dalam state React: dia besar, tidak bisa
 * disalin, dan identitasnya tidak pernah berubah sehingga tidak menambah apa
 * pun pada perbandingan render. Yang masuk state hanyalah keterangan yang
 * memang ditampilkan — nama, ukuran, kemajuan. Berkasnya sendiri hanya
 * dibutuhkan lagi kalau unggahannya perlu diulang.
 */
const files = new Map<string, File>();

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
  uploads: {},
  hasMore: {},
  members: {},
  online: new Set(),
  typing: {},

  setMe: me => set({ me }),
  setConnected: connected => set({ connected }),

  reset: () =>
    set(s => {
      // objectURL menahan berkasnya di memori sampai dilepas. Logout tanpa ini
      // menyisakan setiap gambar yang pernah dipilih selama sesi itu.
      for (const list of Object.values(s.uploads)) {
        for (const u of list) if (u.previewUrl) URL.revokeObjectURL(u.previewUrl);
      }
      files.clear();
      return {
        me: null,
        conversations: [],
        activeId: null,
        messages: {},
        pending: {},
        uploads: {},
        hasMore: {},
        members: {},
        online: new Set(),
        typing: {},
      };
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
    // Hanya unggahan yang SUDAH selesai yang ikut. Yang masih berjalan
    // ditinggal di laci dan tetap bisa dikirim pada pesan berikutnya — lebih
    // baik daripada menahan pesan yang sudah diketik karena satu berkas besar
    // belum tuntas.
    const ready = (get().uploads[conversationId] ?? []).filter(u => u.status === 'ready');
    const attachments = ready.map(u => u.attachment!).filter(Boolean);

    const id = uuidv7();
    const optimistic: PendingMessage = {
      id,
      conversationId,
      body,
      attachments,
      createdAt: new Date().toISOString(),
      status: 'sending',
    };
    set(s => ({
      pending: { ...s.pending, [conversationId]: [...(s.pending[conversationId] ?? []), optimistic] },
    }));

    // Laci dikosongkan sebelum jawaban datang: lampirannya sudah pindah ke
    // gelembung optimistik, dan membiarkannya tetap terlihat di kolom tulis
    // membuat orang mengira berkasnya belum terkirim.
    //
    // Berkas aslinya ikut dilepas di sini. Dia hanya disimpan untuk keperluan
    // mengulang unggahan yang gagal, dan unggahan yang sudah terkirim tidak akan
    // pernah diulang — menahannya berarti setiap foto yang pernah dikirim tetap
    // menempel di memori sampai tabnya ditutup.
    for (const u of ready) {
      if (u.previewUrl) URL.revokeObjectURL(u.previewUrl);
      files.delete(u.key);
    }
    set(s => ({
      uploads: {
        ...s.uploads,
        [conversationId]: (s.uploads[conversationId] ?? []).filter(u => u.status !== 'ready'),
      },
    }));

    await deliver(set, get, conversationId, id, body, attachments);
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
    await deliver(set, get, conversationId, pendingId, entry.body, entry.attachments);
  },

  addFiles: (conversationId, files) => {
    const existing = get().uploads[conversationId] ?? [];
    const room = MAX_ATTACHMENTS - existing.length;
    if (room <= 0) return;

    for (const file of files.slice(0, room)) {
      const key = uuidv7();
      const isImage = file.type.startsWith('image/');

      const terlaluBesar = file.size > MAX_UPLOAD_BYTES;

      const entry: Upload = {
        key,
        name: file.name,
        size: file.size,
        mime: file.type,
        // Pratinjau dibuat dari berkas LOKAL, bukan dari alamat di server.
        // Gambarnya muncul seketika, bahkan sebelum satu byte pun terkirim.
        previewUrl: isImage ? URL.createObjectURL(file) : null,
        progress: 0,
        status: terlaluBesar ? 'failed' : 'uploading',
        attachment: null,
        error: terlaluBesar ? 'Berkas lebih dari 10 MB' : null,
        // Berkas yang ditolak karena ukuran tidak akan pernah berhasil kalau
        // dicoba lagi — ukurannya tidak berubah. Menawarkan tombol yang pasti
        // gagal lebih buruk daripada tidak menawarkan apa-apa.
        retriable: !terlaluBesar,
      };

      set(s => ({
        uploads: { ...s.uploads, [conversationId]: [...(s.uploads[conversationId] ?? []), entry] },
      }));

      if (entry.status === 'uploading') void upload(set, get, conversationId, key, file);
    }
  },

  retryUpload: (conversationId, key) => {
    const entry = (get().uploads[conversationId] ?? []).find(u => u.key === key);
    if (!entry || !files.has(key)) return;

    patchUpload(set, conversationId, key, { status: 'uploading', progress: 0, error: null });
    void upload(set, get, conversationId, key, files.get(key)!);
  },

  removeUpload: (conversationId, key) => {
    const entry = (get().uploads[conversationId] ?? []).find(u => u.key === key);
    if (entry?.previewUrl) URL.revokeObjectURL(entry.previewUrl);
    files.delete(key);

    set(s => ({
      uploads: {
        ...s.uploads,
        [conversationId]: (s.uploads[conversationId] ?? []).filter(u => u.key !== key),
      },
    }));
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

/** Memperbarui satu unggahan tanpa menyentuh yang lain. */
function patchUpload(
  set: (fn: (s: State) => Partial<State>) => void,
  conversationId: string,
  key: string,
  patch: Partial<Upload>,
) {
  set(s => ({
    uploads: {
      ...s.uploads,
      [conversationId]: (s.uploads[conversationId] ?? []).map(u =>
        u.key === key ? { ...u, ...patch } : u,
      ),
    },
  }));
}

/** Satu percobaan unggah; dipakai baik oleh pilihan pertama maupun retry. */
async function upload(
  set: (fn: (s: State) => Partial<State>) => void,
  get: () => State,
  conversationId: string,
  key: string,
  file: File,
) {
  files.set(key, file);

  try {
    const size = await imageSize(file);
    const attachment = await api.uploadAttachment(file, size, fraction => {
      // Unggahan bisa dibatalkan sementara byte-nya masih mengalir. Menulis
      // kemajuan ke entri yang sudah tidak ada akan menghidupkannya kembali.
      if ((get().uploads[conversationId] ?? []).some(u => u.key === key)) {
        patchUpload(set, conversationId, key, { progress: fraction });
      }
    });

    if (!(get().uploads[conversationId] ?? []).some(u => u.key === key)) return;
    patchUpload(set, conversationId, key, { status: 'ready', progress: 1, attachment, error: null });
  } catch (err) {
    if (!(get().uploads[conversationId] ?? []).some(u => u.key === key)) return;
    patchUpload(set, conversationId, key, {
      status: 'failed',
      error: err instanceof ApiError ? err.message : 'Unggahan gagal',
    });
  }
}

/**
 * Membaca ukuran gambar sebelum diunggah.
 *
 * Ukurannya dikirim ke server dan ikut tersimpan supaya penerima bisa memesan
 * ruang di layar sebelum gambarnya selesai dimuat. Tanpa itu, daftar pesan
 * melompat-lompat saat gambar berdatangan — dan lompatan itu terjadi tepat
 * ketika orang sedang membaca.
 */
async function imageSize(file: File): Promise<{ width: number; height: number } | null> {
  if (!file.type.startsWith('image/')) return null;
  try {
    const bitmap = await createImageBitmap(file);
    const size = { width: bitmap.width, height: bitmap.height };
    bitmap.close();
    return size;
  } catch {
    // Format yang tidak bisa dibaca browser tetap boleh dikirim; yang hilang
    // hanya kenyamanan tata letak.
    return null;
  }
}

/** Satu percobaan kirim; dipakai baik oleh kiriman pertama maupun retry. */
async function deliver(
  set: (fn: (s: State) => Partial<State>) => void,
  get: () => State,
  conversationId: string,
  id: string,
  body: string,
  attachments: Attachment[],
) {
  const attachmentIds = attachments.map(a => a.id);

  try {
    const saved = await api.sendMessage(conversationId, id, body, attachmentIds);
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
        get().applyMessage(await api.sendMessage(conversationId, id, body, attachmentIds));
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
