import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  type ClipboardEvent,
  type DragEvent,
  type KeyboardEvent,
} from 'react';
import { useStore } from '../store';
import { conversationTitle } from './Sidebar';
import Avatar, { dotFor, statusLabel } from './Avatar';
import MessageBubble from './MessageBubble';
import UploadStrip from './UploadStrip';
import GroupPanel from './GroupPanel';
import ForwardDialog from './ForwardDialog';
import Icon from './Icon';
import PinBar from './PinBar';
import Tanda from './Tanda';
import EmojiPicker from './EmojiPicker';
import { formatDuration } from './VoicePlayer';
import { canRecord, MAX_RECORDING_MS, useRecorder, type Recording } from '../useRecorder';
import type { Message } from '../types';

/** Indikator "sedang mengetik" dianggap basi setelah jeda ini. */
const TYPING_TTL_MS = 4000;

/** Lama sorotan setelah melompat ke sebuah pesan. */
const JUMP_HIGHLIGHT_MS = 2000;

/**
 * Jeda yang memutus sebuah rentetan pesan.
 *
 * Dua pesan dari orang yang sama dengan jarak setengah jam bukan satu tarikan
 * napas, walau tidak ada siapa pun yang menyela di antaranya. Menempelkannya
 * jadi satu blok menyembunyikan jeda yang justru punya arti.
 */
const RUN_GAP_MS = 5 * 60_000;

const hariSama = (a: string, b: string) =>
  new Date(a).toDateString() === new Date(b).toDateString();

/**
 * Penanda hari di tengah riwayat.
 *
 * Jam saja tidak cukup begitu percakapan melewati tengah malam: "08.15" di
 * bawah "23.40" terbaca seperti tujuh jam yang sama, padahal di antaranya ada
 * satu malam penuh.
 */
function labelHari(iso: string): string {
  const at = new Date(iso);
  if (Number.isNaN(at.getTime())) return '';

  const hariIni = new Date();
  if (hariSama(at.toISOString(), hariIni.toISOString())) return 'Hari ini';

  const kemarin = new Date(hariIni);
  kemarin.setDate(kemarin.getDate() - 1);
  if (hariSama(at.toISOString(), kemarin.toISOString())) return 'Kemarin';

  return at.toLocaleDateString('id-ID', {
    weekday: 'long',
    day: 'numeric',
    month: 'long',
    ...(at.getFullYear() === hariIni.getFullYear() ? {} : { year: 'numeric' }),
  });
}

