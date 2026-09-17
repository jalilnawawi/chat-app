import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { ApiError, api } from '../api';
import { useStore } from '../store';
import type { SearchHit } from '../types';
import Avatar from './Avatar';
import Icon from './Icon';
import { excerpt } from './PinBar';
import { conversationTitle } from './Sidebar';

/** Jeda sebelum kotak pencarian benar-benar bertanya ke server. */
const DEBOUNCE_MS = 300;

/**
 * Sama dengan minPrefixLen di server: kata sependek ini dicocokkan utuh, dan
 * sorotannya harus mengikuti aturan yang sama — kalau tidak, "ke" tersorot di
 * dalam "kemarin" padahal pesan itu ditemukan karena alasan lain.
 */
const MIN_PREFIX = 3;

/** Panjang cuplikan untuk pesan yang panjang. */
const SNIPPET = 160;

/**
 * Panel pencarian pesan.
 *
 * Satu panel untuk dua cakupan — percakapan yang sedang dibuka, atau semuanya
 * — karena pertanyaannya sama: "di mana kalimat itu?". Cakupannya bisa
 * dipindah tanpa mengetik ulang, dan itu yang paling sering dilakukan orang
 * setelah pencarian pertamanya tidak menemukan apa-apa.
 */
export default function SearchPanel({
  scope,
  onScope,
  onClose,
}: {
  /** null = semua percakapan. */
  scope: string | null;
  onScope: (conversationId: string | null) => void;
  onClose: () => void;
}) {
  const conversations = useStore(s => s.conversations);
  const activeId = useStore(s => s.activeId);
  const jumpTo = useStore(s => s.jumpTo);

  const [text, setText] = useState('');
  const [hits, setHits] = useState<SearchHit[]>([]);
  const [terms, setTerms] = useState<string[]>([]);
  const [cursor, setCursor] = useState<string | undefined>();
  const [state, setState] = useState<'idle' | 'loading' | 'done' | 'error'>('idle');
  const [error, setError] = useState('');
  const [more, setMore] = useState(false);
  const input = useRef<HTMLInputElement>(null);
  // Nomor permintaan terakhir. Jawaban yang datang untuk ketikan yang sudah
  // ditinggalkan dibuang — jaringan tidak menjanjikan urutan, dan hasil untuk
  // "rap" yang tiba setelah hasil untuk "rapat" menimpa jawaban yang benar.
  const latest = useRef(0);

  const scoped = scope ? conversations.find(c => c.id === scope) : undefined;
  // Tawaran "percakapan ini" mengikuti percakapan yang sedang dibuka, supaya
  // panel yang dibuka dari sidebar tetap bisa dipersempit.
  const narrowTo = scope ?? activeId;
  const narrowConv = narrowTo ? conversations.find(c => c.id === narrowTo) : undefined;

  useEffect(() => {
    input.current?.focus();
  }, [scope]);

  useEffect(() => {
    const q = text.trim();
    const id = ++latest.current;
    if (q.length < 2) {
      setHits([]);
      setTerms([]);
      setCursor(undefined);
      setState('idle');
      return;
    }
    setState('loading');
    const t = setTimeout(() => {
      api
        .search(q, { conversationId: scope ?? undefined })
        .then(res => {
          if (id !== latest.current) return;
          setHits(res.hits);
          setTerms(res.terms);
          setCursor(res.nextCursor);
          setState('done');
        })
        .catch(err => {
          if (id !== latest.current) return;
          setError(err instanceof ApiError ? err.message : 'Pencarian gagal');
          setState('error');
        });
    }, DEBOUNCE_MS);
    return () => clearTimeout(t);
  }, [text, scope]);

  async function loadMore() {
    if (!cursor) return;
    const id = latest.current;
    setMore(true);
    try {
      const res = await api.search(text.trim(), { conversationId: scope ?? undefined, cursor });
      if (id !== latest.current) return;
      setHits(prev => [...prev, ...res.hits]);
      setCursor(res.nextCursor);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Pencarian gagal');
      setState('error');
    } finally {
      setMore(false);
    }
  }

  const pattern = useMemo(() => termPattern(terms), [terms]);

  async function open(hit: SearchHit) {
    // Di layar sempit panel ini menutupi percakapan; hasil yang ditekan tidak
    // akan terlihat mendarat kalau panelnya tetap di atasnya.
    if (window.matchMedia('(max-width: 767px)').matches) onClose();
    await jumpTo(hit.message.conversationId, hit.message.id, hit.message.seq);
  }

  return (
    <aside className="fixed inset-0 z-30 flex w-full flex-col bg-surface md:static md:z-auto md:w-96 md:shrink-0 md:border-l md:border-line">
      <header className="flex items-center justify-between border-b border-line px-4 py-3">
        <h3 className="text-[15px] font-bold">Cari pesan</h3>
        <button
          onClick={onClose}
          aria-label="Tutup pencarian"
          className="grid size-9 place-items-center rounded-xl text-muted transition hover:bg-canvas hover:text-ink"
        >
          <Icon name="tutup" size={18} />
        </button>
      </header>

      <div className="border-b border-line px-4 py-3">
        <div className="relative">
          <span className="pointer-events-none absolute top-1/2 left-3.5 -translate-y-1/2 text-muted">
            <Icon name="cari" size={18} />
          </span>
          <input
            ref={input}
            value={text}
            type="search"
            maxLength={200}
            onChange={e => setText(e.target.value)}
            onKeyDown={e => {
              if (e.key === 'Escape') onClose();
            }}
            placeholder={scoped ? `Cari di ${conversationTitle(scoped)}…` : 'Cari di semua percakapan…'}
            aria-label="Kata yang dicari"
            className="w-full rounded-xl border border-line-strong bg-canvas py-2.5 pr-3.5 pl-11 text-[15px] outline-none transition focus:bg-surface"
          />
        </div>

        {narrowConv && (
          <div className="mt-2.5 flex gap-1.5" role="group" aria-label="Cakupan pencarian">
            <Chip on={scope !== null} onClick={() => onScope(narrowConv.id)}>
              {conversationTitle(narrowConv)}
            </Chip>
            <Chip on={scope === null} onClick={() => onScope(null)}>
              Semua percakapan
            </Chip>
          </div>
        )}
      </div>

      <div className="flex-1 overflow-y-auto" aria-live="polite" aria-busy={state === 'loading'}>
        {state === 'idle' && (
          <div className="px-5 py-8 text-center">
            <p className="text-sm font-semibold">Ketik kata yang diingat</p>
            {/* Aturan pencocokannya dikatakan terus terang. Kotak pencarian
                yang diam-diam tidak menemukan "dikirim" saat dicari "kirim"
                membuat orang berhenti percaya; yang mengatakannya di muka
                membuat orang mencoba kata lain. */}
            <p className="mx-auto mt-1.5 max-w-[30ch] text-[13px] leading-relaxed text-muted">
              Kata utuh dan awalannya cocok: “kirim” menemukan “kirimkan”, tapi tidak “dikirim”.
            </p>
          </div>
        )}

        {state === 'loading' && hits.length === 0 && (
          <p className="px-5 py-8 text-center text-sm text-muted">Mencari…</p>
        )}

        {state === 'error' && (
          <p className="m-4 rounded-xl bg-danger-soft px-3.5 py-2.5 text-[13px] text-danger">{error}</p>
        )}

        {state === 'done' && hits.length === 0 && (
          <div className="px-5 py-8 text-center">
            <p className="text-sm font-semibold">Tidak ada yang cocok</p>
            <p className="mt-1.5 text-[13px] text-muted">
              {scope ? 'Coba cari di semua percakapan, atau pakai kata lain.' : 'Coba kata lain.'}
            </p>
          </div>
        )}

        {hits.length > 0 && (
          <ul className="p-2">
            {hits.map(h => {
              const conv = conversations.find(c => c.id === h.message.conversationId);
              return (
                <li key={h.message.id}>
                  <button
                    onClick={() => void open(h)}
                    className="flex w-full gap-3 rounded-xl px-2.5 py-2.5 text-left transition hover:bg-canvas"
                  >
                    <Avatar name={h.senderName} url={h.senderAvatarUrl} size={34} />
                    <span className="min-w-0 flex-1">
                      <span className="flex items-baseline gap-2">
                        <span className="min-w-0 flex-1 truncate text-[13px]">
                          <span className="font-bold">{h.senderName}</span>
                          {!scope && conv && (
                            <span className="text-muted"> · {conversationTitle(conv)}</span>
                          )}
                        </span>
                        <span className="shrink-0 text-[11.5px] text-muted tabular-nums">
                          {tanggal(h.message.createdAt)}
                        </span>
                      </span>
                      <span className="mt-0.5 line-clamp-3 text-[13.5px] leading-snug break-words">
                        {h.message.forwarded && (
                          <span className="text-muted italic">Diteruskan: </span>
                        )}
                        <Marked text={snippet(h.message.body || excerpt(h.message), pattern)} pattern={pattern} />
                      </span>
                    </span>
                  </button>
                </li>
              );
            })}
          </ul>
        )}

        {cursor && state === 'done' && (
          <div className="px-4 pb-4 text-center">
            <button
              onClick={() => void loadMore()}
              disabled={more}
              className="rounded-full border border-line bg-surface px-3.5 py-1.5 text-[13px] font-medium text-muted transition hover:border-accent hover:text-accent-text disabled:opacity-50"
            >
              {more ? 'Memuat…' : 'Hasil lainnya'}
            </button>
          </div>
        )}
      </div>
    </aside>
  );
}

