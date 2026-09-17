import { useEffect, useRef, useState } from 'react';
import { useStore } from '../store';
import type { Message, Pin } from '../types';
import Icon from './Icon';
import { formatDuration } from './VoicePlayer';

const kosong: Pin[] = [];

/** Satu baris untuk sebuah pesan: teksnya, atau apa yang dilampirkannya. */
export function excerpt(m: Message): string {
  if (m.body) return m.body;
  const n = m.attachments.length;
  if (n > 1) return `📎 ${n} lampiran`;
  const a = m.attachments[0];
  if (!a) return 'Pesan';
  if (a.mime.startsWith('image/')) return '📷 Gambar';
  if (a.mime.startsWith('video/')) return '🎬 Video';
  if (a.durationMs !== undefined) return `🎤 Pesan suara ${formatDuration(a.durationMs)}`;
  if (a.mime.startsWith('audio/')) return '🎵 Rekaman suara';
  return `📎 ${a.name}`;
}

/**
 * Bilah sematan di bawah kepala percakapan.
 *
 * Yang tampil selalu cuma SATU baris — sematan terbaru — dan daftar lengkapnya
 * dibuka atas permintaan. Bilah yang tumbuh setiap kali sesuatu disematkan
 * akan memakan ruang yang justru dipakai untuk membaca percakapannya.
 *
 * Tidak dirender sama sekali bila tidak ada sematan: bilah kosong bertuliskan
 * "belum ada pesan disematkan" adalah iklan untuk fitur, bukan informasi.
 */
export default function PinBar({
  conversationId,
  canPin,
  nameOf,
  onJump,
}: {
  conversationId: string;
  canPin: boolean;
  nameOf: (userId: string) => string;
  onJump: (m: Message) => void;
}) {
  const pins = useStore(s => s.pins[conversationId] ?? kosong);
  const setPinned = useStore(s => s.setPinned);
  const [open, setOpen] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const boxRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    setOpen(false);
    setError(null);
  }, [conversationId]);

  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => {
      if (!boxRef.current?.contains(e.target as Node)) setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false);
    };
    document.addEventListener('mousedown', onDown);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('mousedown', onDown);
      document.removeEventListener('keydown', onKey);
    };
  }, [open]);

  if (pins.length === 0) return null;
  const top = pins[0]!;

  async function unpin(m: Message) {
    setError(null);
    try {
      await setPinned(m, false);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Gagal melepas sematan');
    }
  }

  return (
    <div ref={boxRef} className="relative z-10 border-b border-line bg-surface">
      <div className="flex items-center gap-2 px-3 py-2 md:px-4">
        <span className="shrink-0 text-accent-text">
          <Icon name="sematan" size={17} />
        </span>
        <button
          onClick={() => onJump(top.message)}
          className="min-w-0 flex-1 text-left"
          title="Lompat ke pesan yang disematkan"
        >
          <span className="block text-[12px] font-bold text-accent-text">
            {pins.length > 1 ? `Disematkan · ${pins.length} pesan` : 'Disematkan'}
          </span>
          <span className="block truncate text-[13px]">
            <span className="font-semibold">{nameOf(top.message.senderId)}:</span>{' '}
            {excerpt(top.message)}
          </span>
        </button>
        {(pins.length > 1 || canPin) && (
          <button
            onClick={() => setOpen(v => !v)}
            aria-expanded={open}
            className="shrink-0 rounded-lg px-2.5 py-1.5 text-[13px] font-semibold text-muted transition hover:bg-canvas hover:text-ink"
          >
            {pins.length > 1 ? 'Semua' : 'Atur'}
          </button>
        )}
      </div>

      {open && (
        <div className="absolute inset-x-2 top-full mt-1 max-h-[60vh] overflow-y-auto rounded-2xl border border-line bg-surface p-1.5 shadow-pop md:inset-x-4">
          {error && (
            <p className="mx-1 mb-1.5 rounded-lg bg-danger-soft px-2.5 py-1.5 text-[13px] text-danger">
              {error}
            </p>
          )}
          <ul>
            {pins.map(p => (
              <li key={p.message.id} className="flex items-start gap-1 rounded-xl hover:bg-canvas">
                <button
                  onClick={() => {
                    setOpen(false);
                    onJump(p.message);
                  }}
                  className="min-w-0 flex-1 px-2.5 py-2 text-left"
                >
                  <span className="block text-[13px]">
                    <span className="font-semibold">{nameOf(p.message.senderId)}</span>
                    <span className="text-muted">
                      {' · '}
                      {new Date(p.message.createdAt).toLocaleDateString('id-ID', {
                        day: 'numeric',
                        month: 'short',
                      })}
                    </span>
                  </span>
                  <span className="line-clamp-2 text-[13px] text-muted">{excerpt(p.message)}</span>
                  {p.pinnedByName && (
                    <span className="mt-0.5 block text-[11.5px] text-muted">
                      Disematkan {p.pinnedByName}
                    </span>
                  )}
                </button>
                {canPin && (
                  <button
                    onClick={() => void unpin(p.message)}
                    aria-label="Lepas sematan"
                    title="Lepas sematan"
                    className="m-1.5 grid size-8 shrink-0 place-items-center rounded-lg text-muted transition hover:bg-danger-soft hover:text-danger"
                  >
                    <Icon name="tutup" size={15} />
                  </button>
                )}
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}
