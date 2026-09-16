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
import type { Message } from '../types';

/** Indikator "sedang mengetik" dianggap basi setelah jeda ini. */
const TYPING_TTL_MS = 4000;

/** Berapa halaman ke belakang yang boleh dimuat saat melompat ke pesan lama. */
const JUMP_MAX_PAGES = 5;

/** Lama sorotan setelah melompat ke sebuah pesan. */
const JUMP_HIGHLIGHT_MS = 2000;

export default function ChatPanel({ send }: { send: (type: string, payload: unknown) => void }) {
  const me = useStore(s => s.me);
  const activeId = useStore(s => s.activeId);
  const conversations = useStore(s => s.conversations);
  const messages = useStore(s => s.messages);
  const pending = useStore(s => s.pending);
  const hasMore = useStore(s => s.hasMore);
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

  const [draft, setDraft] = useState('');
  const [dragging, setDragging] = useState(false);
  const [mentionQuery, setMentionQuery] = useState<{ at: number; text: string } | null>(null);
  const [mentionIndex, setMentionIndex] = useState(0);
  const [jumped, setJumped] = useState<string | null>(null);
  const [panelOpen, setPanelOpen] = useState(false);
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
    bottomRef.current?.scrollIntoView({ block: 'end' });
  }, [activeId]);

  const nameOf = useCallback(
    (userId: string) =>
      roster.find(m => m.userId === userId)?.displayName ??
      (userId === me?.id ? (me?.displayName ?? 'Saya') : 'Seseorang'),
    [roster, me],
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
   * Melompat ke pesan yang dikutip.
   *
   * Pesannya bisa saja belum termuat — kutipan bertahan selamanya, sedangkan
   * riwayat dimuat sehalaman demi sehalaman. Jadi halaman-halaman lama ditarik
   * dulu sampai pesannya ketemu, dengan batas: percakapan yang sudah puluhan
   * ribu pesan tidak boleh menarik semuanya hanya karena seseorang menekan
   * sebuah kutipan tua.
   */
  const jumpTo = useCallback(
    async (messageId: string) => {
      const reveal = () => {
        const el = document.getElementById(`msg-${messageId}`);
        if (!el) return false;
        el.scrollIntoView({ block: 'center', behavior: 'smooth' });
        setJumped(messageId);
        return true;
      };

      if (reveal()) return;
      if (!activeId) return;

      for (let page = 0; page < JUMP_MAX_PAGES; page++) {
        if (!useStore.getState().hasMore[activeId]) break;
        await loadOlder(activeId);
        // Menunggu satu frame: baris barunya baru ada di DOM setelah React
        // sempat menggambar, dan getElementById membaca DOM, bukan state.
        await new Promise(requestAnimationFrame);
        if (reveal()) return;
      }
    },
    [activeId, loadOlder],
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
    return (
      <section className="grid flex-1 place-items-center text-sm text-muted">
        Pilih percakapan untuk mulai.
      </section>
    );
  }

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
      className="relative flex min-w-0 flex-1 flex-col"
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
        <div className="pointer-events-none absolute inset-3 z-10 grid place-items-center rounded-2xl border-2 border-dashed border-accent bg-accent-soft/80 text-sm font-medium text-accent">
          Lepaskan untuk melampirkan
        </div>
      )}

      <header className="flex items-center gap-3 border-b border-line bg-surface px-5 py-3">
        <Avatar
          name={conversationTitle(conversation)}
          url={conversation.peer?.avatarUrl}
          size={36}
          dot={
            conversation.type === 'direct' ? dotFor(peerOnline, peerStatus?.status) : undefined
          }
        />
        <div className="min-w-0 flex-1">
          <h2 className="truncate text-sm font-semibold">{conversationTitle(conversation)}</h2>
          <p className="truncate text-xs text-muted">
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

        {/* Hanya grup yang punya pengelolaan. DM tidak bisa ditambahi orang —
            lihat catatan kebocoran di server/internal/store/group.go — jadi
            tombolnya memang tidak ada di sana, bukan ada tapi menolak. */}
        {conversation.type === 'group' && (
          <button
            onClick={() => setPanelOpen(v => !v)}
            aria-label="Kelola grup"
            title="Kelola grup"
            aria-pressed={panelOpen}
            className={`grid size-9 shrink-0 place-items-center rounded-xl border text-muted transition hover:border-accent hover:text-ink ${
              panelOpen ? 'border-accent bg-accent-soft text-ink' : 'border-line'
            }`}
          >
            ⚙
          </button>
        )}
      </header>

      <div ref={scrollRef} className="flex-1 overflow-y-auto px-5 py-4">
        {/* Pesan ditumpuk dari bawah: percakapan yang masih sedikit tetap
            menempel di dekat kolom tulis, bukan mengambang di atas. */}
        <div className="flex min-h-full flex-col justify-end">
          {hasMore[activeId] && (
            <div className="mb-4 text-center">
              <button
                onClick={() => void loadOlder(activeId)}
                className="rounded-full border border-line px-3 py-1 text-xs text-muted transition hover:text-ink"
              >
                Muat pesan lama
              </button>
            </div>
          )}

          {list.map((m, i) => (
            <div
              key={m.id}
              className={`rounded-2xl transition-colors ${
                jumped === m.id ? 'bg-accent-soft/70' : ''
              }`}
            >
              <MessageBubble
                message={m}
                mine={m.senderId === me.id}
                showAuthor={conversation.type === 'group' && list[i - 1]?.senderId !== m.senderId}
                authorName={nameOf(m.senderId)}
                readByPeer={conversation.type === 'direct' && m.senderId === me.id && peerRead >= m.seq}
                highlights={highlights}
                callsMe={callsMe(m)}
                nameOf={nameOf}
                onReply={onReply}
                onJump={id => void jumpTo(id)}
                onSeen={onSeenMention}
              />
            </div>
          ))}

          {queue.map(p => (
            <MessageBubble
              key={p.id}
              pending={p}
              mine
              showAuthor={false}
              authorName=""
              highlights={highlights}
              nameOf={nameOf}
            />
          ))}

          <div ref={bottomRef} />
        </div>
      </div>

      {typers.length > 0 && (
        <p className="px-5 pb-1 text-xs text-muted">{typers.join(', ')} sedang mengetik…</p>
      )}

      <UploadStrip conversationId={activeId} />

      {replying && (
        <div className="flex items-start gap-2 border-t border-line bg-canvas px-4 py-2 text-xs">
          <span className="mt-0.5 text-muted">↩</span>
          <div className="min-w-0 flex-1">
            <p className="font-medium text-muted">Membalas {nameOf(replying.senderId)}</p>
            <p className="truncate text-muted/80">
              {replying.body || '📎 Lampiran'}
            </p>
          </div>
          <button
            onClick={() => setReplyTo(activeId, null)}
            aria-label="Batalkan balasan"
            className="rounded px-1 text-muted transition hover:text-ink"
          >
            ✕
          </button>
        </div>
      )}

      <div className="relative border-t border-line bg-surface p-3">
        {mentionQuery && candidates.length > 0 && (
          <div className="absolute bottom-full left-3 z-20 mb-1 w-64 overflow-hidden rounded-xl border border-line bg-surface shadow-lg">
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
                className={`block w-full px-3 py-2 text-left text-sm transition ${
                  i === mentionIndex ? 'bg-accent-soft' : 'hover:bg-canvas'
                }`}
              >
                {c.all ? (
                  <>
                    <span className="font-medium">@semua</span>
                    <span className="ml-2 text-xs text-muted">membangunkan seluruh anggota</span>
                  </>
                ) : (
                  <span className="font-medium">{c.name}</span>
                )}
              </button>
            ))}
          </div>
        )}

        <div className="flex items-end gap-2">
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
                className="grid size-10 shrink-0 place-items-center rounded-xl border border-line text-muted transition hover:border-accent hover:text-ink"
              >
                📎
              </button>
            </>
          )}

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
            placeholder={
              conversation.type === 'group'
                ? 'Tulis pesan…  (@ untuk menyebut, Enter kirim)'
                : 'Tulis pesan…  (Enter kirim, Shift+Enter baris baru)'
            }
            className="max-h-32 min-h-10 flex-1 resize-none rounded-xl border border-line bg-canvas px-3 py-2 text-sm outline-none focus:border-accent"
          />
          <button
            onClick={() => void submit()}
            // Unggahan yang belum selesai menahan tombol kirim. Melepasnya
            // lebih awal akan mengirim pesan TANPA lampiran yang jelas-jelas
            // terlihat di layar — kegagalan yang diam dan membingungkan.
            disabled={(!draft.trim() && !attachable) || uploading}
            className="rounded-xl bg-accent px-4 py-2.5 text-sm font-medium text-white transition hover:opacity-90 disabled:opacity-40"
          >
            {uploading ? 'Mengunggah…' : 'Kirim'}
          </button>
        </div>
      </div>
    </section>

      {panelOpen && conversation.type === 'group' && (
        <GroupPanel conversation={conversation} onClose={() => setPanelOpen(false)} />
      )}
    </div>
  );
}