function Chip({ on, onClick, children }: { on: boolean; onClick: () => void; children: ReactNode }) {
  return (
    <button
      onClick={onClick}
      aria-pressed={on}
      className={`max-w-[60%] truncate rounded-full border px-3 py-1 text-[13px] font-semibold transition ${
        on
          ? 'border-accent bg-accent-soft text-accent-text'
          : 'border-line-strong text-muted hover:text-ink'
      }`}
    >
      {children}
    </button>
  );
}

function tanggal(iso: string): string {
  const at = new Date(iso);
  const now = new Date();
  if (at.toDateString() === now.toDateString()) {
    return at.toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit' });
  }
  return at.toLocaleDateString('id-ID', {
    day: 'numeric',
    month: 'short',
    ...(at.getFullYear() === now.getFullYear() ? {} : { year: 'numeric' }),
  });
}

/**
 * Pola sorotan dari kata yang DICOCOKKAN SERVER.
 *
 * Batas kata ditulis sebagai "bukan huruf atau angka" dengan kelas Unicode,
 * bukan `\b`: `\b` di JavaScript hanya mengenal huruf Latin tanpa aksen, dan
 * "café" akan terpotong di tengah.
 */
function termPattern(terms: string[]): RegExp | null {
  if (terms.length === 0) return null;
  const parts = [...terms]
    .sort((a, b) => b.length - a.length)
    .map(t => {
      const e = t.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
      return [...t].length >= MIN_PREFIX ? `${e}[\\p{L}\\p{N}]*` : `${e}(?![\\p{L}\\p{N}])`;
    });
  return new RegExp(`(?<![\\p{L}\\p{N}])(?:${parts.join('|')})`, 'giu');
}