export default function ChatPanel({
  send,
  onSearch,
}: {
  send: (type: string, payload: unknown) => void;
  /** Membuka panel pencarian, dipersempit ke percakapan ini. */
  onSearch: (conversationId: string) => void;
}) {
  const me = useStore(s => s.me);
  const activeId = useStore(s => s.activeId);
  const conversations = useStore(s => s.conversations);
  const messages = useStore(s => s.messages);
  const pending = useStore(s => s.pending);
  const hasMore = useStore(s => s.hasMore);
  const hasNewer = useStore(s => s.hasNewer);
  const pins = useStore(s => s.pins);
  const jumpTarget = useStore(s => s.jumpTarget);
  const storeJump = useStore(s => s.jumpTo);
  const loadNewer = useStore(s => s.loadNewer);
  const returnToLatest = useStore(s => s.returnToLatest);
  const setPinned = useStore(s => s.setPinned);
  const members = useStore(s => s.members);
  const typing = useStore(s => s.typing);
  const online = useStore(s => s.online);
  const statuses = useStore(s => s.statuses);
  const sendMessage = useStore(s => s.sendMessage);
  const loadOlder = useStore(s => s.loadOlder);
  const uploads = useStore(s => s.uploads);
  // Server yang belum menjawab dianggap MATI: lebih baik tombolnya muncul
  // sedikit terlambat daripada muncul lalu ditarik kembali.
  const attachments = useStore(s => s.config?.attachments ?? false);
  const addFiles = useStore(s => s.addFiles);
  const replyMap = useStore(s => s.replyTo);
  const setReplyTo = useStore(s => s.setReplyTo);
  const noteMention = useStore(s => s.noteMention);
  const setMentionAll = useStore(s => s.setMentionAll);
  const ackMention = useStore(s => s.ackMention);
  const closeConversation = useStore(s => s.closeConversation);

  const [draft, setDraft] = useState('');
  const [dragging, setDragging] = useState(false);
  const [mentionQuery, setMentionQuery] = useState<{ at: number; text: string } | null>(null);
  const [mentionIndex, setMentionIndex] = useState(0);
  const [jumped, setJumped] = useState<string | null>(null);
  const [panelOpen, setPanelOpen] = useState(false);
  const [forwarding, setForwarding] = useState<Message | null>(null);
  const [pinError, setPinError] = useState<string | null>(null);
  const [emojiOpen, setEmojiOpen] = useState(false);
  // Kunci unggahan rekaman yang langsung dikirim begitu selesai terunggah.
  const [sendWhenReady, setSendWhenReady] = useState<string | null>(null);
  const emojiBox = useRef<HTMLDivElement>(null);
  const emojiButton = useRef<HTMLButtonElement>(null);
  const scrollRef = useRef<HTMLDivElement>(null);
  const bottomRef = useRef<HTMLDivElement>(null);
  const fileInput = useRef<HTMLInputElement>(null);
  const textarea = useRef<HTMLTextAreaElement>(null);
  const typingSentAt = useRef(0);
  const [, force] = useState(0);

  const conversation = conversations.find(c => c.id === activeId);
  const list = activeId ? (messages[activeId] ?? []) : [];
  const queue = activeId ? (pending[activeId] ?? []) : [];
  const drafts = activeId ? (uploads[activeId] ?? []) : [];
  const roster = activeId ? (members[activeId] ?? []) : [];
  const uploading = drafts.some(u => u.status === 'uploading');
  const attachable = drafts.some(u => u.status === 'ready');
  const replying = activeId ? (replyMap[activeId] ?? null) : null;

  // Dihitung sekali: kemampuan merekam tidak berubah selama halaman terbuka.
  const recordable = useMemo(() => attachments && canRecord(), [attachments]);

  const onRecorded = useCallback(
    ({ file, durationMs }: Recording) => {
      if (!activeId) return;
      // Rekaman masuk laci lampiran seperti berkas lain — unggahan, kemajuan,
      // dan coba-lagi-nya sama — lalu dikirim sendiri begitu siap. Kalau
      // unggahannya gagal, dia tetap di laci dengan tombol coba lagi, dan
      // tidak ada yang hilang.
      const [key] = addFiles(activeId, [file], durationMs);
      if (key) setSendWhenReady(key);
    },
    [activeId, addFiles],
  );
  const recorder = useRecorder(onRecorded);
  const recording = recorder.phase !== 'idle';
  const stopRecording = useRef(recorder.stop);
  stopRecording.current = recorder.stop;

  useEffect(() => {
    if (!sendWhenReady || !activeId) return;
    const u = drafts.find(d => d.key === sendWhenReady);
    if (!u || u.status === 'failed') {
      setSendWhenReady(null);
      return;
    }
    if (u.status !== 'ready') return;
    setSendWhenReady(null);
    // Teks yang sempat diketik selama rekamannya terunggah TIDAK ikut: dia
    // tetap di kolom tulis, menunggu dikirim dengan sengaja.
    void sendMessage(activeId, '');
  }, [drafts, sendWhenReady, activeId, sendMessage]);

  // Selama merekam, Escape membuang rekamannya — jalan keluar yang sama
  // dengan membatalkan balasan atau suntingan.
  useEffect(() => {
    if (!recording) return;
    const onKey = (e: globalThis.KeyboardEvent) => {
      if (e.key === 'Escape') stopRecording.current(false);
    };
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, [recording]);

  useEffect(() => {
    if (!emojiOpen) return;
    const onDown = (e: PointerEvent) => {
      const t = e.target as Node;
      if (emojiBox.current?.contains(t) || emojiButton.current?.contains(t)) return;
      setEmojiOpen(false);
    };
    const onKey = (e: globalThis.KeyboardEvent) => {
      if (e.key === 'Escape') setEmojiOpen(false);
    };
    document.addEventListener('pointerdown', onDown);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('pointerdown', onDown);
      document.removeEventListener('keydown', onKey);
    };
  }, [emojiOpen]);

  // Indikator typing kedaluwarsa sendiri; render ulang berkala agar hilang
  // walaupun pengirimnya keburu putus sebelum mengirim "typing: false".
  useEffect(() => {
    const t = setInterval(() => force(n => n + 1), 1000);
    return () => clearInterval(t);
  }, []);

  // Turun ke bawah saat pesan bertambah, tapi hanya kalau pengguna memang
  // sedang di dekat bawah — jangan rebut posisi orang yang sedang baca ke atas.
  const lastCount = useRef(0);
  useLayoutEffect(() => {
    const el = scrollRef.current;
    if (!el) return;

    const grew = list.length + queue.length > lastCount.current;
    lastCount.current = list.length + queue.length;

    const nearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 200;
    if (grew && nearBottom) bottomRef.current?.scrollIntoView({ block: 'end' });
  }, [list.length, queue.length]);

  // Pindah ruang: langsung ke pesan terbaru, dan pemilih sebutan ditutup.
  useLayoutEffect(() => {
    lastCount.current = 0;
    setMentionQuery(null);
    // Panel ditutup saat berpindah ruang: dia menampilkan anggota percakapan
    // TERTENTU, dan panel yang tetap terbuka sesaat menampilkan orang-orang
    // dari percakapan yang barusan ditinggalkan.
    setPanelOpen(false);
    // Rekaman yang sedang berjalan dibuang: suara yang direkam untuk satu
    // orang tidak boleh terkirim ke orang lain hanya karena layarnya berganti.
    stopRecording.current(false);
    setSendWhenReady(null);
    setEmojiOpen(false);
    bottomRef.current?.scrollIntoView({ block: 'end' });
  }, [activeId]);

  const nameOf = useCallback(
    (userId: string) =>
      roster.find(m => m.userId === userId)?.displayName ??
      (userId === me?.id ? (me?.displayName ?? 'Saya') : 'Seseorang'),
    [roster, me],
  );

  const avatarOf = useCallback(
    (userId: string) => roster.find(m => m.userId === userId)?.avatarUrl,
    [roster],
  );

  // Nama yang layak disorot di dalam teks pesan. Dihitung sekali per daftar
  // anggota, bukan di dalam tiap gelembung: sebuah percakapan dua ratus orang
  // akan menyusun daftar yang sama dua ratus kali untuk satu halaman riwayat.
  const highlights = useMemo(() => {
    const out = roster.map(m => ({ name: m.displayName, isMe: m.userId === me?.id }));
    if (conversation?.type === 'group') out.push({ name: 'semua', isMe: true });
    return out;
  }, [roster, me, conversation?.type]);

  const onSeenMention = useCallback(
    (seq: number) => {
      if (activeId) ackMention(activeId, seq);
    },
    [activeId, ackMention],
  );

  const onReply = useCallback(
    (m: Message) => {
      if (!activeId) return;
      setReplyTo(activeId, m);
      textarea.current?.focus();
    },
    [activeId, setReplyTo],
  );

  /**
   * Melompat ke sebuah pesan: kutipan, catatan sematan, bilah sematan.
   *
   * Memuat pesannya adalah urusan store — termasuk memuat JENDELA di
   * sekitarnya bila pesannya jauh di belakang. Yang tersisa di sini cuma
   * menggulir ke sana, dan itu dikerjakan efek di bawah, supaya lompatan yang
   * dimulai dari luar panel ini — hasil pencarian — mendarat dengan cara yang
   * sama persis.
   */
  const jumpTo = useCallback(
    (messageId: string, seq?: number) => {
      if (activeId) void storeJump(activeId, messageId, seq);
    },
    [activeId, storeJump],
  );

  useEffect(() => {
    if (!jumpTarget || jumpTarget.conversationId !== activeId) return;
    // Menunggu satu frame: baris barunya baru ada di DOM setelah React sempat
    // menggambar, dan getElementById membaca DOM, bukan state.
    const frame = requestAnimationFrame(() => {
      const el = document.getElementById(`msg-${jumpTarget.messageId}`);
      if (!el) return;
      el.scrollIntoView({ block: 'center', behavior: 'smooth' });
      setJumped(jumpTarget.messageId);
    });
    return () => cancelAnimationFrame(frame);
  }, [jumpTarget, activeId]);

  const onForward = useCallback((m: Message) => setForwarding(m), []);

  const onTogglePin = useCallback(
    (m: Message, pinned: boolean) => {
      setPinError(null);
      setPinned(m, pinned).catch(err =>
        setPinError(err instanceof Error ? err.message : 'Sematan gagal diubah'),
      );
    },
    [setPinned],
  );

  useEffect(() => {
    if (!jumped) return;
    const t = setTimeout(() => setJumped(null), JUMP_HIGHLIGHT_MS);
    return () => clearTimeout(t);
  }, [jumped]);

  // Calon sebutan untuk kata yang sedang diketik. "semua" hanya di grup:
  // membangunkan seluruh anggota DM adalah membangunkan satu orang, dan itu
  // sudah dilakukan pesan biasa.
  const candidates = useMemo(() => {
    if (!mentionQuery || !me) return [];
    const q = mentionQuery.text.toLowerCase();

    const people = roster
      .filter(m => m.userId !== me.id && m.displayName.toLowerCase().includes(q))
      .map(m => ({ id: m.userId, name: m.displayName, all: false }));

    const all =
      conversation?.type === 'group' && 'semua'.startsWith(q)
        ? [{ id: 'semua', name: 'semua', all: true }]
        : [];

    return [...all, ...people].slice(0, 6);
  }, [mentionQuery, roster, me, conversation?.type]);

  if (!conversation || !activeId || !me) {
    // Di layar sempit tidak ada "sebelah kiri" — daftarnya sedang memenuhi
    // layar, dan panel ini tidak perlu ikut hadir untuk mengatakan bahwa dia
    // kosong.
    return (
      <section className="hidden flex-1 flex-col items-center justify-center gap-3 px-8 text-center md:flex">
        <Tanda size={56} />
        <p className="text-base font-bold">Belum ada yang dibuka</p>
        <p className="max-w-[26ch] text-sm leading-relaxed text-muted">
          Pilih sebuah percakapan di sebelah kiri, atau mulai yang baru.
        </p>
      </section>
    );
  }

  // Siapa yang boleh menyematkan: kedua orang di DM, pemilik di grup — aturan
  // yang sama dengan server. Tombolnya tidak ditampilkan kepada yang tidak
  // boleh, bukan ditampilkan lalu ditolak.
  const canPin =
    conversation.type === 'direct' ||
    roster.some(m => m.userId === me.id && m.role === 'owner');
  const pinnedIds = new Set((pins[activeId] ?? []).map(p => p.message.id));
  const viewingOld = hasNewer[activeId] === true;

  const typers = Object.entries(typing[activeId] ?? {})
    .filter(([userId, entry]) => userId !== me.id && Date.now() - entry.at < TYPING_TTL_MS)
    .map(([, entry]) => entry.displayName);

  // Read receipt di DM: apakah lawan bicara sudah membaca sampai seq tertentu.
  const peerRead =
    conversation.type === 'direct'
      ? Math.max(
          0,
          ...(members[activeId] ?? []).filter(m => m.userId !== me.id).map(m => m.lastReadSeq),
        )
      : 0;

  function notifyTyping() {
    // Dikirim paling sering sekali per 2 detik; sisanya cuma menghabiskan bandwidth.
    const now = Date.now();
    if (now - typingSentAt.current < 2000) return;
    typingSentAt.current = now;
    send('typing', { conversationId: activeId, typing: true });
  }

  /**
   * Mencari sebutan yang sedang diketik, dari posisi kursor ke belakang.
   *
   * Berhenti di "@" pertama yang berada di awal baris atau setelah spasi, dan
   * menyerah kalau sudah melewati satu baris penuh. Spasi TIDAK mengakhiri
   * pencarian — nama tampilan boleh mengandung spasi, dan "@Budi Santoso" harus
   * tetap bisa dipilih dari daftar.
   */
  function detectMention(value: string, caret: number) {
    const before = value.slice(0, caret);
    const at = before.lastIndexOf('@');
    if (at < 0) {
      setMentionQuery(null);
      return;
    }
    const prev = at > 0 ? before[at - 1] : ' ';
    const query = before.slice(at + 1);
    if (!/\s/.test(prev!) || query.includes('\n') || query.length > 32) {
      setMentionQuery(null);
      return;
    }
    setMentionQuery({ at, text: query });
    setMentionIndex(0);
  }

  function chooseMention(choice: { id: string; name: string; all: boolean }) {
    if (!mentionQuery || !activeId) return;

    const el = textarea.current;
    const caret = el?.selectionStart ?? draft.length;
    const next = draft.slice(0, mentionQuery.at) + '@' + choice.name + ' ' + draft.slice(caret);

    setDraft(next);
    setMentionQuery(null);

    // Yang dicatat adalah ID-nya, dan inilah yang dikirim ke server. Teksnya
    // hanya untuk dibaca manusia — server tidak pernah menguraikannya.
    if (choice.all) setMentionAll(activeId, true);
    else noteMention(activeId, choice.id, choice.name);

    queueMicrotask(() => {
      const pos = mentionQuery.at + choice.name.length + 2;
      el?.focus();
      el?.setSelectionRange(pos, pos);
    });
  }

  async function submit() {
    const body = draft.trim();
    // Pesan boleh tanpa teks asalkan ada lampiran yang sudah selesai diunggah.
    if (!body && !attachable) return;

    setDraft('');
    setMentionQuery(null);
    typingSentAt.current = 0;
    send('typing', { conversationId: activeId, typing: false });
    await sendMessage(activeId!, body);
  }

  /**
   * Menyisipkan emoji di posisi kursor, bukan di ujung teks.
   *
   * Papan emoji dibiarkan terbuka: orang jarang memilih satu saja. Di layar
   * sentuh kolom tulisnya TIDAK difokuskan kembali — fokus membuka papan
   * ketik, dan papan ketik menutupi papan emoji yang baru saja dipakai.
   */
  function insertEmoji(emoji: string) {
    const el = textarea.current;
    const start = el?.selectionStart ?? draft.length;
    const end = el?.selectionEnd ?? draft.length;
    setDraft(draft.slice(0, start) + emoji + draft.slice(end));
    notifyTyping();

    const pos = start + emoji.length;
    requestAnimationFrame(() => {
      if (!el) return;
      if (!window.matchMedia('(hover: none)').matches) el.focus();
      el.setSelectionRange(pos, pos);
    });
  }

  function onKeyDown(e: KeyboardEvent<HTMLTextAreaElement>) {
    // Saat daftar sebutan terbuka, panah dan Enter miliknya — bukan milik
    // kolom tulis. Tanpa ini, Enter mengirim pesan yang masih berisi "@bud".
    if (mentionQuery && candidates.length > 0) {
      if (e.key === 'ArrowDown') {
        e.preventDefault();
        setMentionIndex(i => (i + 1) % candidates.length);
        return;
      }
      if (e.key === 'ArrowUp') {
        e.preventDefault();
        setMentionIndex(i => (i - 1 + candidates.length) % candidates.length);
        return;
      }
      if (e.key === 'Enter' || e.key === 'Tab') {
        e.preventDefault();
        chooseMention(candidates[mentionIndex]!);
        return;
      }
      if (e.key === 'Escape') {
        e.preventDefault();
        setMentionQuery(null);
        return;
      }
    }

    // Escape membatalkan balasan — jalan keluar yang sama dengan membatalkan
    // penyuntingan, jadi tidak perlu dihafal terpisah.
    if (e.key === 'Escape' && replying) {
      e.preventDefault();
      setReplyTo(activeId!, null);
      return;
    }

    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      void submit();
    }
  }

  // Satu gerbang untuk KETIGA jalan masuk berkas — tombol, seret, dan tempel.
  //
  // Menjaga tombolnya saja tidak cukup: menyeret berkas ke jendela dan menempel
  // tangkapan layar adalah dua cara yang tidak pernah melewati tombol itu, dan
  // keduanya akan berakhir sebagai unggahan yang ditolak server tanpa pernah
  // ada yang menawarkannya.
  function pick(files: FileList | File[] | null) {
    if (!attachments || !files || !activeId) return;
    const daftar = Array.from(files);
    if (daftar.length === 0) return;
    addFiles(activeId, daftar);
  }

  // Menempel gambar langsung dari papan klip — cara paling cepat mengirim
  // tangkapan layar, dan yang paling sering dicoba orang tanpa diberi tahu.
  function onPaste(e: ClipboardEvent<HTMLTextAreaElement>) {
    const files = Array.from(e.clipboardData.files);
    if (files.length === 0 || !attachments) return;
    e.preventDefault();
    pick(files);
  }

  function onDrop(e: DragEvent<HTMLElement>) {
    e.preventDefault();
    setDragging(false);
    pick(e.dataTransfer.files);
  }

  const peerOnline = conversation.peer ? online.has(conversation.peer.id) : false;
  // Status dibaca dari peta siaran, bukan dari salinan di dalam `peer`: yang
  // kedua membeku pada saat daftar percakapan diambil.
  const peerStatus = conversation.peer ? statuses[conversation.peer.id] : undefined;

  /** Apakah sebuah pesan memanggil pembaca — menentukan sorotan dan penanda. */
  const callsMe = (m: Message) =>
    m.senderId !== me.id && !m.deletedAt && (m.mentionsAll || m.mentions.includes(me.id));

  return (
    <div className="flex min-w-0 flex-1">
    <section
      className="relative flex min-w-0 flex-1 flex-col bg-canvas"
      // dragenter/dragover harus di-preventDefault, kalau tidak browser
      // membuka berkasnya sendiri dan meninggalkan halaman ini sepenuhnya.
      onDragEnter={e => {
        e.preventDefault();
        if (attachments && e.dataTransfer.types.includes('Files')) setDragging(true);
      }}
      onDragOver={e => e.preventDefault()}
      onDragLeave={e => {
        // Kursor yang bergerak antar elemen anak juga memicu dragleave.
        // Tanpa pemeriksaan ini, sorotan berkedip sepanjang perjalanan kursor.
        if (e.currentTarget.contains(e.relatedTarget as Node | null)) return;
        setDragging(false);
      }}
      onDrop={onDrop}
    >
      {dragging && (
        <div className="pointer-events-none absolute inset-3 z-10 grid place-items-center rounded-[20px] border-2 border-dashed border-accent bg-accent-soft/90 text-[15px] font-bold text-accent-text">
          Lepaskan untuk melampirkan
        </div>
      )}

      <header className="flex items-center gap-2 border-b border-line bg-surface px-2 py-2.5 md:px-4 md:py-3">
        {/* Hanya ada di layar sempit, tempat daftar percakapan benar-benar
            pergi saat sebuah percakapan dibuka. Di layar lebar keduanya
            bersebelahan, dan tombol kembali tidak mengembalikan apa pun. */}
        <button
          onClick={closeConversation}
          aria-label="Kembali ke daftar percakapan"
          className="grid size-10 shrink-0 place-items-center rounded-xl text-muted transition hover:bg-canvas hover:text-ink md:hidden"
        >
          <Icon name="kembali" />
        </button>

        <Avatar
          name={conversationTitle(conversation)}
          url={conversation.peer?.avatarUrl}
          size={40}
          grup={conversation.type === 'group'}
          dot={
            conversation.type === 'direct' ? dotFor(peerOnline, peerStatus?.status) : undefined
          }
        />
        <div className="min-w-0 flex-1">
          <h2 className="truncate text-[15px] font-bold">{conversationTitle(conversation)}</h2>
          <p className="truncate text-[13px] text-muted">
            {conversation.type === 'group'
              ? `${roster.length} anggota`
              : // Status yang dipasang orangnya menggantikan Online/Offline.
                // Yang pertama dinyatakan dengan sengaja; yang kedua cuma kabar
                // tentang apakah tabnya kebetulan terbuka — dan "Online" di
                // sebelah orang yang baru saja menulis "sedang rapat" adalah
                // undangan untuk mengganggunya.
                statusLabel(peerStatus?.status, peerStatus?.text, peerStatus?.expiresAt) ||
                (peerOnline ? 'Online' : 'Offline')}
          </p>
        </div>

        <button
          onClick={() => onSearch(activeId)}
          aria-label="Cari di percakapan ini"
          title="Cari di percakapan ini"
          className="grid size-10 shrink-0 place-items-center rounded-xl text-muted transition hover:bg-canvas hover:text-ink"
        >
          <Icon name="cari" />
        </button>

        {/* Hanya grup yang punya pengelolaan. DM tidak bisa ditambahi orang —
            lihat catatan kebocoran di server/internal/store/group.go — jadi
            tombolnya memang tidak ada di sana, bukan ada tapi menolak. */}
        {conversation.type === 'group' && (
          <button
            onClick={() => setPanelOpen(v => !v)}
            aria-label="Kelola grup"
            title="Kelola grup"
            aria-pressed={panelOpen}
            className={`grid size-10 shrink-0 place-items-center rounded-xl transition ${
              panelOpen
                ? 'bg-accent-soft text-accent-text'
                : 'text-muted hover:bg-canvas hover:text-ink'
            }`}
          >
            <Icon name="anggota" />
          </button>
        )}
      </header>

      <PinBar
        conversationId={activeId}
        canPin={canPin}
        nameOf={nameOf}
        onJump={m => jumpTo(m.id, m.seq)}
      />

      {pinError && (
        <p className="flex items-center justify-between gap-2 border-b border-line bg-danger-soft px-4 py-2 text-[13px] text-danger">
          {pinError}
          <button
            onClick={() => setPinError(null)}
            aria-label="Tutup"
            className="grid size-6 shrink-0 place-items-center rounded-md hover:bg-surface"
          >
            <Icon name="tutup" size={14} />
          </button>
        </p>
      )}

      <div ref={scrollRef} className="flex-1 overflow-y-auto px-3 py-3 md:px-6 md:py-4">
        {/* Pesan ditumpuk dari bawah: percakapan yang masih sedikit tetap
            menempel di dekat kolom tulis, bukan mengambang di atas. */}
        <div className="flex min-h-full flex-col justify-end">
          {hasMore[activeId] && (
            <div className="mb-4 text-center">
              <button
                onClick={() => void loadOlder(activeId)}
                className="rounded-full border border-line bg-surface px-3.5 py-1.5 text-[13px] font-medium text-muted transition hover:border-accent hover:text-accent-text"
              >
                Muat pesan lama
              </button>
            </div>
          )}

          {list.map((m, i) => {
            const prev = list[i - 1];
            // Hari baru selalu memutus rentetan: pesan pertama sesudah tengah
            // malam adalah pembuka, walau pengirimnya orang yang sama.
            const hariBaru = !prev || !hariSama(prev.createdAt, m.createdAt);
            const firstOfRun =
              hariBaru ||
              prev.senderId !== m.senderId ||
              prev.kind === 'system' ||
              new Date(m.createdAt).getTime() - new Date(prev.createdAt).getTime() > RUN_GAP_MS;

            return (
              <div key={m.id}>
                {hariBaru && m.createdAt && (
                  <div className="my-4 flex justify-center">
                    <span className="rounded-full bg-ink/8 px-3 py-1 text-[12px] font-bold text-muted">
                      {labelHari(m.createdAt)}
                    </span>
                  </div>
                )}

                <div
                  className={`rounded-bubble transition-colors ${
                    jumped === m.id ? 'bg-accent-soft' : ''
                  }`}
                >
                  <MessageBubble
                    message={m}
                    mine={m.senderId === me.id}
                    showAuthor={conversation.type === 'group' && firstOfRun}
                    authorName={nameOf(m.senderId)}
                    authorAvatar={avatarOf(m.senderId)}
                    firstOfRun={firstOfRun}
                    gutter={conversation.type === 'group'}
                    readByPeer={
                      conversation.type === 'direct' && m.senderId === me.id && peerRead >= m.seq
                    }
                    highlights={highlights}
                    callsMe={callsMe(m)}
                    nameOf={nameOf}
                    onReply={onReply}
                    onJump={jumpTo}
                    onForward={onForward}
                    onTogglePin={canPin ? onTogglePin : undefined}
                    pinned={pinnedIds.has(m.id)}
                    onSeen={onSeenMention}
                  />
                </div>
              </div>
            );
          })}

          {/* Jendela lama: yang di bawahnya belum dimuat. Dikatakan
              terang-terangan, karena riwayat yang berhenti di tengah tanpa
              keterangan terbaca seperti percakapan yang memang berakhir di
              sana. */}
          {viewingOld && (
            <div className="mt-4 flex flex-wrap items-center justify-center gap-2">
              <button
                onClick={() => void loadNewer(activeId)}
                className="rounded-full border border-line bg-surface px-3.5 py-1.5 text-[13px] font-medium text-muted transition hover:border-accent hover:text-accent-text"
              >
                Muat pesan berikutnya
              </button>
            </div>
          )}

          {queue.map((pe, i) => (
            <MessageBubble
              key={pe.id}
              pending={pe}
              mine
              showAuthor={false}
              authorName=""
              // Antrean selalu milik kita sendiri dan berurutan, jadi hanya yang
              // paling depan yang membuka rentetan baru.
              firstOfRun={i === 0}
              gutter={conversation.type === 'group'}
              highlights={highlights}
              nameOf={nameOf}
            />
          ))}

          <div ref={bottomRef} />

          {/* Menempel di tepi bawah area baca selama jendela lama terbuka —
              jalan pulang yang selalu terlihat, di mana pun orangnya sedang
              menggulir. */}
          {viewingOld && (
            <div className="pointer-events-none sticky bottom-1 z-10 mt-2 flex justify-center">
              <button
                onClick={() => void returnToLatest(activeId)}
                className="pointer-events-auto flex items-center gap-1.5 rounded-full bg-accent px-3.5 py-2 text-[13px] font-semibold text-accent-ink shadow-pop transition hover:brightness-110"
              >
                <Icon name="bawah" size={16} />
                Ke pesan terbaru
              </button>
            </div>
          )}
        </div>
      </div>

      {typers.length > 0 && (
        <p className="flex items-center gap-2 px-5 pb-1.5 text-[13px] text-muted">
          <span className="denyut flex shrink-0 items-center gap-1" aria-hidden>
            <span className="size-1.5 rounded-full bg-muted" />
            <span className="size-1.5 rounded-full bg-muted" />
            <span className="size-1.5 rounded-full bg-muted" />
          </span>
          <span className="truncate">{typers.join(', ')} sedang mengetik</span>
        </p>
      )}

      <UploadStrip conversationId={activeId} />

      {replying && (
        <div className="flex items-start gap-2.5 border-t border-line bg-surface px-3 py-2.5 md:px-4">
          <span className="mt-0.5 shrink-0 text-accent-text">
            <Icon name="balas" size={16} />
          </span>
          <div className="min-w-0 flex-1 border-l-[3px] border-accent pl-2.5">
            <p className="text-[13px] font-bold text-accent-text">
              Membalas {nameOf(replying.senderId)}
            </p>
            <p className="truncate text-[13px] text-muted">{replying.body || 'Lampiran'}</p>
          </div>
          <button
            onClick={() => setReplyTo(activeId, null)}
            aria-label="Batalkan balasan"
            className="grid size-7 shrink-0 place-items-center rounded-lg text-muted transition hover:bg-canvas hover:text-ink"
          >
            <Icon name="tutup" size={16} />
          </button>
        </div>
      )}

      <div className="relative border-t border-line bg-surface p-3">
        {mentionQuery && candidates.length > 0 && (
          <div className="absolute bottom-full left-3 z-20 mb-2 w-72 overflow-hidden rounded-2xl border border-line bg-surface shadow-pop">
            {candidates.map((c, i) => (
              <button
                key={c.id}
                // onMouseDown, bukan onClick: klik biasa lebih dulu memicu blur
                // pada textarea, dan posisi kursor yang dipakai untuk menyisipkan
                // nama sudah hilang saat handler-nya jalan.
                onMouseDown={e => {
                  e.preventDefault();
                  chooseMention(c);
                }}
                onMouseEnter={() => setMentionIndex(i)}
                className={`block w-full px-3.5 py-2.5 text-left text-sm transition ${
                  i === mentionIndex ? 'bg-accent-soft' : 'hover:bg-canvas'
                }`}
              >
                {c.all ? (
                  <>
                    <span className="font-bold">@semua</span>
                    <span className="ml-2 text-[13px] text-muted">membangunkan seluruh anggota</span>
                  </>
                ) : (
                  <span className="font-semibold">{c.name}</span>
                )}
              </button>
            ))}
          </div>
        )}

        {emojiOpen && !recording && (
          <div ref={emojiBox} className="absolute bottom-full left-3 z-20 mb-2">
            <EmojiPicker onPick={insertEmoji} />
          </div>
        )}

        {recorder.error && (
          <p
            role="alert"
            className="mb-2 flex items-center justify-between gap-2 rounded-xl bg-danger-soft px-3 py-2 text-[13px] text-danger"
          >
            {recorder.error}
            <button
              onClick={recorder.clearError}
              aria-label="Tutup"
              className="grid size-6 shrink-0 place-items-center rounded-md hover:bg-surface"
            >
              <Icon name="tutup" size={14} />
            </button>
          </p>
        )}

        {recording ? (
          <RecordingBar
            starting={recorder.phase === 'starting'}
            elapsed={recorder.elapsed}
            levels={recorder.levels}
            onCancel={() => recorder.stop(false)}
            onSend={() => recorder.stop(true)}
          />
        ) : (
          <div className="flex items-end gap-2">
            {/* Satu kotak berisi tiga hal: emoji di kiri, tulisan di tengah,
                lampiran di kanan. Keduanya tombol yang mengubah APA yang
                ditulis, jadi tempatnya di dalam kotak tulis — tombol di
                luarnya hanya satu, yang mengirim. */}
            <div className="flex min-w-0 flex-1 items-end rounded-[22px] border border-line-strong bg-canvas transition focus-within:border-accent focus-within:bg-surface">
              <button
                ref={emojiButton}
                onClick={() => setEmojiOpen(v => !v)}
                aria-label="Pilih emoji"
                aria-expanded={emojiOpen}
                title="Emoji"
                className={`grid size-11 shrink-0 place-items-center rounded-full transition ${
                  emojiOpen ? 'text-accent-text' : 'text-muted hover:text-ink'
                }`}
              >
                <Icon name="emoji" size={22} />
              </button>

              <textarea
                ref={textarea}
                value={draft}
                rows={1}
                onChange={e => {
                  setDraft(e.target.value);
                  detectMention(e.target.value, e.target.selectionStart);
                  notifyTyping();
                }}
                onClick={e => detectMention(e.currentTarget.value, e.currentTarget.selectionStart)}
                onBlur={() => setMentionQuery(null)}
                onPaste={onPaste}
                onKeyDown={onKeyDown}
                // Satu kalimat pendek untuk semua ruang. Petunjuk "@ untuk
                // menyebut" sebelumnya terpotong di tengah pada layar ponsel —
                // petunjuk yang tidak selesai terbaca lebih buruk daripada tidak
                // ada, dan daftar sebutannya toh muncul sendiri begitu @ diketik.
                placeholder="Tulis pesan…"
                aria-label="Tulis pesan"
                className="max-h-36 min-h-11 min-w-0 flex-1 resize-none bg-transparent py-2.5 text-[15px] leading-6 outline-none placeholder:text-muted focus-visible:outline-none"
              />

              {/* Tombolnya hanya ada kalau server ini memang menerima lampiran —
                  aturan yang sama dengan tombol notifikasi di sidebar dan tombol
                  kelola grup: yang tidak boleh ditekan tidak ditampilkan, bukan
                  ditampilkan lalu ditolak. */}
              {attachments && (
                <>
                  <input
                    ref={fileInput}
                    type="file"
                    multiple
                    hidden
                    onChange={e => {
                      pick(e.target.files);
                      // Dikosongkan supaya memilih berkas yang SAMA dua kali berturut-
                      // turut tetap memicu change — nilainya tidak berubah, jadi
                      // browser tidak akan memberi tahu.
                      e.target.value = '';
                    }}
                  />
                  <button
                    onClick={() => fileInput.current?.click()}
                    aria-label="Lampirkan berkas"
                    title="Lampirkan berkas — bisa juga seret ke sini atau tempel gambar"
                    className="grid size-11 shrink-0 place-items-center rounded-full text-muted transition hover:text-ink"
                  >
                    <Icon name="klip" />
                  </button>
                </>
              )}
            </div>

            {/* Kotak kosong: tombolnya merekam. Ada yang bisa dikirim: tombolnya
                mengirim. Satu tempat, satu tombol — dua tombol bulat
                bersebelahan dengan warna yang sama akan tertukar oleh ibu jari. */}
            {recordable && !draft.trim() && !attachable && !uploading ? (
              <button
                onClick={() => void recorder.start()}
                aria-label="Rekam pesan suara"
                title={`Rekam pesan suara — paling lama ${formatDuration(MAX_RECORDING_MS)}`}
                className="flex h-11 shrink-0 items-center gap-2 rounded-full bg-accent px-3 font-semibold text-accent-ink transition hover:brightness-110 active:scale-95 sm:px-4"
              >
                <Icon name="mikrofon" />
                <span className="hidden sm:inline">Rekam</span>
              </button>
            ) : (
              <button
                onClick={() => void submit()}
                // Unggahan yang belum selesai menahan tombol kirim. Melepasnya
                // lebih awal akan mengirim pesan TANPA lampiran yang jelas-jelas
                // terlihat di layar — kegagalan yang diam dan membingungkan.
                disabled={(!draft.trim() && !attachable) || uploading}
                aria-label="Kirim pesan"
                title={uploading ? 'Menunggu unggahan selesai' : 'Kirim (Enter)'}
                className="flex h-11 shrink-0 items-center gap-2 rounded-full bg-accent px-3 font-semibold text-accent-ink transition hover:brightness-110 active:scale-95 disabled:opacity-40 disabled:hover:brightness-100 sm:px-4"
              >
                <Icon name="kirim" size={20} />
                {/* Kata "Kirim" ikut tampil begitu ada tempatnya. Pesawat kertas
                    sudah dikenal luas, tapi tombol berlabel tetap lebih cepat
                    dipercaya oleh orang yang baru pertama membuka aplikasi ini. */}
                <span className="hidden sm:inline">{uploading ? 'Mengunggah…' : 'Kirim'}</span>
              </button>
            )}
          </div>
        )}
      </div>
    </section>

      {forwarding && (
        <ForwardDialog message={forwarding} onClose={() => setForwarding(null)} />
      )}

      {panelOpen && conversation.type === 'group' && (
        <GroupPanel conversation={conversation} onClose={() => setPanelOpen(false)} />
      )}
    </div>
  );
}

