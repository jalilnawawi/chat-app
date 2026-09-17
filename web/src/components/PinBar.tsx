import { useEffect, useRef, useState } from 'react';
import { useStore } from '../store';
import type { Message, Pin } from '../types';
import Icon from './Icon';
import { excerpt } from '../format';

const kosong: Pin[] = [];

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
  const toggleRef = useRef<HTMLButtonElement>(null);
  const listRef = useRef<HTMLDivElement>(null);

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
      if (e.key !== 'Escape') return;
      setOpen(false);
      // Fokus kembali ke tombol yang membukanya, bukan hilang ke halaman.
      if (listRef.current?.contains(document.activeElement)) toggleRef.current?.focus();
    };
    // Daftar yang dibuka lewat papan ketik langsung menerima fokus di butir
    // pertamanya; pembaca layar tahu dia sudah di dalam daftar.
    listRef.current?.querySelector<HTMLButtonElement>('button')?.focus({ preventScroll: true });
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
      <div className="flex items-center gap-2 px-3 py-1 md:px-4 md:py-2">
        <span className="shrink-0 text-accent-text">
          <Icon name="sematan" size={17} />
        </span>
        <button
          onClick={() => onJump(top.message)}
          className="min-h-10 min-w-0 flex-1 text-left"
          title="Lompat ke pesan yang disematkan"
        >
          {/* Satu baris di semua layar: kepala percakapan dan bilah sematan
              tidak boleh memakan ruang baca sebelum satu pesan pun terlihat,
              dan isi lengkapnya tinggal satu ketukan. Di layar lebar judulnya
              duduk di depan kalimat; di ponsel hanya untuk pembaca layar. */}
          <span className="block truncate text-[14px]">
            {/* Dua elemen, bukan `md:not-sr-only`: yang terakhir mengembalikan
                `white-space: normal` dan memotong judul ke baris sendiri. */}
            <span className="sr-only md:hidden">Disematkan:</span>
            <span className="mr-1.5 hidden font-bold text-accent-text md:inline">
              {pins.length > 1 ? `Disematkan (${pins.length})` : 'Disematkan'}
            </span>
            <span className="font-semibold">{nameOf(top.message.senderId)}:</span>{' '}
            {excerpt(top.message)}
          </span>
        </button>
        {(pins.length > 1 || canPin) && (
          <button
            ref={toggleRef}
            onClick={() => setOpen(v => !v)}
            aria-expanded={open}
            aria-label={pins.length > 1 ? `Lihat semua ${pins.length} sematan` : 'Lihat sematan'}
            className="min-h-10 shrink-0 rounded-xl px-3 text-[14px] font-semibold text-muted transition hover:bg-canvas hover:text-ink"
          >
            {/* "Atur" tidak mengatakan apa yang diatur, dan "kelola" terdengar
                seperti menu pengaturan. Yang dibuka memang daftar untuk dilihat. */}
            {pins.length > 1 ? (
              <>
                <span className="md:hidden">Semua ({pins.length})</span>
                <span className="hidden md:inline">Lihat semua ({pins.length})</span>
              </>
            ) : (
              <>
                <span className="md:hidden">Lihat</span>
                <span className="hidden md:inline">Lihat sematan</span>
              </>
            )}
          </button>
        )}
      </div>

      {open && (
        <div
          ref={listRef}
          role="region"
          aria-label="Pesan yang disematkan"
          className="absolute inset-x-2 top-full mt-1 max-h-[60vh] overflow-y-auto rounded-2xl border border-line bg-surface p-1.5 shadow-pop md:inset-x-4"
        >
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
                    className="m-1 grid size-10 shrink-0 place-items-center rounded-xl text-muted transition hover:bg-danger-soft hover:text-danger"
                  >
                    <Icon name="tutup" size={16} />
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
