import { useEffect, useMemo, useState } from 'react';
import { MAX_FORWARD_TARGETS, useStore, type ForwardOutcome } from '../store';
import type { Message } from '../types';
import Avatar from './Avatar';
import Icon from './Icon';
import { conversationTitle } from './Sidebar';

/**
 * Meneruskan satu pesan ke beberapa percakapan.
 *
 * Pilihannya dibatasi lima. Itu bukan batas teknis — tiap tujuan adalah satu
 * pengiriman biasa — melainkan batas yang sama dengan kelonggaran kuota
 * terusan di server, dan batas yang dipakai aplikasi chat besar untuk
 * memperlambat pesan berantai. Tombol yang membiarkan orang memilih dua puluh
 * lalu menolak lima belas di antaranya adalah tombol yang berbohong.
 */
export default function ForwardDialog({
  message,
  onClose,
}: {
  message: Message;
  onClose: () => void;
}) {
  const conversations = useStore(s => s.conversations);
  const forwardMessage = useStore(s => s.forwardMessage);

  const [query, setQuery] = useState('');
  const [picked, setPicked] = useState<string[]>([]);
  const [busy, setBusy] = useState(false);
  const [outcome, setOutcome] = useState<ForwardOutcome[] | null>(null);

  const shown = useMemo(() => {
    const q = query.trim().toLowerCase();
    return q
      ? conversations.filter(c => conversationTitle(c).toLowerCase().includes(q))
      : conversations;
  }, [conversations, query]);

  const full = picked.length >= MAX_FORWARD_TARGETS;

  function toggle(id: string) {
    setPicked(prev =>
      prev.includes(id) ? prev.filter(x => x !== id) : full ? prev : [...prev, id],
    );
  }

  async function submit() {
    if (picked.length === 0) return;
    setBusy(true);
    try {
      const got = await forwardMessage(message.id, picked);
      // Semua berhasil: selesai, tanpa laporan. Laporan hanya layak ada kalau
      // ada yang perlu diketahui.
      if (got.every(o => o.ok)) onClose();
      else setOutcome(got);
    } finally {
      setBusy(false);
    }
  }

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, [onClose]);

  const titleOf = (id: string) => {
    const c = conversations.find(x => x.id === id);
    return c ? conversationTitle(c) : 'Percakapan';
  };

  const preview =
    message.body ||
    (message.attachments.length > 1
      ? `📎 ${message.attachments.length} lampiran`
      : message.attachments[0]?.name || 'Lampiran');

  return (
    <div
      className="fixed inset-0 z-40 flex items-end justify-center bg-ink/40 p-0 backdrop-blur-[2px] sm:items-center sm:p-4"
      onClick={onClose}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-label="Teruskan pesan"
        className="flex max-h-[85vh] w-full flex-col rounded-t-[24px] border border-line bg-surface p-5 shadow-pop sm:max-w-md sm:rounded-[24px]"
        onClick={e => e.stopPropagation()}
      >
        <h2 className="text-lg font-bold tracking-tight">Teruskan pesan</h2>

        <p className="mt-3 line-clamp-3 rounded-xl border-l-[3px] border-accent bg-canvas px-3 py-2 text-[13px] text-muted">
          {preview}
        </p>
        {/* Dikatakan di muka: orang perlu tahu apa yang IKUT dan apa yang
            TIDAK sebelum menekan tombolnya, bukan sesudahnya. */}
        <p className="mt-2 text-[12px] leading-relaxed text-muted">
          Penerima melihat isinya dengan tanda “Diteruskan”, tanpa nama penulis aslinya.
        </p>

        {outcome ? (
          <>
            <ul className="mt-4 flex flex-col gap-1.5">
              {outcome.map(o => (
                <li key={o.conversationId} className="flex items-baseline gap-2 text-sm">
                  <span className={`font-bold ${o.ok ? 'text-ok' : 'text-danger'}`}>
                    {o.ok ? 'Terkirim' : 'Gagal'}
                  </span>
                  <span className="min-w-0 flex-1 truncate">{titleOf(o.conversationId)}</span>
                  {o.error && <span className="shrink-0 text-[12px] text-muted">{o.error}</span>}
                </li>
              ))}
            </ul>
            <div className="mt-4 flex justify-end">
              <button
                onClick={onClose}
                className="rounded-xl bg-accent px-4 py-2.5 text-sm font-semibold text-accent-ink transition hover:brightness-110"
              >
                Tutup
              </button>
            </div>
          </>
        ) : (
          <>
            <div className="relative mt-4">
              <span className="pointer-events-none absolute top-1/2 left-3.5 -translate-y-1/2 text-muted">
                <Icon name="cari" size={18} />
              </span>
              <input
                autoFocus
                value={query}
                onChange={e => setQuery(e.target.value)}
                placeholder="Cari percakapan…"
                className="w-full rounded-xl border border-line-strong bg-canvas py-2.5 pr-3.5 pl-11 text-[15px] outline-none transition focus:bg-surface"
              />
            </div>

            <p className="mt-2 text-[12px] text-muted" aria-live="polite">
              {full
                ? `Paling banyak ${MAX_FORWARD_TARGETS} percakapan sekaligus`
                : `${picked.length} dari ${MAX_FORWARD_TARGETS} dipilih`}
            </p>

            <ul className="-mx-1 mt-2 flex-1 overflow-y-auto px-1">
              {shown.length === 0 && (
                <li className="py-8 text-center text-sm text-muted">Tidak ada percakapan ditemukan.</li>
              )}
              {shown.map(c => {
                const on = picked.includes(c.id);
                const off = !on && full;
                return (
                  <li key={c.id}>
                    <button
                      onClick={() => toggle(c.id)}
                      disabled={off}
                      aria-pressed={on}
                      className={`mb-0.5 flex w-full items-center gap-3 rounded-xl px-2.5 py-2 text-left transition disabled:opacity-40 ${
                        on ? 'bg-accent-soft' : 'hover:bg-canvas'
                      }`}
                    >
                      <Avatar
                        name={conversationTitle(c)}
                        url={c.peer?.avatarUrl}
                        size={36}
                        grup={c.type === 'group'}
                      />
                      <span className="min-w-0 flex-1 truncate text-[15px] font-semibold">
                        {conversationTitle(c)}
                      </span>
                      <span
                        className={`grid size-6 shrink-0 place-items-center rounded-lg border-2 transition ${
                          on
                            ? 'border-accent bg-accent text-accent-ink'
                            : 'border-line-strong text-transparent'
                        }`}
                      >
                        <Icon name="terbaca" size={13} />
                      </span>
                    </button>
                  </li>
                );
              })}
            </ul>

            <div className="mt-4 flex justify-end gap-2">
              <button
                onClick={onClose}
                className="rounded-xl px-4 py-2.5 text-sm font-medium text-muted transition hover:bg-canvas hover:text-ink"
              >
                Batal
              </button>
              <button
                onClick={() => void submit()}
                disabled={picked.length === 0 || busy}
                className="flex items-center gap-2 rounded-xl bg-accent px-4 py-2.5 text-sm font-semibold text-accent-ink transition hover:brightness-110 disabled:opacity-40 disabled:hover:brightness-100"
              >
                <Icon name="teruskan" size={16} />
                {busy ? 'Meneruskan…' : picked.length > 1 ? `Teruskan (${picked.length})` : 'Teruskan'}
              </button>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
