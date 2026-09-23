import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState, type DragEvent } from 'react';
import { useStore } from '../store';
import { buildHistory, rowId, rowLabel } from '../chatHistory';
import { useHistoryScroll } from '../useHistoryScroll';
import { useRovingLog } from '../useRovingLog';
import { useCoarsePointer } from '../useMediaQuery';
import ChatHeader from './ChatHeader';
import Composer, { type ComposerHandle } from './Composer';
import {
  DayDivider,
  DeletedRun,
  EmptyConversation,
  HistorySkeleton,
  NoConversation,
} from './ConversationStates';
import type { WaitingDelete } from './DeleteNotice';
import NoticeStack, { type Flash } from './NoticeStack';
import ForwardDialog from './ForwardDialog';
import GroupPanel from './GroupPanel';
import Icon from './Icon';
import IconButton from './ui/IconButton';
import PillButton from './ui/PillButton';
import MessageBubble from './MessageBubble';
import PinBar from './PinBar';
import { excerpt } from '../format';
import { T } from '../teks';
import { conversationTitle } from './Sidebar';
import { DELETE_GRACE_MS } from '../store';
import type { Message, PendingMessage } from '../types';

/** Indikator "sedang mengetik" dianggap basi setelah jeda ini. */
const TYPING_TTL_MS = 4000;

/**
 * Jeda sebelum bilah "menyambungkan ulang" muncul.
 *
 * Koneksi yang putus sesaat — ganti jaringan, server berganti instance — pulih
 * dalam satu-dua detik. Bilah yang berkedip untuk setiap putus sesaat itu
 * mengajari orang mengabaikannya, persis sebelum putus yang sungguhan.
 */
const OFFLINE_NOTICE_DELAY_MS = 2500;

/** Lama sorotan setelah melompat ke sebuah pesan. */
const JUMP_HIGHLIGHT_MS = 2000;

/** Lama kabar singkat ("Teks disalin.") tampil. */
const FLASH_MS = 2500;

/**
 * Jeda sebelum sematan benar-benar dikirim. Sematan disiarkan ke semua orang
 * sebagai catatan di riwayat, dan satu Enter yang meleset di menu tidak boleh
 * langsung menulis catatan itu.
 */
const PIN_GRACE_MS = 4000;