/**
 * Bilah yang menggantikan kolom tulis selama merekam.
 *
 * Tiga hal saja: buang, seberapa lama, kirim. Batang-batang di tengah adalah
 * tingkat suara yang benar-benar tertangkap, bukan hiasan — mikrofon yang
 * dibisukan dari perangkatnya terlihat sebagai garis datar, sebelum lima menit
 * sunyi terlanjur terkirim.
 */
function RecordingBar({
  starting,
  elapsed,
  levels,
  onCancel,
  onSend,
}: {
  starting: boolean;
  elapsed: number;
  levels: number[];
  onCancel: () => void;
  onSend: () => void;
}) {
  const BARS = 32;
  const padded = [...Array<number>(Math.max(0, BARS - levels.length)).fill(0), ...levels];
  const sisa = MAX_RECORDING_MS - elapsed;

  return (
    <div role="group" aria-label="Merekam pesan suara" className="flex items-center gap-2">
      <button
        onClick={onCancel}
        aria-label="Buang rekaman"
        title="Buang rekaman (Esc)"
        className="grid size-11 shrink-0 place-items-center rounded-full text-danger transition hover:bg-danger-soft"
      >
        <Icon name="hapus" />
      </button>

      <div className="flex h-11 min-w-0 flex-1 items-center gap-3 rounded-[22px] border border-line-strong bg-canvas px-4">
        {starting ? (
          <span className="truncate text-[14px] text-muted">Menunggu izin mikrofon…</span>
        ) : (
          <>
            <span className="rekam size-2.5 shrink-0 rounded-full bg-danger" aria-hidden />
            <span className="shrink-0 text-[15px] font-semibold tabular-nums" aria-live="off">
              {formatDuration(elapsed)}
            </span>
            <span
              aria-hidden
              className="flex h-6 min-w-0 flex-1 items-center justify-end gap-[3px] overflow-hidden"
            >
              {padded.map((level, i) => (
                <span
                  key={i}
                  className="w-[3px] shrink-0 rounded-full bg-accent"
                  style={{ height: `${Math.max(3, Math.round(level * 24))}px` }}
                />
              ))}
            </span>
            {/* Peringatan menjelang batas, bukan hitungan mundur sepanjang
                waktu: yang perlu tahu hanya yang hampir sampai. */}
            {sisa <= 30_000 && (
              <span className="shrink-0 text-[12px] font-semibold text-danger tabular-nums">
                sisa {formatDuration(sisa)}
              </span>
            )}
          </>
        )}
      </div>

      <button
        onClick={onSend}
        disabled={starting}
        aria-label="Kirim pesan suara"
        className="flex h-11 shrink-0 items-center gap-2 rounded-full bg-accent px-3 font-semibold text-accent-ink transition hover:brightness-110 active:scale-95 disabled:opacity-40 sm:px-4"
      >
        <Icon name="kirim" size={20} />
        <span className="hidden sm:inline">Kirim</span>
      </button>
    </div>
  );
}
