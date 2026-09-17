import { create } from 'zustand';
import { ApiError, api } from './api';
import { uuidv7 } from './uuid';
import type {
  Conversation,
  Me,
  Member,
  Message,
  PendingMessage,
  Pin,
  ReactionSummary,
  ServerConfig,
  StatusKind,
  Upload,
  User,
  UserStatus,
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
 * Hapus yang sedang ditahan.
 *
 * `due` adalah waktu (ms epoch) permintaannya dikirim. Selama dijeda — kabarnya
 * sedang disentuh kursor atau fokus papan ketik — `remaining` menyimpan sisa
 * waktunya dan pengatur waktunya berhenti.
 */
export type PendingDelete = {
  conversationId: string;
  due: number;
  remaining?: number;
  error?: string;
};

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

/**
 * Berapa pesan di belakang pesan tertua yang dimuat masih dianggap "dekat".
 *
 * Yang sedekat ini disusul dengan menarik halaman lama satu per satu, dan
 * riwayatnya tetap utuh sampai ujung terbaru. Yang lebih jauh dimuat sebagai
 * JENDELA tersendiri — menarik puluhan halaman untuk menampilkan satu pesan
 * tahun lalu adalah lompatan yang tidak pernah sampai.
 */
const JUMP_NEAR = 150;

/** Batas pilihan saat meneruskan — sama dengan kelonggaran kuota terusan di server. */
export const MAX_FORWARD_TARGETS = 5;

/** Tujuan lompatan terakhir; `nonce` membuat lompatan ke pesan yang sama terbaca sebagai lompatan baru. */
type JumpTarget = { conversationId: string; messageId: string; nonce: number };

/** Hasil meneruskan ke beberapa percakapan sekaligus, per tujuan. */
export type ForwardOutcome = { conversationId: string; ok: boolean; error?: string };

type State = {
  me: Me | null;
  connected: boolean;

  /**
   * Apa yang bisa dilakukan server ini. null = belum sempat ditanyakan.
   *
   * Dipakai untuk TIDAK MENAMPILKAN tombol yang pasti ditolak — aturan yang
   * sama dengan panel kelola grup. Selama masih null, yang bisa dimatikan
   * dianggap MATI: menampilkan tombol lalu menariknya kembali sepersekian detik
   * kemudian lebih buruk daripada menampilkannya sedikit terlambat.
   */
  config: ServerConfig | null;

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
  /**
   * true = yang sedang dimuat adalah JENDELA di tengah riwayat, bukan riwayat
   * yang menyentuh ujung terbaru. Lihat jumpTo.
   *
   * Selama ini true, pesan baru yang datang lewat siaran TIDAK ditempelkan ke
   * daftar: menempelkannya di bawah jendela berarti dua potong riwayat dengan
   * lubang tak terlihat di antaranya. Mereka ditampung di `stash` dan
   * digabungkan begitu jendelanya menyambung kembali ke ujung.
   */
  hasNewer: Record<string, boolean>;
  members: Record<string, Member[]>;
  /** Pesan yang disematkan, per percakapan. undefined = belum pernah dimuat. */
  pins: Record<string, Pin[]>;
  /** Pesan yang baru saja dituju lompatan; ChatPanel yang menggulir ke sana. */
  jumpTarget: JumpTarget | null;

  online: Set<string>;
  /**
   * Status orang lain, per id. Dua keadaan yang sengaja dipisah dari `online`
   * di atas: presence diturunkan dari koneksi yang hidup dan hilang begitu
   * tabnya ditutup; status adalah pernyataan yang dibuat orang dan bertahan
   * melewati logout. Titik di sidebar menampilkan gabungan keduanya.
   *
   * Hanya memuat yang BUKAN bawaan. Yang tidak ada di sini adalah 'available'
   * tanpa teks, dan server memang tidak pernah mengirimkannya.
   */
  statuses: Record<string, UserStatus>;
  typing: Record<string, Record<string, TypingEntry>>;

  /** Pesan yang sedang dibalas, per percakapan. null = tidak sedang membalas. */
  replyTo: Record<string, Message | null>;
  /** Sebutan yang sudah dipilih dari daftar, per percakapan. */
  mentionDraft: Record<string, MentionDraft>;
  /**
   * Pesan yang sedang menunggu dihapus, per id.
   *
   * Hapus berlaku untuk semua orang dan tidak bisa ditarik kembali di server,
   * jadi permintaannya ditahan beberapa detik di sini. Selama ditahan,
   * gelembungnya sudah tampil sebagai "dihapus" dan kabar "Urungkan" ada di
   * layar. `error` terisi bila server menolak saat waktunya tiba.
   */
  deleting: Record<string, PendingDelete>;

  setMe: (u: Me | null) => void;
  setConnected: (v: boolean) => void;
  loadConfig: () => Promise<void>;

  updateProfile: (displayName: string) => Promise<void>;
  setStatus: (status: StatusKind, text: string, expiresAt: string | null) => Promise<void>;
  uploadAvatar: (file: File, onProgress: (fraction: number) => void) => Promise<void>;
  removeAvatar: () => Promise<void>;

  loadConversations: () => Promise<void>;
  openConversation: (id: string) => Promise<void>;
  /** Melepas percakapan yang sedang dibuka; di layar sempit inilah "kembali". */
  closeConversation: () => void;
  loadOlder: (id: string) => Promise<void>;
  /** Memuat lanjutan jendela ke arah yang lebih baru. */
  loadNewer: (id: string) => Promise<void>;
  /** Meninggalkan jendela dan kembali ke pesan terbaru. */
  returnToLatest: (id: string) => Promise<void>;
  /**
   * Melompat ke sebuah pesan, memuatnya dulu bila perlu — termasuk membuka
   * percakapannya. `seq` wajib untuk pesan yang belum termuat.
   */
  jumpTo: (conversationId: string, messageId: string, seq?: number) => Promise<boolean>;

  loadPins: (conversationId: string) => Promise<void>;
  setPinned: (message: Message, pinned: boolean) => Promise<void>;
  forwardMessage: (messageId: string, targets: string[]) => Promise<ForwardOutcome[]>;

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
  /**
   * Mengembalikan kunci tiap unggahan yang diterima laci. `durationMs` hanya
   * untuk rekaman suara, dan hanya bila berkasnya satu.
   */
  addFiles: (conversationId: string, files: File[], durationMs?: number) => string[];
  retryUpload: (conversationId: string, key: string) => void;
  removeUpload: (conversationId: string, key: string) => void;
  retryMessage: (conversationId: string, pendingId: string) => Promise<void>;
  editMessage: (id: string, body: string) => Promise<void>;
  deleteMessage: (id: string) => Promise<void>;
  /** Menjadwalkan hapus; baru dikirim ke server setelah DELETE_GRACE_MS. */
  scheduleDelete: (message: Message) => void;
  undoDelete: (id: string) => void;
  /** Mengurungkan semua hapus yang masih menunggu, di semua percakapan. */
  undoDeletes: () => void;
  /** Menghentikan hitung mundur semua hapus yang menunggu (WCAG 2.2.1). */
  pauseDeletes: () => void;
  resumeDeletes: () => void;
  /** Mengirim ulang hapus yang ditolak server, tanpa jeda. */
  retryDelete: (id: string) => void;
  /** Menutup kabar hapus yang gagal. */
  dismissDelete: (id: string) => void;
  /** Membuang pesan yang tidak jadi dikirim dari antrean. */
  discardPending: (conversationId: string, pendingId: string) => void;

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
  applyStatus: (status: UserStatus) => void;
  applyStatusSnapshot: (statuses: UserStatus[]) => void;
  applyUserUpdated: (user: User) => void;

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

/**
 * Pesan yang datang selagi percakapannya sedang menampilkan jendela lama.
 *
 * Di luar state dengan alasan yang sama dengan `files`: tidak ada yang
 * menampilkannya, dan menyimpannya di state berarti setiap pesan baru memicu
 * render ulang untuk sesuatu yang tidak terlihat.
 */
const stash = new Map<string, Message[]>();

/**
 * Percakapan yang jendelanya SEDANG diminta. Penampungan sudah harus berlaku
 * sejak permintaan berangkat: pesan yang datang di antaranya akan ditempel ke
 * riwayat lama, lalu hilang tertimpa jendela begitu jawabannya tiba.
 */
const windowing = new Set<string>();

/**
 * Jeda sebelum hapus benar-benar dikirim — waktu untuk "Urungkan".
 *
 * Delapan detik, bukan lima: penggunanya lintas umur, dan waktu yang cukup
 * untuk membaca kabarnya, memahaminya, lalu menemukan tombolnya diukur dari
 * pembaca yang paling lambat. Hitung mundurnya tampil di kabar itu sendiri.
 */
export const DELETE_GRACE_MS = 8000;

/** Pengatur waktu hapus yang sedang menunggu, per id pesan. */
const deleteTimers = new Map<string, ReturnType<typeof setTimeout>>();

/**
 * Hapus yang sedang ditahan tetap dikirim saat halaman ditutup.
 *
 * Kabarnya sudah mengatakan "menghapus"; menutup tab dalam jeda itu tidak
 * boleh diam-diam membatalkannya. Yang ingin membatalkan punya tombolnya.
 */
if (typeof window !== 'undefined') {
  window.addEventListener('pagehide', () => {
    // Yang dijeda juga: jeda hanya menunda, bukan membatalkan.
    for (const [id, d] of Object.entries(useStore.getState().deleting)) {
      if (!d.error) api.deleteMessageOnExit(id);
    }
    for (const t of deleteTimers.values()) clearTimeout(t);
    deleteTimers.clear();
  });
}

/**
 * Antrean kirim yang disimpan di perangkat.
 *
 * Pesan yang sudah ditulis tapi belum sampai ke server adalah pesan yang
 * hilang kalau halamannya dimuat ulang — dan "pesan tidak boleh hilang" adalah
 * janji pertama aplikasi ini. Disimpan per akun, di localStorage: kecil, dan
 * cukup untuk beberapa kalimat yang tertahan jaringan.
 *
 * Yang dipulihkan selalu berstatus gagal. Pengiriman yang sedang berjalan saat
 * halaman ditutup tidak diketahui nasibnya, dan "gagal" adalah keadaan yang
 * punya tombol; `retryStranded` yang mengirimnya ulang begitu tersambung.
 */
const kunciAntrean = (userId: string) => `antrean:${userId}`;

function muatAntrean(userId: string): Record<string, PendingMessage[]> {
  try {
    const raw = localStorage.getItem(kunciAntrean(userId));
    if (!raw) return {};
    const parsed = JSON.parse(raw) as Record<string, PendingMessage[]>;
    const out: Record<string, PendingMessage[]> = {};
    for (const [cid, list] of Object.entries(parsed)) {
      if (!Array.isArray(list) || list.length === 0) continue;
      out[cid] = list.map(p => ({ ...p, status: 'failed' as const, error: p.error }));
    }
    return out;
  } catch {
    return {};
  }
}

function simpanAntrean(userId: string, pending: Record<string, PendingMessage[]>) {
  try {
    const isi = Object.fromEntries(Object.entries(pending).filter(([, l]) => l.length > 0));
    if (Object.keys(isi).length === 0) localStorage.removeItem(kunciAntrean(userId));
    else localStorage.setItem(kunciAntrean(userId), JSON.stringify(isi));
  } catch {
    // Penyimpanan penuh atau diblokir: antreannya tetap hidup selama tab ini
    // terbuka, yang hilang hanya ketahanannya terhadap muat ulang.
  }
}

function hapusAntrean(userId: string) {
  try {
    localStorage.removeItem(kunciAntrean(userId));
  } catch {
    // Tidak ada yang bisa dilakukan; lihat simpanAntrean.
  }
}

/** Menggabungkan antrean pulihan tanpa menggandakan yang sudah ada. */
function mergeAntrean(
  current: Record<string, PendingMessage[]>,
  restored: Record<string, PendingMessage[]>,
): Record<string, PendingMessage[]> {
  const out = { ...current };
  for (const [cid, list] of Object.entries(restored)) {
    const have = new Set((out[cid] ?? []).map(p => p.id));
    out[cid] = [...list.filter(p => !have.has(p.id)), ...(out[cid] ?? [])];
  }
  return out;
}

/**
 * Mengirim ulang pesan yang gagal TANPA sebab dari server — jaringan putus,
 * atau dipulihkan dari muat ulang. Yang ditolak server dengan alasan jelas
 * (kuota, lampiran tidak sah) tidak disentuh: mengulanginya diam-diam hanya
 * menghasilkan penolakan yang sama.
 */
function retryStranded(get: () => State) {
  const { pending, retryMessage } = get();
  for (const [cid, list] of Object.entries(pending)) {
    for (const p of list) {
      if (p.status === 'failed' && !p.error) void retryMessage(cid, p.id);
    }
  }
}

/** Menggabungkan tampungan ke daftar, lalu mengosongkannya. */
function drainStash(id: string, list: Message[]): Message[] {
  const held = stash.get(id) ?? [];
  stash.delete(id);
  return held.reduce(upsert, list);
}

const isPinNotice = (m: Message) =>
  m.kind === 'system' &&
  (m.systemEvent?.type === 'message.pinned' || m.systemEvent?.type === 'message.unpinned');

/** Menyisipkan atau memperbarui pesan sambil menjaga urutan seq. */
function upsert(list: Message[], m: Message): Message[] {
  const idx = list.findIndex(x => x.id === m.id);
  if (idx >= 0) {
    const next = list.slice();
    next[idx] = keepReactions(list[idx]!, m);
    return next;
  }
  const next = list.slice();
  let i = next.length;
  while (i > 0 && next[i - 1]!.seq > m.seq) i--;
  next.splice(i, 0, m);
  return next;
}

/**
 * Pesan yang sama datang lagi — hasil edit, atau kiriman ulang dari server.
 *
 * Siaran edit adalah SATU payload untuk semua orang, jadi dia tidak pernah
 * membawa ringkasan reaksi (yang memuat "apakah aku ikut"): larik reaksinya
 * selalu kosong. Menimpanya apa adanya membuat setiap pesan yang disunting
 * kehilangan semua reaksinya di layar sampai halamannya dimuat ulang. Yang
 * dihapus memang kehilangan reaksinya — server membuangnya bersama isinya.
 */
function keepReactions(old: Message, m: Message): Message {
  if (m.deletedAt || m.reactions.length > 0 || old.reactions.length === 0) return m;
  return { ...m, reactions: old.reactions, reactionSeq: Math.max(old.reactionSeq, m.reactionSeq) };
}

export const useStore = create<State>((set, get) => ({
  me: null,
  connected: false,
  config: null,
  conversations: [],
  activeId: null,
  messages: {},
  pending: {},
  uploads: {},
  hasMore: {},
  hasNewer: {},
  members: {},
  pins: {},
  jumpTarget: null,
  online: new Set(),
  statuses: {},
  typing: {},
  replyTo: {},
  mentionDraft: {},
  deleting: {},

  setMe: me => {
    const before = get().me?.id;
    set({ me });
    // Antrean yang tertinggal dari kunjungan sebelumnya dipulihkan sekali,
    // saat orangnya dikenali — bukan saat modul dimuat, karena sebelum itu
    // belum jelas antrean SIAPA yang boleh ditampilkan di perangkat ini.
    if (me && me.id !== before) {
      const restored = muatAntrean(me.id);
      if (Object.keys(restored).length > 0) {
        set(s => ({ pending: mergeAntrean(s.pending, restored) }));
        if (get().connected) retryStranded(get);
      }
    }
  },
  setConnected: connected => {
    set({ connected });
    // Tersambung lagi: pesan yang gagal karena jaringan dicoba ulang sendiri.
    // Itu janji bilah "menyambungkan ulang" di ChatPanel, dan aman karena id
    // pesan dibuat di sini — server mengembalikan pesan yang sama, bukan
    // pesan kedua.
    if (connected) retryStranded(get);
  },

  /**
   * Kegagalannya diam. Server yang tidak menjawab pertanyaan ini adalah server
   * yang juga tidak akan menjawab yang lain, dan pesan kesalahan tentang
   * "konfigurasi" di layar orang yang baru membuka aplikasi tidak memberi tahu
   * apa pun yang bisa dia lakukan. Yang tersisa: config tetap null, dan yang
   * bisa dimatikan dianggap mati.
   */
  loadConfig: async () => {
    try {
      set({ config: await api.serverConfig() });
    } catch {
      set({ config: null });
    }
  },

  reset: () =>
    set(s => {
      // Antrean yang belum terkirim ikut dibuang saat keluar. Komputer yang
      // dipakai bergantian tidak boleh menyimpan kalimat orang sebelumnya
      // untuk ditampilkan — apalagi dikirim — atas nama orang berikutnya.
      if (s.me) hapusAntrean(s.me.id);
      for (const t of deleteTimers.values()) clearTimeout(t);
      deleteTimers.clear();
      // objectURL menahan berkasnya di memori sampai dilepas. Logout tanpa ini
      // menyisakan setiap gambar yang pernah dipilih selama sesi itu.
      for (const list of Object.values(s.uploads)) {
        for (const u of list) if (u.previewUrl) URL.revokeObjectURL(u.previewUrl);
      }
      files.clear();
      stash.clear();
      // `config` sengaja tidak ikut dibuang: dia menggambarkan SERVER-nya,
      // bukan orang yang barusan keluar. Membuangnya berarti halaman masuk
      // kehilangan jawaban "apakah pemulihan password tersedia" tepat setelah
      // seseorang logout — dan menanyakannya lagi untuk jawaban yang sama.
      return {
        me: null,
        conversations: [],
        activeId: null,
        messages: {},
        pending: {},
        uploads: {},
        hasMore: {},
        hasNewer: {},
        members: {},
        pins: {},
        jumpTarget: null,
        online: new Set(),
        statuses: {},
        typing: {},
        replyTo: {},
        mentionDraft: {},
        deleting: {},
      };
    }),

  /**
   * Keempat tindakan akun menerapkan jawabannya sendiri ke `me`.
   *
   * Server memang menyiarkan `user.updated` ke kita juga — kita termasuk
   * penerimanya justru supaya tab lain ikut berubah — tapi menunggu siaran itu
   * berarti layar orang yang menekan tombolnya sendiri adalah yang paling
   * terakhir berubah. Jalur yang sama dengan pengelolaan grup di Fase 9b.
   */
  updateProfile: async displayName => {
    set({ me: await api.updateProfile(displayName) });
  },

  setStatus: async (status, text, expiresAt) => {
    const got = await api.setStatus(status, text, expiresAt);
    set(s =>
      s.me
        ? {
            me: {
              ...s.me,
              status: got.status,
              statusText: got.text,
              statusExpiresAt: got.expiresAt,
            },
          }
        : s,
    );
  },

  uploadAvatar: async (file, onProgress) => {
    set({ me: await api.uploadAvatar(file, onProgress) });
  },

  removeAvatar: async () => {
    set({ me: await api.removeAvatar() });
  },

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

    // Sematan dibaca ulang SETIAP kali percakapan dibuka, bukan sekali saja.
    // Daftarnya pendek dan kuerinya murah, dan ini satu-satunya cara yang
    // pasti menangkap sematan yang ikut lepas karena pesannya dihapus selagi
    // kita offline — penghapusan itu tidak meninggalkan catatan sistem.
    void get().loadPins(id);

    get().markReadUpTo(id);
  },

  /**
   * Hanya melepas penunjuknya — riwayat, anggota, dan draf unggahan dibiarkan
   * utuh di tempatnya.
   *
   * Di layar lebar tombolnya memang tidak ada: daftar dan percakapan tampil
   * bersebelahan, dan "menutup" akan menyisakan panel kosong tanpa alasan. Yang
   * membutuhkannya adalah layar sempit, tempat keduanya bergantian mengisi satu
   * kolom yang sama.
   */
  closeConversation: () => set({ activeId: null }),

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

  loadNewer: async id => {
    const list = get().messages[id] ?? [];
    const newest = list[list.length - 1]?.seq;
    if (!newest || !get().hasNewer[id]) return;

    const page = await api.messagesAfter(id, newest);
    set(s => {
      const merged = page.messages.reduce(upsert, s.messages[id] ?? []);
      return {
        // Jendela yang baru saja menyambung ke ujung menerima tampungannya:
        // pesan yang datang selagi jendelanya belum sampai ke sana.
        messages: { ...s.messages, [id]: page.hasNewer ? merged : drainStash(id, merged) },
        hasNewer: { ...s.hasNewer, [id]: page.hasNewer },
      };
    });
    if (!page.hasNewer) get().markReadUpTo(id);
  },

  returnToLatest: async id => {
    const page = await api.messages(id);
    set(s => ({
      messages: { ...s.messages, [id]: drainStash(id, page.messages) },
      hasMore: { ...s.hasMore, [id]: page.hasMore },
      hasNewer: { ...s.hasNewer, [id]: false },
    }));
    get().markReadUpTo(id);
  },

  /**
   * Melompat ke sebuah pesan.
   *
   * Tiga kemungkinan, dari yang termurah: pesannya sudah termuat; pesannya
   * dekat di belakang, jadi halaman lama ditarik satu per satu dan riwayatnya
   * tetap utuh; atau pesannya jauh, dan yang dimuat adalah JENDELA di
   * sekitarnya. Yang terakhir menggantikan riwayat yang sedang tampil — dan
   * `hasNewer` yang mengingat bahwa ujung terbarunya belum termuat.
   *
   * `seq` yang membuat jendela bisa diminta. Kutipan, sematan, catatan sistem,
   * dan hasil pencarian semuanya membawanya, justru untuk keperluan ini.
   */
  jumpTo: async (conversationId, messageId, seq) => {
    if (get().activeId !== conversationId) await get().openConversation(conversationId);

    const loaded = () => (get().messages[conversationId] ?? []).some(m => m.id === messageId);

    if (!loaded() && get().hasMore[conversationId] !== false) {
      const oldest = get().messages[conversationId]?.[0]?.seq;
      const near = seq !== undefined && oldest !== undefined && oldest - seq <= JUMP_NEAR;
      if (near || seq === undefined) {
        for (let page = 0; page < 4 && !loaded() && get().hasMore[conversationId]; page++) {
          await get().loadOlder(conversationId);
        }
      }
    }

    if (!loaded() && seq !== undefined) {
      windowing.add(conversationId);
      try {
        const win = await api.messagesAround(conversationId, seq);
        set(s => ({
          messages: {
            ...s.messages,
            [conversationId]: win.hasNewer
              ? win.messages
              : drainStash(conversationId, win.messages),
          },
          hasMore: { ...s.hasMore, [conversationId]: win.hasMore },
          hasNewer: { ...s.hasNewer, [conversationId]: win.hasNewer },
        }));
      } finally {
        windowing.delete(conversationId);
      }
    }

    if (!loaded()) return false;
    set({ jumpTarget: { conversationId, messageId, nonce: Date.now() } });
    return true;
  },

  loadPins: async conversationId => {
    try {
      const pins = await api.pins(conversationId);
      set(s => ({ pins: { ...s.pins, [conversationId]: pins } }));
    } catch {
      // Daftar sematan yang gagal dimuat tidak layak jadi pesan kesalahan:
      // percakapannya tetap bisa dipakai sepenuhnya tanpa dia.
    }
  },

  /**
   * Menyematkan atau melepas.
   *
   * Tidak optimistik. Sematan adalah tindakan yang dilihat SEMUA anggota dan
   * meninggalkan catatan di riwayat mereka; menampilkannya sebelum server
   * setuju berarti menampilkan sesuatu yang mungkin ditolak karena batas
   * jumlahnya.
   */
  setPinned: async (message, pinned) => {
    const res = pinned ? await api.pin(message.id) : await api.unpin(message.id);
    if (res.notice) get().applyMessage(res.notice);
    await get().loadPins(message.conversationId);
  },

  /**
   * Meneruskan satu pesan ke beberapa percakapan.
   *
   * Satu pengiriman per tujuan, BERURUTAN, bukan serentak: kuota terusan di
   * server dihitung per orang, dan lima permintaan yang tiba bersamaan
   * berlomba memakai token yang sama. Yang gagal dilaporkan per tujuan —
   * satu grup yang menolak tidak boleh membuat empat yang berhasil terlihat
   * gagal.
   */
  forwardMessage: async (messageId, targets) => {
    const out: ForwardOutcome[] = [];
    for (const conversationId of targets.slice(0, MAX_FORWARD_TARGETS)) {
      try {
        get().applyMessage(await api.forwardMessage(conversationId, uuidv7(), messageId));
        out.push({ conversationId, ok: true });
      } catch (err) {
        out.push({
          conversationId,
          ok: false,
          error: err instanceof ApiError ? err.message : 'Gagal terkirim',
        });
      }
    }
    return out;
  },

  // Alur kirim optimistik: pesan langsung muncul dengan id final buatan client,
  // lalu dikonfirmasi (atau ditandai gagal) setelah server menjawab. Karena id
  // sudah final, percobaan ulang tidak akan menghasilkan pesan ganda.
  sendMessage: async (conversationId, body) => {
    // Menulis saat sedang membaca jendela lama: kembali ke ujung dulu. Pesan
    // yang dikirim dari sana harus terlihat mendarat di tempatnya, bukan
    // menghilang ke tampungan di bawah jendela yang sedang dibaca.
    if (get().hasNewer[conversationId]) await get().returnToLatest(conversationId);

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

  addFiles: (conversationId, files, durationMs) => {
    const existing = get().uploads[conversationId] ?? [];
    const room = MAX_ATTACHMENTS - existing.length;
    if (room <= 0) return [];
    const keys: string[] = [];

    for (const file of files.slice(0, room)) {
      const key = uuidv7();
      const isImage = file.type.startsWith('image/');

      const terlaluBesar = file.size > MAX_UPLOAD_BYTES;

      const entry: Upload = {
        key,
        name: file.name,
        size: file.size,
        mime: file.type,
        durationMs,
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

      if (entry.status === 'uploading') void upload(set, get, conversationId, key, file, durationMs);
      keys.push(key);
    }
    return keys;
  },

  retryUpload: (conversationId, key) => {
    const entry = (get().uploads[conversationId] ?? []).find(u => u.key === key);
    if (!entry || !files.has(key)) return;

    patchUpload(set, conversationId, key, { status: 'uploading', progress: 0, error: null });
    void upload(set, get, conversationId, key, files.get(key)!, entry.durationMs);
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

  scheduleDelete: message => {
    const { id, conversationId } = message;
    set(s => ({
      deleting: { ...s.deleting, [id]: { conversationId, due: Date.now() + DELETE_GRACE_MS } },
    }));
    armDelete(set, get, id, DELETE_GRACE_MS);
  },

  undoDelete: id => {
    clearTimeout(deleteTimers.get(id));
    deleteTimers.delete(id);
    set(s => {
      const next = { ...s.deleting };
      delete next[id];
      return { deleting: next };
    });
  },

  undoDeletes: () => {
    for (const [id, d] of Object.entries(get().deleting)) {
      if (!d.error) get().undoDelete(id);
    }
  },

  pauseDeletes: () => {
    const now = Date.now();
    const next = { ...get().deleting };
    let changed = false;
    for (const [id, d] of Object.entries(next)) {
      if (d.error || d.remaining !== undefined) continue;
      clearTimeout(deleteTimers.get(id));
      deleteTimers.delete(id);
      next[id] = { ...d, remaining: Math.max(0, d.due - now) };
      changed = true;
    }
    if (changed) set({ deleting: next });
  },

  resumeDeletes: () => {
    const now = Date.now();
    const next = { ...get().deleting };
    const armed: [string, number][] = [];
    for (const [id, d] of Object.entries(next)) {
      if (d.remaining === undefined) continue;
      // Dilanjutkan dengan paling sedikit dua detik: orang yang baru saja
      // melepas kursor dari kabarnya tidak boleh kehilangan pilihannya
      // di detik yang sama.
      const ms = Math.max(2000, d.remaining);
      next[id] = { conversationId: d.conversationId, due: now + ms };
      armed.push([id, ms]);
    }
    if (armed.length === 0) return;
    set({ deleting: next });
    for (const [id, ms] of armed) armDelete(set, get, id, ms);
  },

  retryDelete: id => {
    const d = get().deleting[id];
    if (!d) return;
    set(s => ({ deleting: { ...s.deleting, [id]: { conversationId: d.conversationId, due: Date.now() } } }));
    armDelete(set, get, id, 0);
  },

  dismissDelete: id => get().undoDelete(id),

  discardPending: (conversationId, pendingId) =>
    set(s => ({
      pending: {
        ...s.pending,
        [conversationId]: (s.pending[conversationId] ?? []).filter(p => p.id !== pendingId),
      },
    })),

  applyMessage: m => {
    const current = get().messages[m.conversationId] ?? [];
    const known = current.some(x => x.id === m.id);

    // Catatan sematan yang BARU adalah tanda untuk membaca ulang daftar
    // sematan — satu-satunya kabar yang dikirim server tentang itu, dan dia
    // juga yang sampai lewat susulan setelah reconnect. Yang sudah pernah
    // diterapkan tidak memicu apa-apa: kabar yang sama datang lewat jawaban
    // HTTP dan lewat siaran.
    if (!known && isPinNotice(m) && get().pins[m.conversationId] !== undefined) {
      void get().loadPins(m.conversationId);
    }

    set(s => {
      // Sedang membaca jendela lama: pesan BARU ditampung, bukan ditempel.
      // Pesan yang sudah ada di jendela — suntingan, penghapusan — tetap
      // diterapkan di tempatnya.
      const windowed = (s.hasNewer[m.conversationId] || windowing.has(m.conversationId)) && !known;
      if (windowed) {
        stash.set(m.conversationId, upsert(stash.get(m.conversationId) ?? [], m));
      }
      const list = windowed ? current : upsert(s.messages[m.conversationId] ?? [], m);

      // Sematan mengikuti pesannya: yang dihapus pergi dari daftar, yang
      // disunting tampil versi barunya.
      const pinned = s.pins[m.conversationId];
      const pins =
        pinned && pinned.some(p => p.message.id === m.id)
          ? {
              ...s.pins,
              [m.conversationId]: m.deletedAt
                ? pinned.filter(p => p.message.id !== m.id)
                : pinned.map(p =>
                    p.message.id === m.id ? { ...p, message: keepReactions(p.message, m) } : p,
                  ),
            }
          : s.pins;

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
        pins,
      };
    });
  },

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
      stash.delete(conversationId);
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
        hasNewer: drop(s.hasNewer),
        members: drop(s.members),
        pins: drop(s.pins),
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

  /**
   * Satu perubahan status.
   *
   * Yang kembali ke bawaan DIHAPUS dari peta, bukan disimpan sebagai
   * 'available'. Peta ini hanya pernah memuat yang punya sesuatu untuk
   * diceritakan — sama dengan apa yang dikirim snapshot — jadi menyimpan
   * keadaan bawaan di sini membuat dua bentuk untuk satu arti, dan setiap
   * pembacanya harus mengenali keduanya.
   */
  applyStatus: status =>
    set(s => {
      const next = { ...s.statuses };
      if (status.status === 'available' && !status.text) delete next[status.userId];
      else next[status.userId] = status;

      // Status kita sendiri ikut ke `me`: siarannya memang kembali kepada kita
      // supaya tab lain ikut berubah, dan tanpa baris ini tab itu tetap
      // menampilkan pilihan yang lama di panel akunnya.
      const me =
        s.me && status.userId === s.me.id
          ? { ...s.me, status: status.status, statusText: status.text, statusExpiresAt: status.expiresAt }
          : s.me;
      return { statuses: next, me };
    }),

  applyStatusSnapshot: statuses =>
    set(() => {
      // Ditimpa, bukan digabung: snapshot adalah KEADAAN sekarang, dan status
      // yang habis waktunya selagi kita offline memang harus hilang. Dia tidak
      // pernah datang sebagai kabar tersendiri — tidak ada yang menyiarkan
      // kedaluwarsa, karena tidak ada yang menjaganya.
      const next: Record<string, UserStatus> = {};
      for (const st of statuses) next[st.userId] = st;
      return { statuses: next };
    }),

  /**
   * Nama atau foto seseorang berubah.
   *
   * Disalin ke SEMUA tempat yang menyimpan salinannya: `peer` pada daftar
   * percakapan, daftar anggota tiap percakapan, dan `me` bila itu kita sendiri.
   * Melewatkan salah satunya berarti foto baru muncul di satu tempat dan foto
   * lama bertahan di tempat lain — bentuk kegagalan yang terlihat seperti cache
   * yang rusak, padahal cache-nya justru bekerja dengan benar.
   */
  applyUserUpdated: user =>
    set(s => ({
      me: s.me && s.me.id === user.id ? { ...s.me, ...user } : s.me,
      conversations: s.conversations.map(c =>
        c.peer?.id === user.id ? { ...c, peer: { ...c.peer, ...user } } : c,
      ),
      members: Object.fromEntries(
        Object.entries(s.members).map(([id, roster]) => [
          id,
          roster.map(m =>
            m.userId === user.id
              ? { ...m, displayName: user.displayName, avatarUrl: user.avatarUrl }
              : m,
          ),
        ]),
      ),
    })),

  syncCursors: () => {
    const { messages, hasNewer } = get();
    const cursors: Record<string, number> = {};
    for (const [id, list] of Object.entries(messages)) {
      // Jendela lama tidak ikut menyusul. Cursor-nya adalah ujung JENDELA,
      // dan susulan dari sana akan menumpahkan dua ratus pesan lama ke
      // tampungan; yang terlewat toh dimuat ulang begitu jendelanya kembali ke
      // ujung.
      if (hasNewer[id]) continue;
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
    // Membaca jendela lama bukan membaca yang terbaru.
    if (get().hasNewer[conversationId]) return;
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
  durationMs?: number,
) {
  files.set(key, file);

  try {
    const size = await imageSize(file);
    const attachment = await api.uploadAttachment(file, { ...size, durationMs }, fraction => {
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

/**
 * Memasang pengatur waktu satu hapus yang ditahan. Saat waktunya tiba dan hapus
 * itu masih ditunggu (tidak diurungkan, tidak dijeda), permintaannya dikirim.
 */
function armDelete(
  set: (fn: (s: State) => Partial<State>) => void,
  get: () => State,
  id: string,
  ms: number,
) {
  clearTimeout(deleteTimers.get(id));
  deleteTimers.set(
    id,
    setTimeout(() => {
      deleteTimers.delete(id);
      const d = get().deleting[id];
      if (!d || d.remaining !== undefined || d.error) return;
      get()
        .deleteMessage(id)
        .then(() =>
          set(s => {
            const next = { ...s.deleting };
            delete next[id];
            return { deleting: next };
          }),
        )
        .catch(err =>
          set(s => ({
            deleting: {
              ...s.deleting,
              [id]: {
                conversationId: d.conversationId,
                due: 0,
                error: err instanceof Error ? err.message : 'Pesan gagal dihapus',
              },
            },
          })),
        );
    }, ms),
  );
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
    // Sebab hanya dicatat untuk penolakan yang memang dari server ini (4xx).
    // Jaringan putus dan server yang tidak menjawab (5xx — termasuk 502 dari
    // proxy di depannya) dibiarkan tanpa sebab, supaya retryStranded
    // mengirimnya ulang begitu koneksinya pulih.
    const reason = err instanceof ApiError && err.status < 500 ? err.message : undefined;
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

// Setiap perubahan antrean langsung disimpan. Pembandingnya identitas objek:
// zustand membuat objek `pending` baru hanya saat isinya memang berubah.
useStore.subscribe((state, prev) => {
  if (state.me && state.pending !== prev.pending) simpanAntrean(state.me.id, state.pending);
});
