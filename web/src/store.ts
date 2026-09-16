import { create } from 'zustand';
import { ApiError, api } from './api';
import { uuidv7 } from './uuid';
import type {
  Conversation,
  Member,
  Message,
  PendingMessage,
  ReactionSummary,
  Upload,
  User,
} from './types';

/**
 * Sebutan yang sedang disusun untuk sebuah percakapan.
 *
 * Disimpan sebagai peta id -> nama tampilan, bukan sekadar daftar id, karena
 * saat pengiriman daftar itu harus DISARING ulang terhadap teks yang benar-
 * benar jadi dikirim. Orang yang memilih sebuah nama dari daftar lalu
 * menghapusnya lagi dari kalimatnya tidak sedang memanggil siapa-siapa, dan
 * namanya yang tersimpan diam-diam akan tetap membangunkan orangnya.
 */
type MentionDraft = { names: Record<string, string>; all: boolean };

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

  /** Pesan yang sedang dibalas, per percakapan. null = tidak sedang membalas. */
  replyTo: Record<string, Message | null>;
  /** Sebutan yang sudah dipilih dari daftar, per percakapan. */
  mentionDraft: Record<string, MentionDraft>;

  setMe: (u: User | null) => void;
  setConnected: (v: boolean) => void;

  loadConversations: () => Promise<void>;
  openConversation: (id: string) => Promise<void>;
  loadOlder: (id: string) => Promise<void>;

  sendMessage: (conversationId: string, body: string) => Promise<void>;
  setReplyTo: (conversationId: string, message: Message | null) => void;
  noteMention: (conversationId: string, userId: string, displayName: string) => void;
  setMentionAll: (conversationId: string, all: boolean) => void;
  toggleReaction: (conversationId: string, messageId: string, emoji: string) => Promise<void>;

  renameGroup: (conversationId: string, title: string) => Promise<void>;
  addMembers: (conversationId: string, userIds: string[]) => Promise<void>;
  removeMember: (conversationId: string, userId: string) => Promise<void>;
  transferOwnership: (conversationId: string, userId: string) => Promise<void>;
  leaveGroup: (conversationId: string) => Promise<void>;
  addFiles: (conversationId: string, files: File[]) => void;
  retryUpload: (conversationId: string, key: string) => void;
  removeUpload: (conversationId: string, key: string) => void;
  retryMessage: (conversationId: string, pendingId: string) => Promise<void>;
  editMessage: (id: string, body: string) => Promise<void>;
  deleteMessage: (id: string) => Promise<void>;

  applyMessage: (m: Message) => void;
  applyReaction: (
    conversationId: string,
    messageId: string,
    userId: string,
    emoji: string,
    added: boolean,
    reactionSeq: number,
  ) => void;
  applyReactionBatch: (
    conversationId: string,
    entries: { messageId: string; reactionSeq: number; reactions: ReactionSummary[] }[],
  ) => void;
  applyConversation: (c: Conversation) => void;
  applyConversationUpdated: (conversationId: string, title: string, members: Member[]) => void;
  applyConversationRemoved: (conversationId: string) => void;
  applyRead: (conversationId: string, userId: string, lastReadSeq: number) => void;
  applyTyping: (conversationId: string, userId: string, displayName: string, typing: boolean) => void;
  setOnline: (ids: string[]) => void;
  setPresence: (userId: string, online: boolean) => void;

  /** Cursor per percakapan untuk resume setelah reconnect. */
  syncCursors: () => Record<string, number>;
  /**
   * Cursor KEDUA, pada jam yang berbeda.
   *
   * Reaksi menempel pada pesan lama yang `seq`-nya sudah berhenti bergerak,
   * jadi cursor pesan tidak akan pernah menyusulkannya. Lihat
   * server/internal/store/reactions.go.
   */
  reactionCursors: () => Record<string, number>;
  markReadUpTo: (conversationId: string) => void;
  ackMention: (conversationId: string, seq: number) => void;
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
  replyTo: {},
  mentionDraft: {},

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
        replyTo: {},
        mentionDraft: {},
      };
    }),

  loadConversations: async () => {
    set({ conversations: await api.conversations() });
  },

  /**
   * Membuka percakapan: riwayat dan daftar anggota, masing-masing diperiksa
   * SENDIRI.
   *
   * Sebelumnya keduanya bergantung pada satu syarat, `!messages[id]` — dan itu
   * salah karena `messages[id]` bisa terisi tanpa riwayat pernah dimuat: satu
   * pesan yang datang lewat WebSocket ke percakapan yang BELUM dibuka sudah
   * cukup. Akibatnya daftar anggota tidak pernah diambil, dan semua yang
   * bergantung padanya diam-diam berhenti bekerja: judul grup menulis "0
   * anggota", nama pengirim jadi "Seseorang", dan sebutan tidak tersorot.
   *
   * Kegagalannya tidak terlihat di DM dua orang — di sana nama lawan bicara
   * datang dari `peer` pada daftar percakapan, bukan dari daftar anggota.
   *
   * hasMore dipakai sebagai penanda "riwayat sudah pernah dimuat" karena dia
   * hanya pernah diisi oleh pemuatan riwayat; messages[id] tidak begitu.
   */
  openConversation: async id => {
    set({ activeId: id });

    const needHistory = get().hasMore[id] === undefined;
    const needMembers = get().members[id] === undefined;

    await Promise.all([
      needHistory
        ? api.messages(id).then(page =>
            set(s => ({
              // Digabung, bukan ditimpa: pesan yang datang lewat siaran selagi
              // halaman ini diambil tidak boleh hilang hanya karena jawabannya
              // datang belakangan.
              messages: { ...s.messages, [id]: page.messages.reduce(upsert, s.messages[id] ?? []) },
              hasMore: { ...s.hasMore, [id]: page.hasMore },
            })),
          )
        : null,
      needMembers
        ? api.members(id).then(members => set(s => ({ members: { ...s.members, [id]: members } })))
        : null,
    ]);

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

    const replying = get().replyTo[conversationId] ?? null;
    const draft = get().mentionDraft[conversationId];

    // Sebutan DISARING ulang terhadap teks yang benar-benar jadi dikirim.
    //
    // Memilih sebuah nama dari daftar lalu menghapusnya lagi dari kalimat
    // adalah hal yang biasa terjadi, dan id yang tertinggal diam-diam akan
    // tetap membangunkan orangnya — sebuah panggilan yang namanya sudah tidak
    // ada di mana pun dalam pesan itu.
    const mentionedUserIds = draft
      ? Object.entries(draft.names)
          .filter(([, name]) => body.includes('@' + name))
          .map(([id]) => id)
      : [];
    const mentionsAll = Boolean(draft?.all) && body.includes('@semua');

    const id = uuidv7();
    const optimistic: PendingMessage = {
      id,
      conversationId,
      body,
      attachments,
      createdAt: new Date().toISOString(),
      status: 'sending',
      replyTo: replying
        ? {
            id: replying.id,
            seq: replying.seq,
            senderId: replying.senderId,
            body: replying.body,
            deleted: Boolean(replying.deletedAt),
          }
        : undefined,
      mentionedUserIds,
      mentionsAll,
    };
    set(s => ({
      pending: { ...s.pending, [conversationId]: [...(s.pending[conversationId] ?? []), optimistic] },
      // Kutipan dan sebutan dikosongkan bersama laci lampiran: ketiganya sudah
      // pindah ke gelembung optimistik, dan membiarkannya tetap terlihat di
      // kolom tulis membuat pesan berikutnya membalas hal yang sama tanpa
      // disengaja.
      replyTo: { ...s.replyTo, [conversationId]: null },
      mentionDraft: { ...s.mentionDraft, [conversationId]: { names: {}, all: false } },
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

    await deliver(set, get, conversationId, optimistic);
  },

  setReplyTo: (conversationId, message) =>
    set(s => ({ replyTo: { ...s.replyTo, [conversationId]: message } })),

  noteMention: (conversationId, userId, displayName) =>
    set(s => {
      const current = s.mentionDraft[conversationId] ?? { names: {}, all: false };
      return {
        mentionDraft: {
          ...s.mentionDraft,
          [conversationId]: { ...current, names: { ...current.names, [userId]: displayName } },
        },
      };
    }),

  setMentionAll: (conversationId, all) =>
    set(s => {
      const current = s.mentionDraft[conversationId] ?? { names: {}, all: false };
      return { mentionDraft: { ...s.mentionDraft, [conversationId]: { ...current, all } } };
    }),

  retryMessage: async (conversationId, pendingId) => {
    const entry = (get().pending[conversationId] ?? []).find(p => p.id === pendingId);
    if (!entry) return;

    set(s => ({
      pending: {
        ...s.pending,
        [conversationId]: (s.pending[conversationId] ?? []).map(p =>
          p.id === pendingId ? { ...p, status: 'sending' as const, error: undefined } : p,
        ),
      },
    }));
    await deliver(set, get, conversationId, entry);
  },

  /**
   * Menekan atau melepas satu emoji.
   *
   * Diterapkan optimistik lebih dulu, lalu dikonfirmasi lewat jam reaksi. Yang
   * membuat itu aman adalah applyReaction di bawah: dia tahu membedakan
   * "perubahanku yang baru kembali dari server" dari "perubahan orang lain",
   * jadi satu penekanan tidak pernah terhitung dua kali walau kabarnya datang
   * lewat dua jalan sekaligus.
   */
  toggleReaction: async (conversationId, messageId, emoji) => {
    const me = get().me;
    if (!me) return;

    const message = (get().messages[conversationId] ?? []).find(m => m.id === messageId);
    if (!message) return;

    const mine = message.reactions.some(r => r.emoji === emoji && r.mine);
    get().applyReaction(conversationId, messageId, me.id, emoji, !mine, 0);

    try {
      if (mine) await api.removeReaction(messageId, emoji);
      else await api.addReaction(messageId, emoji);
    } catch {
      // Dikembalikan ke keadaan semula. Reaksi yang gagal tidak layak diberi
      // pesan kesalahan sendiri — orangnya akan menekan lagi kalau memang
      // masih mau, dan tombol yang kembali ke posisi awal sudah mengatakan
      // semuanya.
      get().applyReaction(conversationId, messageId, me.id, emoji, mine, 0);
    }
  },

  /**
   * Kelima tindakan pengelolaan grup.
   *
   * Semuanya menerapkan jawabannya sendiri ke state lewat jalur yang SAMA
   * dengan yang dipakai siaran ke anggota lain (applyConversationUpdated).
   * Server memang menyiarkannya ke kita juga, tapi menunggu siaran itu berarti
   * layar orang yang menekan tombolnya sendiri adalah yang paling terakhir
   * berubah — dan applyConversationUpdated memang menimpa, jadi kabar yang
   * datang belakangan tidak merusak apa pun.
   */
  renameGroup: async (conversationId, title) => {
    const got = await api.renameGroup(conversationId, title);
    get().applyConversationUpdated(conversationId, got.title, got.members);
  },

  addMembers: async (conversationId, userIds) => {
    const got = await api.addMembers(conversationId, userIds);
    get().applyConversationUpdated(conversationId, got.title, got.members);
  },

  removeMember: async (conversationId, userId) => {
    const got = await api.removeMember(conversationId, userId);
    get().applyConversationUpdated(conversationId, got.title, got.members);
  },

  transferOwnership: async (conversationId, userId) => {
    const got = await api.transferOwnership(conversationId, userId);
    get().applyConversationUpdated(conversationId, got.title, got.members);
  },

  leaveGroup: async conversationId => {
    await api.leaveGroup(conversationId);
    // Siaran "kamu bukan anggota lagi" memang menyusul dari server, tapi
    // percakapan yang baru saja kita tinggalkan tidak boleh masih terbuka di
    // layar selama perjalanan itu.
    get().applyConversationRemoved(conversationId);
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

      // Penanda sebutan dinaikkan di sini juga, bukan hanya di database.
      //
      // Server sudah menuliskannya saat pesannya masuk, tapi client tidak
      // memuat ulang daftar percakapan setiap ada pesan baru — dan tanpa baris
      // ini, panggilan baru terlihat setelah halaman di-refresh. Sengaja TIDAK
      // dibatalkan oleh "percakapan ini sedang terbuka": itu tugas ackMention,
      // yang baru berjalan setelah pesannya benar-benar terlihat.
      const callsMe =
        m.senderId !== s.me?.id &&
        !m.deletedAt &&
        (m.mentionsAll || (s.me ? m.mentions.includes(s.me.id) : false));

      const conversations = s.conversations.map(c =>
        c.id === m.conversationId
          ? {
              ...c,
              lastMessage: m,
              lastSeq: Math.max(c.lastSeq, m.seq),
              updatedAt: m.createdAt,
              mentionSeq: callsMe ? Math.max(c.mentionSeq, m.seq) : c.mentionSeq,
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

  applyReaction: (conversationId, messageId, userId, emoji, added, reactionSeq) =>
    set(s => {
      const list = s.messages[conversationId] ?? [];
      const idx = list.findIndex(m => m.id === messageId);
      if (idx < 0) return s;

      const message = list[idx]!;

      // Gerbang pertama: jam yang tidak maju berarti kabar ini sudah pernah
      // terpasang. Satu perubahan sampai ke sini lewat DUA jalan — jawaban
      // HTTP atas penekanan kita sendiri, dan siaran WebSocket ke semua
      // anggota — dan tanpa gerbang ini hitungannya naik dua.
      if (reactionSeq > 0 && reactionSeq <= message.reactionSeq) return s;

      // Gerbang kedua: perubahan kita sendiri yang sudah kita pasang
      // optimistik. Yang datang cuma nomor jamnya; keadaannya sudah benar.
      const mine = message.reactions.some(r => r.emoji === emoji && r.mine);
      if (userId === s.me?.id && added === mine) {
        if (reactionSeq <= message.reactionSeq) return s;
        const stamped = list.slice();
        stamped[idx] = { ...message, reactionSeq };
        return { messages: { ...s.messages, [conversationId]: stamped } };
      }

      const reactions = applyDelta(message.reactions, emoji, added, userId === s.me?.id);
      const next = list.slice();
      next[idx] = {
        ...message,
        reactions,
        reactionSeq: Math.max(message.reactionSeq, reactionSeq),
      };
      return { messages: { ...s.messages, [conversationId]: next } };
    }),

  applyReactionBatch: (conversationId, entries) =>
    set(s => {
      const list = s.messages[conversationId] ?? [];
      if (list.length === 0 || entries.length === 0) return s;

      const byId = new Map(entries.map(e => [e.messageId, e]));
      let changed = false;

      const next = list.map(m => {
        const got = byId.get(m.id);
        // Susulan membawa KEADAAN, bukan selisih — jadi dia menimpa, bukan
        // menambah. Itu yang membuat pencabutan yang terjadi selagi kita
        // offline ikut terlihat: daftarnya datang kosong, dan kosong adalah
        // jawaban yang benar.
        if (!got || got.reactionSeq <= m.reactionSeq) return m;
        changed = true;
        return { ...m, reactions: got.reactions, reactionSeq: got.reactionSeq };
      });

      return changed ? { messages: { ...s.messages, [conversationId]: next } } : s;
    }),

  applyConversation: c =>
    set(s => (s.conversations.some(x => x.id === c.id) ? s : { conversations: [c, ...s.conversations] })),

  applyConversationUpdated: (conversationId, title, members) =>
    set(s => ({
      conversations: s.conversations.map(c =>
        c.id === conversationId ? { ...c, title } : c,
      ),
      // Ditimpa, bukan digabung: yang datang adalah KEADAAN setelah perubahan,
      // dan orang yang baru saja dikeluarkan memang harus hilang dari daftar.
      members: { ...s.members, [conversationId]: members },
    })),

  applyConversationRemoved: conversationId =>
    set(s => {
      // objectURL pratinjau dilepas lebih dulu. Dia menahan berkasnya di memori
      // sampai dilepas, dan membuang entrinya saja meninggalkan gambar yang
      // tidak lagi punya siapa pun yang bisa melepaskannya — kebocoran yang
      // sama persis dengan yang ditutup di reset().
      for (const u of s.uploads[conversationId] ?? []) {
        if (u.previewUrl) URL.revokeObjectURL(u.previewUrl);
        files.delete(u.key);
      }

      // Semua yang menempel pada percakapan ini ikut dibuang. Menyisakan
      // riwayatnya di memori berarti percakapan yang sudah bukan milik kita
      // tetap bisa muncul kembali begitu ada satu event yang menyebut id-nya.
      const drop = <T,>(rec: Record<string, T>) => {
        const next = { ...rec };
        delete next[conversationId];
        return next;
      };
      return {
        conversations: s.conversations.filter(c => c.id !== conversationId),
        activeId: s.activeId === conversationId ? null : s.activeId,
        messages: drop(s.messages),
        pending: drop(s.pending),
        uploads: drop(s.uploads),
        hasMore: drop(s.hasMore),
        members: drop(s.members),
        typing: drop(s.typing),
        replyTo: drop(s.replyTo),
        mentionDraft: drop(s.mentionDraft),
      };
    }),

  applyRead: (conversationId, userId, lastReadSeq) =>
    set(s => {
      const roster = s.members[conversationId];
      return {
        // Daftar anggota hanya DIPERBARUI di sini, tidak pernah dibuat.
        //
        // Sebelumnya baris ini berbunyi `(s.members[id] ?? []).map(...)` dan
        // menulis hasilnya kembali — sehingga sebuah read receipt yang datang
        // untuk percakapan yang belum pernah dibuka menanam larik KOSONG di
        // sana. Larik kosong itu tidak bisa dibedakan dari "sudah dimuat dan
        // memang kosong", jadi daftar anggotanya tidak pernah diambil lagi:
        // judul grup menulis "0 anggota", nama pengirim jadi "Seseorang", dan
        // sebutan berhenti tersorot. Read receipt datang jauh lebih sering
        // daripada orang membuka percakapan, jadi keadaan itu nyaris permanen.
        members: roster
          ? {
              ...s.members,
              [conversationId]: roster.map(m =>
                m.userId === userId ? { ...m, lastReadSeq } : m,
              ),
            }
          : s.members,
        conversations: s.conversations.map(c =>
          c.id === conversationId && userId === s.me?.id
            ? { ...c, lastReadSeq, unread: Math.max(0, c.lastSeq - lastReadSeq) }
            : c,
        ),
      };
    }),

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

  reactionCursors: () => {
    const cursors: Record<string, number> = {};
    for (const [id, list] of Object.entries(get().messages)) {
      let max = 0;
      for (const m of list) if (m.reactionSeq > max) max = m.reactionSeq;
      cursors[id] = max;
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

  /**
   * Menurunkan penanda "ada yang menyebut kamu" — dipanggil hanya setelah
   * pesan yang memanggil namanya BENAR-BENAR terlihat di layar, bukan saat
   * percakapannya dibuka.
   */
  ackMention: (conversationId, seq) => {
    const conv = get().conversations.find(c => c.id === conversationId);
    if (!conv || conv.mentionAckSeq >= seq) return;

    set(s => ({
      conversations: s.conversations.map(c =>
        c.id === conversationId ? { ...c, mentionAckSeq: Math.max(c.mentionAckSeq, seq) } : c,
      ),
    }));
    void api.ackMentions(conversationId, seq).catch(() => {});
  },
}));

/**
 * Menerapkan satu selisih pada ringkasan reaksi sebuah pesan.
 *
 * Urutan emoji dijaga: yang pertama muncul tetap di depan, supaya tombolnya
 * tidak berpindah tempat di bawah jari orang yang sedang menekannya.
 */
function applyDelta(
  reactions: ReactionSummary[],
  emoji: string,
  added: boolean,
  isMine: boolean,
): ReactionSummary[] {
  const idx = reactions.findIndex(r => r.emoji === emoji);

  if (!added) {
    if (idx < 0) return reactions;
    const current = reactions[idx]!;
    if (current.count <= 1) return reactions.filter((_, i) => i !== idx);
    const next = reactions.slice();
    next[idx] = {
      ...current,
      count: current.count - 1,
      mine: isMine ? false : current.mine,
    };
    return next;
  }

  if (idx < 0) return [...reactions, { emoji, count: 1, mine: isMine }];
  const current = reactions[idx]!;
  const next = reactions.slice();
  next[idx] = { ...current, count: current.count + 1, mine: current.mine || isMine };
  return next;
}

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
  entry: PendingMessage,
) {
  const { id, body } = entry;
  const extras = {
    attachmentIds: entry.attachments.map(a => a.id),
    replyToId: entry.replyTo?.id,
    mentionedUserIds: entry.mentionedUserIds,
    mentionsAll: entry.mentionsAll,
  };

  try {
    get().applyMessage(await api.sendMessage(conversationId, id, body, extras));
  } catch (err) {
    // Kena kuota: tunggu selama yang diminta server, lalu coba sekali lagi
    // diam-diam. Aman justru karena `id` dibuat di client — pengiriman ulang
    // menghasilkan pesan yang sama, bukan pesan kedua. Menampilkan "gagal" di
    // sini akan menyuruh orang menekan tombol kirim lagi, dan tekanan tepat
    // saat server sedang minta pelan-pelan adalah kebalikan dari yang berguna.
    //
    // Kecuali untuk @semua: kuotanya memang dihitung dalam menit, bukan detik,
    // dan menunggunya diam-diam berarti layar yang membeku sepuluh detik lalu
    // gagal juga. Yang itu langsung dilaporkan beserta sebabnya.
    if (err instanceof ApiError && err.status === 429 && !extras.mentionsAll) {
      await new Promise(r => setTimeout(r, Math.min(err.retryAfterMs || 1000, 10_000)));
      try {
        get().applyMessage(await api.sendMessage(conversationId, id, body, extras));
        return;
      } catch {
        // Jatuh ke penandaan gagal di bawah.
      }
    }
    const reason = err instanceof ApiError ? err.message : undefined;
    set(s => ({
      pending: {
        ...s.pending,
        [conversationId]: (s.pending[conversationId] ?? []).map(p =>
          p.id === id ? { ...p, status: 'failed' as const, error: reason } : p,
        ),
      },
    }));
  }
}