/**
 * Satu percakapan: kepala, riwayat, kabar-kabar di atas kolom tulis, dan
 * kolom tulisnya.
 *
 * Yang tinggal di sini adalah yang menyambungkan bagian-bagian itu — siapa
 * yang sedang dipilih di riwayat, pesan mana yang sedang dihapus, apa yang
 * diumumkan ke pembaca layar. Bagian-bagiannya sendiri punya berkasnya
 * masing-masing: ChatHeader, Composer, ConversationStates, DeleteNotice, dan
 * aturan gulir serta papan ketik riwayat di useHistoryScroll/useRovingLog.
 */
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
  const loadOlder = useStore(s => s.loadOlder);
  const returnToLatest = useStore(s => s.returnToLatest);
  const setPinned = useStore(s => s.setPinned);
  const members = useStore(s => s.members);
  const typing = useStore(s => s.typing);
  const attachments = useStore(s => s.config?.attachments ?? false);
  const addFiles = useStore(s => s.addFiles);
  const setReplyTo = useStore(s => s.setReplyTo);
  const ackMention = useStore(s => s.ackMention);
  const connected = useStore(s => s.connected);
  const deleting = useStore(s => s.deleting);
  const undoDeletes = useStore(s => s.undoDeletes);
  const pauseDeletes = useStore(s => s.pauseDeletes);
  const resumeDeletes = useStore(s => s.resumeDeletes);
  const retryDelete = useStore(s => s.retryDelete);
  const dismissDelete = useStore(s => s.dismissDelete);
  const discardPending = useStore(s => s.discardPending);

  const [dragging, setDragging] = useState(false);
  const [jumped, setJumped] = useState<string | null>(null);
  const [panelOpen, setPanelOpen] = useState(false);
  const [forwarding, setForwarding] = useState<Message | null>(null);
  const [pinError, setPinError] = useState<string | null>(null);
  const [showOffline, setShowOffline] = useState(false);
  // Baris yang sedang dipilih di riwayat (roving focus). null = yang terakhir.
  const [focusedRow, setFocusedRow] = useState<string | null>(null);
  // Permintaan menyunting dari ↑ di kolom kosong; `nonce` membuat permintaan
  // kedua untuk pesan yang sama tetap terbaca sebagai permintaan baru.
  const [editRequest, setEditRequest] = useState<{ id: string; nonce: number } | null>(null);
  const [flash, setFlash] = useState<Flash | null>(null);
  // Satu wilayah status untuk semua kabar yang harus dibacakan: pesan baru,
  // hapus yang dijadwalkan, salinan. Wilayah ini selalu terpasang — yang
  // lahir bersama teksnya sering dilewatkan pembaca layar.
  const [announcement, setAnnouncementRaw] = useState('');
  // Kalimat yang sama dua kali berturut-turut tidak dibacakan ulang oleh
  // pembaca layar — isi wilayahnya tidak berubah. Spasi tak terlihat yang
  // bergantian membuatnya berubah.
  const announceFlip = useRef(false);
  const setAnnouncement = useCallback((text: string) => {
    announceFlip.current = !announceFlip.current;
    setAnnouncementRaw(announceFlip.current ? `${text}\u200b` : text);
  }, []);
  const [, force] = useState(0);

  const scrollRef = useRef<HTMLDivElement>(null);
  const contentRef = useRef<HTMLDivElement>(null);
  const bottomRef = useRef<HTMLDivElement>(null);
  const composer = useRef<ComposerHandle>(null);
  const undoButton = useRef<HTMLButtonElement>(null);
  const lastAnnounced = useRef<{ conversationId: string | null; messageId: string | null }>({
    conversationId: null,
    messageId: null,
  });

  const touch = useCoarsePointer();
  const conversation = conversations.find(c => c.id === activeId);
  const list = useMemo(() => (activeId ? (messages[activeId] ?? []) : []), [messages, activeId]);
  const queue = activeId ? (pending[activeId] ?? []) : [];
  const roster = useMemo(() => (activeId ? (members[activeId] ?? []) : []), [members, activeId]);
  const rows = useMemo(() => buildHistory(list), [list]);

  const history = useHistoryScroll({
    scrollRef,
    contentRef,
    bottomRef,
    roomId: activeId,
    messageCount: list.length,
    lastMessageId: list.at(-1)?.id ?? null,
    pendingCount: queue.length,
    failedCount: queue.filter(p => p.status === 'failed').length,
  });

  // Baris yang ada di urutan Tab. Bawaannya yang terakhir, supaya Tab pertama
  // ke dalam riwayat mendarat di tempat orang biasa membaca.
  const activeRow =
    focusedRow && rows.some(r => rowId(r) === focusedRow)
      ? focusedRow
      : rows.length > 0
        ? rowId(rows.at(-1)!)
        : null;

  const roving = useRovingLog({
    logRef: scrollRef,
    activeRow,
    onEmptyAction: () => setAnnouncement(T.noActionForDeleted),
  });

  // Putus sesaat tidak diumumkan; yang bertahan lebih dari beberapa detik ya.
  useEffect(() => {
    if (connected) {
      setShowOffline(false);
      return;
    }
    const t = setTimeout(() => setShowOffline(true), OFFLINE_NOTICE_DELAY_MS);
    return () => clearTimeout(t);
  }, [connected]);

  // Indikator typing kedaluwarsa sendiri; render ulang berkala agar hilang
  // walaupun pengirimnya keburu putus sebelum mengirim "typing: false".
  useEffect(() => {
    const t = setInterval(() => force(n => n + 1), 1000);
    return () => clearInterval(t);
  }, []);

  useEffect(() => {
    if (!flash) return;
    if (flash.sticky) return;
    const t = setTimeout(() => setFlash(null), FLASH_MS);
    return () => clearTimeout(t);
  }, [flash]);

  useEffect(() => {
    if (!jumped) return;
    const t = setTimeout(() => setJumped(null), JUMP_HIGHLIGHT_MS);
    return () => clearTimeout(t);
  }, [jumped]);

  // Pindah ruang: pilihan riwayat dan panel ikut direset. Panel menampilkan
  // anggota percakapan TERTENTU, dan panel yang tetap terbuka sesaat
  // menampilkan orang-orang dari percakapan yang barusan ditinggalkan.
  useLayoutEffect(() => {
    setFocusedRow(null);
    setEditRequest(null);
    setPanelOpen(false);
  }, [activeId]);

  // ---- hapus yang ditahan ----

  const waitingIds = Object.entries(deleting)
    .filter(([, d]) => !d.error)
    .map(([id]) => id);
  const failedDelete = Object.entries(deleting).find(([, d]) => d.error);
  const waitingCount = waitingIds.length;
  const lastWaiting = useRef(0);

  // Hapus baru dijadwalkan: diumumkan, dan fokus yang hilang bersama tombol
  // menu pesannya dipindah ke "Urungkan" — satu-satunya tindakan yang masih
  // masuk akal untuk pesan itu.
  useEffect(() => {
    const more = waitingCount > lastWaiting.current;
    lastWaiting.current = waitingCount;
    if (!more) return;
    setAnnouncement(T.deleteScheduled(DELETE_GRACE_MS / 1000));
    const frame = requestAnimationFrame(() => {
      const lost = !document.activeElement || document.activeElement === document.body;
      const undo = undoButton.current;
      if (!lost || !undo) return;
      // Ditandai supaya kabar hapus tidak menganggap fokus ini sebagai
      // "orangnya sedang membaca" dan menjeda hitung mundurnya.
      undo.dataset.autofocus = '1';
      undo.focus();
    });
    return () => cancelAnimationFrame(frame);
  }, [waitingCount]);

  const focusRowOf = (ids: string[]) =>
    requestAnimationFrame(() => {
      if (document.activeElement && document.activeElement !== document.body) return;
      const row = ids
        .map(id => scrollRef.current?.querySelector<HTMLElement>(`[data-msg="${id}"]`))
        .find(Boolean);
      (row ?? composer.current)?.focus();
    });

  const undoAll = () => {
    undoDeletes();
    setAnnouncement(T.deleteUndone);
    focusRowOf(waitingIds);
  };

  // ---- tindakan pesan ----

  // Pembaca sendiri disebut "Kamu" — di kutipan, di bilah sematan, di mana
  // pun nama pengirim ditulis. Nama sendiri di sana terbaca seperti orang lain.
  const nameOf = useCallback(
    (userId: string) =>
      userId === me?.id
        ? 'Kamu'
        : (roster.find(m => m.userId === userId)?.displayName ?? 'Seseorang'),
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
      composer.current?.focus();
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
      // Lompatan membawa pembaca menjauh dari bawah: riwayat berhenti
      // menempel, supaya gambar yang selesai dimuat tidak menariknya kembali.
      history.release();
      el.scrollIntoView({ block: 'center', behavior: 'smooth' });
      setJumped(jumpTarget.messageId);
    });
    return () => cancelAnimationFrame(frame);
  }, [jumpTarget, activeId]);

  const onForward = useCallback((m: Message) => setForwarding(m), []);

  const showFlash = useCallback((f: Flash) => {
    setFlash(f);
    setAnnouncement(f.text);
  }, []);

  const onCopy = useCallback(
    (m: Message) => {
      if (!navigator.clipboard) {
        showFlash({ tone: 'warn', text: T.copyBlocked });
        return;
      }
      navigator.clipboard
        .writeText(m.body)
        .then(() => showFlash({ tone: 'ok', text: T.copied }))
        .catch(() => showFlash({ tone: 'warn', text: T.copyFailed }));
    },
    [showFlash],
  );

  // Pesan gagal yang ditarik kembali ke kolom tulis. Teks yang sedang ditulis
  // tidak ditimpa: yang ditarik ditaruh di depannya, dipisah baris baru.
  const onRewrite = useCallback(
    (p: PendingMessage) => {
      composer.current?.restoreDraft(p.body);
      discardPending(p.conversationId, p.id);
      if (p.attachments.length > 0) {
        showFlash({ tone: 'warn', text: T.attachmentsDropped });
      }
    },
    [discardPending, showFlash],
  );

  const onEditEnd = useCallback(() => {
    requestAnimationFrame(() => composer.current?.focus());
  }, []);

  // Sematan ditahan sebentar dengan "Urungkan", seperti hapus: dia langsung
  // menjadi catatan yang dilihat semua orang, dan tidak bisa ditarik diam-diam.
  const pinPending = useRef<{ message: Message; pinned: boolean; timer: number } | null>(null);

  const commitPin = useCallback(() => {
    const p = pinPending.current;
    if (!p) return;
    clearTimeout(p.timer);
    pinPending.current = null;
    setFlash(f => (f?.sticky ? null : f));
    setPinError(null);
    setPinned(p.message, p.pinned).catch(err =>
      setPinError(err instanceof Error ? err.message : T.pinFailed),
    );
  }, [setPinned]);

  const cancelPin = useCallback(() => {
    const p = pinPending.current;
    if (!p) return;
    clearTimeout(p.timer);
    pinPending.current = null;
    setFlash(null);
    setAnnouncement(T.pinUndone);
    focusRowOf([p.message.id]);
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const onTogglePin = useCallback(
    (m: Message, pinned: boolean) => {
      commitPin();
      pinPending.current = {
        message: m,
        pinned,
        timer: window.setTimeout(commitPin, PIN_GRACE_MS),
      };
      showFlash({
        tone: 'ok',
        text: pinned ? T.pinning : T.unpinning,
        action: { label: T.undo, onClick: cancelPin },
        sticky: true,
      });
    },
    [commitPin, cancelPin, showFlash],
  );

  // Sematan yang masih ditahan tetap dikirim saat pindah ruang atau pergi:
  // yang ingin membatalkannya punya tombolnya selama kabar itu terlihat.
  useEffect(() => () => commitPin(), [activeId, commitPin]);

  // Esc di kolom tulis yang kosong: ke riwayat, ke pesan TERBARU.
  const leaveToHistory = useCallback(() => {
    const rowsEls = scrollRef.current?.querySelectorAll<HTMLElement>('[data-msg]');
    const last = rowsEls?.[rowsEls.length - 1];
    if (!last) return false;
    setFocusedRow(null);
    last.focus();
    last.scrollIntoView({ block: 'nearest' });
    return true;
  }, []);

  // ↑ di kolom kosong: sunting pesan terakhir sendiri yang masih bisa disunting.
  const editLast = useCallback(() => {
    if (!me) return false;
    const last = [...list]
      .reverse()
      .find(m => m.senderId === me.id && !m.deletedAt && m.body !== '' && !deleting[m.id]);
    if (!last) return false;
    setFocusedRow(last.id);
    setEditRequest(r => ({ id: last.id, nonce: (r?.nonce ?? 0) + 1 }));
    document.getElementById(`msg-${last.id}`)?.scrollIntoView({ block: 'nearest' });
    return true;
  }, [list, me, deleting]);

  // Pesan baru di ujung riwayat: kalau fokus sedang tidak di riwayat, baris
  // yang masuk urutan Tab ikut pindah ke pesan terbaru. Kalau sedang di sana,
  // pilihan orangnya tidak direbut.
  const tailId = list.at(-1)?.id;
  useEffect(() => {
    if (!scrollRef.current?.contains(document.activeElement)) setFocusedRow(null);
  }, [tailId]);

  // Pesan baru dari orang lain dibacakan satu kalimat. Riwayat yang dimuat,
  // pesan lama yang ditarik ke atas, dan pesan sendiri tidak.
  useEffect(() => {
    const last = list.at(-1);
    const seen = lastAnnounced.current;
    const sameRoom = seen.conversationId === activeId;
    lastAnnounced.current = { conversationId: activeId, messageId: last?.id ?? null };
    if (!sameRoom || !last || last.id === seen.messageId || !me) return;
    if (last.senderId === me.id || last.kind === 'system' || last.deletedAt) return;
    const who = roster.find(m => m.userId === last.senderId)?.displayName ?? 'Seseorang';
    setAnnouncement(`Pesan baru dari ${who}: ${excerpt(last)}`);
  }, [list, activeId, me, roster]);

  if (!conversation || !activeId || !me) return <NoConversation />;

  // Siapa yang boleh menyematkan: kedua orang di DM, pemilik di grup — aturan
  // yang sama dengan server. Tombolnya tidak ditampilkan kepada yang tidak
  // boleh, bukan ditampilkan lalu ditolak.
  const isGroup = conversation.type === 'group';
  const canPin = !isGroup || roster.some(m => m.userId === me.id && m.role === 'owner');
  const pinnedIds = new Set((pins[activeId] ?? []).map(p => p.message.id));
  const viewingOld = hasNewer[activeId] === true;
  const historyLoading = hasMore[activeId] === undefined && list.length === 0;
  const empty = !historyLoading && list.length === 0 && queue.length === 0;

  const typers = Object.entries(typing[activeId] ?? {})
    .filter(([userId, entry]) => userId !== me.id && Date.now() - entry.at < TYPING_TTL_MS)
    .map(([, entry]) => entry.displayName);

  // Read receipt di DM: apakah lawan bicara sudah membaca sampai seq tertentu.
  const peerRead = isGroup
    ? 0
    : Math.max(0, ...roster.filter(m => m.userId !== me.id).map(m => m.lastReadSeq));

  /** Apakah sebuah pesan memanggil pembaca — menentukan sorotan dan penanda. */
  const callsMe = (m: Message) =>
    m.senderId !== me.id && !m.deletedAt && (m.mentionsAll || m.mentions.includes(me.id));

  // Kabar hapus mencakup SEMUA percakapan: pindah ruang di tengah jeda tidak
  // boleh menyembunyikan satu-satunya jalan untuk membatalkannya.
  const waiting: WaitingDelete[] = waitingIds.map(id => {
    const d = deleting[id]!;
    const m = (messages[d.conversationId] ?? []).find(x => x.id === id);
    return { ...d, id, text: m ? excerpt(m) : '' };
  });

  function onDrop(e: DragEvent<HTMLElement>) {
    e.preventDefault();
    setDragging(false);
    const files = Array.from(e.dataTransfer.files);
    if (attachments && files.length > 0) addFiles(activeId!, files);
  }

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
            {T.dropToAttach}
          </div>
        )}

        <ChatHeader
          conversation={conversation}
          memberCount={roster.length}
          panelOpen={panelOpen}
          onTogglePanel={() => setPanelOpen(v => !v)}
          onSearch={() => onSearch(activeId)}
        />

        <PinBar
          conversationId={activeId}
          canPin={canPin}
          nameOf={nameOf}
          onJump={m => jumpTo(m.id, m.seq)}
        />

        {showOffline && (
          <p
            role="status"
            // Bukan saffron: saffron berarti ada yang memanggilmu, dan kabar
            // jaringan bukan panggilan.
            className="flex items-center gap-2.5 border-b border-line bg-surface px-4 py-2.5 text-[14px] text-ink"
          >
            <span className="denyut flex shrink-0 items-center gap-1" aria-hidden>
              <span className="size-1.5 rounded-full bg-muted" />
              <span className="size-1.5 rounded-full bg-muted" />
              <span className="size-1.5 rounded-full bg-muted" />
            </span>
            <span>
              <strong className="font-semibold">{T.reconnecting}</strong> {T.reconnectingBody}
            </span>
          </p>
        )}

        {pinError && (
          <p
            role="alert"
            className="flex items-center justify-between gap-2 border-b border-line bg-danger-soft py-1.5 pr-2 pl-4 text-[14px] text-danger"
          >
            {pinError}
            <IconButton icon="tutup" size="sm" tone="danger" label={T.closeError} onClick={() => setPinError(null)} />
          </p>
        )}

        <div
          ref={scrollRef}
          // Log tanpa pengumuman otomatis: riwayat yang dimuat dan pesan lama
          // yang ditarik ke atas juga "tambahan", dan membacakan semuanya
          // menenggelamkan satu kalimat yang penting.
          role="log"
          aria-live="off"
          aria-label="Riwayat pesan"
          aria-busy={historyLoading}
          aria-describedby="petunjuk-riwayat"
          onKeyDown={roving.onKeyDown}
          className="flex-1 overflow-y-auto px-3 py-3 md:px-6 md:py-5"
        >
          <p id="petunjuk-riwayat" className="sr-only">
            Panah atas dan bawah untuk berpindah pesan, Enter untuk membuka tindakan pesan.
          </p>
          {/* Pesan ditumpuk dari bawah: percakapan yang masih sedikit tetap
              menempel di dekat kolom tulis, bukan mengambang di atas. */}
          {/* Kolom baca dibatasi: di layar lebar, pesan sendiri dan pesan
              orang lain yang berjarak seribu piksel membuat mata menyeberang
              layar untuk setiap giliran bicara. */}
          <div ref={contentRef} className="mx-auto flex min-h-full w-full max-w-[58rem] flex-col justify-end">
            {hasMore[activeId] && (
              <div className="mb-4 text-center">
                <PillButton tone="quiet" size="sm" className="mx-auto" onClick={() => void loadOlder(activeId)}>
                  {T.loadOlder}
                </PillButton>
              </div>
            )}

            {historyLoading && <HistorySkeleton />}

            {empty && (
              <EmptyConversation
                title={conversationTitle(conversation)}
                avatarUrl={conversation.peer?.avatarUrl}
                group={isGroup}
                canAttach={attachments}
                touch={touch}
              />
            )}

            {rows.map(row => {
              const id = rowId(row);
              const first = row.kind === 'message' ? row.message : row.messages[0]!;
              return (
                <div key={id}>
                  {row.newDay && first.createdAt && <DayDivider iso={first.createdAt} />}

                  {/* Satu baris riwayat = satu pemberhentian roving focus.
                      Cincinnya digambar pada gelembung (`.sasaran-fokus`),
                      bukan selebar baris — lihat index.css. */}
                  <div
                    data-msg={id}
                    role="article"
                    tabIndex={activeRow === id ? 0 : -1}
                    aria-label={rowLabel(row, nameOf, me.id, x => deleting[x] !== undefined)}
                    onFocus={() => setFocusedRow(id)}
                    className={`cincin-sendiri rounded-bubble outline-none transition-colors ${
                      jumped === id ? 'bg-accent-soft' : ''
                    }`}
                  >
                    {row.kind === 'deleted' ? (
                      <DeletedRun
                        messages={row.messages}
                        mine={first.senderId === me.id}
                        gutter={isGroup}
                      />
                    ) : (
                      <MessageBubble
                        message={row.message}
                        mine={row.message.senderId === me.id}
                        showAuthor={isGroup && row.firstOfRun}
                        authorName={nameOf(row.message.senderId)}
                        authorAvatar={avatarOf(row.message.senderId)}
                        firstOfRun={row.firstOfRun}
                        gutter={isGroup}
                        readByPeer={
                          !isGroup && row.message.senderId === me.id && peerRead >= row.message.seq
                        }
                        highlights={highlights}
                        callsMe={callsMe(row.message)}
                        nameOf={nameOf}
                        onReply={onReply}
                        onJump={jumpTo}
                        onForward={onForward}
                        onTogglePin={canPin ? onTogglePin : undefined}
                        pinned={pinnedIds.has(row.message.id)}
                        onSeen={onSeenMention}
                        meId={me.id}
                        deleting={Boolean(deleting[id] && !deleting[id]!.error)}
                        onCopy={onCopy}
                        editSignal={editRequest?.id === id ? editRequest.nonce : 0}
                        onEditEnd={onEditEnd}
                      />
                    )}
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
                <PillButton tone="quiet" size="sm" onClick={() => void loadNewer(activeId)}>
                  {T.loadNewer}
                </PillButton>
              </div>
            )}

            {queue.map((pe, i) => (
              <MessageBubble
                key={pe.id}
                pending={pe}
                mine
                showAuthor={false}
                authorName=""
                // Antrean selalu milik kita sendiri dan berurutan, jadi hanya
                // yang paling depan yang membuka rentetan baru.
                firstOfRun={i === 0}
                gutter={isGroup}
                highlights={highlights}
                nameOf={nameOf}
                onRewrite={onRewrite}
              />
            ))}

            <div ref={bottomRef} />

            {/* Jalan pulang ke pesan terbaru, menempel di tepi bawah area baca:
                selama jendela lama terbuka, dan setiap kali pembaca menggulir
                jauh ke atas — dengan jumlah pesan yang datang sementara itu. */}
            {(viewingOld || history.away) && (
              <div className="pointer-events-none sticky bottom-1 z-10 mt-2 flex justify-center">
                <button
                  onClick={() => (viewingOld ? void returnToLatest(activeId) : history.toBottom())}
                  aria-label={
                    history.unseen > 0
                      ? `${T.newMessages(history.unseen)}, ke pesan terbaru`
                      : T.newMessages(0)
                  }
                  className="pointer-events-auto flex min-h-11 items-center gap-1.5 rounded-full bg-accent px-4 text-[14px] font-semibold text-accent-ink shadow-pop transition hover:brightness-110"
                >
                  <Icon name="bawah" size={16} />
                  {T.newMessages(viewingOld ? 0 : history.unseen)}
                </button>
              </div>
            )}
          </div>
        </div>

        {/* Wilayah status selalu ada, walau kosong: pembaca layar hanya
            mengumumkan perubahan di wilayah yang sudah terdaftar sebelumnya. */}
        <div role="status" aria-live="polite">
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
        </div>
        <p role="status" aria-live="polite" className="sr-only">
          {announcement}
        </p>

        <NoticeStack
          failed={failedDelete ? { id: failedDelete[0], error: failedDelete[1].error! } : null}
          waiting={waiting}
          flash={flash}
          undoRef={undoButton}
          onRetry={retryDelete}
          onKeep={id => {
            dismissDelete(id);
            focusRowOf([id]);
          }}
          onUndo={undoAll}
          onPause={pauseDeletes}
          onResume={resumeDeletes}
        />

        <Composer
          ref={composer}
          conversationId={activeId}
          isGroup={isGroup}
          roster={roster}
          meId={me.id}
          send={send}
          onEditLast={editLast}
          onLeaveToHistory={leaveToHistory}
        />
      </section>

      {forwarding && <ForwardDialog message={forwarding} onClose={() => setForwarding(null)} />}

      {panelOpen && isGroup && (
        <GroupPanel conversation={conversation} onClose={() => setPanelOpen(false)} />
      )}
    </div>
  );
}