/** Memotong pesan panjang di sekitar kecocokan pertamanya. */
function snippet(text: string, pattern: RegExp | null): string {
  if (text.length <= SNIPPET || !pattern) return text;
  pattern.lastIndex = 0;
  const at = pattern.exec(text)?.index ?? 0;
  pattern.lastIndex = 0;
  const start = Math.max(0, at - 50);
  const end = Math.min(text.length, start + SNIPPET);
  return (start > 0 ? '…' : '') + text.slice(start, end) + (end < text.length ? '…' : '');
}

function Marked({ text, pattern }: { text: string; pattern: RegExp | null }) {
  if (!pattern) return <>{text}</>;
  const out: ReactNode[] = [];
  let last = 0;
  for (const m of text.matchAll(pattern)) {
    if (m.index > last) out.push(text.slice(last, m.index));
    out.push(
      // Teal, bukan mangga: mangga di aplikasi ini hanya berarti "ada yang
      // memanggilmu", dan kata yang kebetulan dicari bukan panggilan.
      <mark key={m.index} className="rounded bg-accent-soft px-0.5 font-bold text-accent-text">
        {m[0]}
      </mark>,
    );
    last = m.index + m[0].length;
  }
  if (last < text.length) out.push(text.slice(last));
  return <>{out}</>;
}
