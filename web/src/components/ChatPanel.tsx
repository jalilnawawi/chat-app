import {
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type ClipboardEvent,
  type DragEvent,
  type KeyboardEvent,
} from 'react';
import { useStore } from '../store';
import { conversationTitle } from './Sidebar';
import MessageBubble from './MessageBubble';
import UploadStrip from './UploadStrip';

/** Indikator "sedang mengetik" dianggap basi setelah jeda ini. */
const TYPING_TTL_MS = 4000;

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
  const sendMessage = useStore(s => s.sendMessage);
  const loadOlder = useStore(s => s.loadOlder);
  const uploads = useStore(s => s.uploads);
  const addFiles = useStore(s => s.addFiles);

  const [draft, setDraft] = useState('');
  const [dragging, setDragging] = useState(false);
  const scrollRef = useRef<HTMLDivElement>(null);
  const bottomRef = useRef<HTMLDivElement>(null);
  const fileInput = useRef<HTMLInputElement>(null);
  const typingSentAt = useRef(0);
  const [, force] = useState(0);

  const conversation = conversations.find(c => c.id === activeId);
  const list = activeId ? (messages[activeId] ?? []) : [];
  const queue = activeId ? (pending[activeId] ?? []) : [];
  const drafts = activeId ? (uploads[activeId] ?? []) : [];
  const uploading = drafts.some(u => u.status === 'uploading');
  const attachable = drafts.some(u => u.status === 'ready');

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

  // Pindah ruang: langsung ke pesan terbaru.
  useLayoutEffect(() => {
    lastCount.current = 0;
    bottomRef.current?.scrollIntoView({ block: 'end' });
  }, [activeId]);

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

  async function submit() {
    const body = draft.trim();
    // Pesan boleh tanpa teks asalkan ada lampiran yang sudah selesai diunggah.
    if (!body && !attachable) return;

    setDraft('');
    typingSentAt.current = 0;
    send('typing', { conversationId: activeId, typing: false });
    await sendMessage(activeId!, body);
  }

  function onKeyDown(e: KeyboardEvent<HTMLTextAreaElement>) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      void submit();
    }
  }

  function pick(files: FileList | null) {
    if (!files || files.length === 0 || !activeId) return;
    addFiles(activeId, Array.from(files));
  }

  // Menempel gambar langsung dari papan klip — cara paling cepat mengirim
  // tangkapan layar, dan yang paling sering dicoba orang tanpa diberi tahu.
  function onPaste(e: ClipboardEvent<HTMLTextAreaElement>) {
    const files = Array.from(e.clipboardData.files);
    if (files.length === 0) return;
    e.preventDefault();
    if (activeId) addFiles(activeId, files);
  }

  function onDrop(e: DragEvent<HTMLElement>) {
    e.preventDefault();
    setDragging(false);
    pick(e.dataTransfer.files);
  }

  const peerOnline = conversation.peer ? online.has(conversation.peer.id) : false;

  return (
    <section
      className="relative flex min-w-0 flex-1 flex-col"
      // dragenter/dragover harus di-preventDefault, kalau tidak browser
      // membuka berkasnya sendiri dan meninggalkan halaman ini sepenuhnya.
      onDragEnter={e => {
        e.preventDefault();
        if (e.dataTransfer.types.includes('Files')) setDragging(true);
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
        <div className="min-w-0">
          <h2 className="truncate text-sm font-semibold">{conversationTitle(conversation)}</h2>
          <p className="text-xs text-muted">
            {conversation.type === 'group'
              ? `${(members[activeId] ?? []).length} anggota`
              : peerOnline
                ? 'Online'
                : 'Offline'}
          </p>
        </div>
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
          <MessageBubble
            key={m.id}
            message={m}
            mine={m.senderId === me.id}
            showAuthor={conversation.type === 'group' && list[i - 1]?.senderId !== m.senderId}
            authorName={
              (members[activeId] ?? []).find(x => x.userId === m.senderId)?.displayName ?? 'Seseorang'
            }
            readByPeer={conversation.type === 'direct' && m.senderId === me.id && peerRead >= m.seq}
          />
        ))}

        {queue.map(p => (
          <MessageBubble key={p.id} pending={p} mine showAuthor={false} authorName="" />
        ))}

        <div ref={bottomRef} />
        </div>
      </div>

      {typers.length > 0 && (
        <p className="px-5 pb-1 text-xs text-muted">
          {typers.join(', ')} sedang mengetik…
        </p>
      )}

      <UploadStrip conversationId={activeId} />

      <div className="border-t border-line bg-surface p-3">
        <div className="flex items-end gap-2">
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

          <textarea
            value={draft}
            rows={1}
            onChange={e => {
              setDraft(e.target.value);
              notifyTyping();
            }}
            onPaste={onPaste}
            onKeyDown={onKeyDown}
            placeholder="Tulis pesan…  (Enter kirim, Shift+Enter baris baru)"
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
  );
}
