import { useEffect, useLayoutEffect, useRef, useState, type KeyboardEvent } from 'react';
import { useStore } from '../store';
import { conversationTitle } from './Sidebar';
import MessageBubble from './MessageBubble';

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

  const [draft, setDraft] = useState('');
  const scrollRef = useRef<HTMLDivElement>(null);
  const bottomRef = useRef<HTMLDivElement>(null);
  const typingSentAt = useRef(0);
  const [, force] = useState(0);

  const conversation = conversations.find(c => c.id === activeId);
  const list = activeId ? (messages[activeId] ?? []) : [];
  const queue = activeId ? (pending[activeId] ?? []) : [];

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
    if (!body) return;
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

  const peerOnline = conversation.peer ? online.has(conversation.peer.id) : false;

  return (
    <section className="flex min-w-0 flex-1 flex-col">
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

      <div className="border-t border-line bg-surface p-3">
        <div className="flex items-end gap-2">
          <textarea
            value={draft}
            rows={1}
            onChange={e => {
              setDraft(e.target.value);
              notifyTyping();
            }}
            onKeyDown={onKeyDown}
            placeholder="Tulis pesan…  (Enter kirim, Shift+Enter baris baru)"
            className="max-h-32 min-h-10 flex-1 resize-none rounded-xl border border-line bg-canvas px-3 py-2 text-sm outline-none focus:border-accent"
          />
          <button
            onClick={() => void submit()}
            disabled={!draft.trim()}
            className="rounded-xl bg-accent px-4 py-2.5 text-sm font-medium text-white transition hover:opacity-90 disabled:opacity-40"
          >
            Kirim
          </button>
        </div>
      </div>
    </section>
  );
}
