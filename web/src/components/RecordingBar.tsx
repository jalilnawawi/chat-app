import { useEffect, useRef, useState } from 'react';
import Icon from './Icon';
import PillButton from './ui/PillButton';
import { formatDuration } from '../format';
import { MAX_RECORDING_MS } from '../useRecorder';

/** Rekaman di bawah panjang ini dibuang tanpa bertanya — belum ada yang hilang. */
const DISCARD_WITHOUT_ASKING_MS = 5000;

/**
 * Selama ini, bilah menulis "Mendengarkan…" alih-alih batang suara. Batang
 * yang masih datar di detik pertama terbaca seperti mikrofon yang mati.
 */
const LISTENING_HINT_MS = 1500;

/** Berapa lama "Buang?" menunggu ketukan kedua sebelum kembali tenang. */
const DISCARD_ARM_MS = 4000;

/**
 * Bilah yang menggantikan kolom tulis selama merekam.
 *
 * Tiga hal saja: buang, seberapa lama, kirim. Batang-batang di tengah adalah
 * tingkat suara yang benar-benar tertangkap, bukan hiasan — mikrofon yang
 * dibisukan dari perangkatnya terlihat sebagai garis datar, sebelum lima menit
 * sunyi terlanjur terkirim.
 */
export default function RecordingBar({
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

  // Membuang rekaman yang panjang butuh dua langkah. Satu tombol Escape —
  // yang juga dipakai untuk menutup papan emoji dan membatalkan balasan —
  // tidak boleh menghapus lima menit bicara.
  const [armed, setArmed] = useState(false);
  const cancelRef = useRef<() => void>(() => {});
  cancelRef.current = () => {
    if (starting || armed || elapsed < DISCARD_WITHOUT_ASKING_MS) onCancel();
    else setArmed(true);
  };

  useEffect(() => {
    if (!armed) return;
    const t = setTimeout(() => setArmed(false), DISCARD_ARM_MS);
    return () => clearTimeout(t);
  }, [armed]);

  useEffect(() => {
    const onKey = (e: globalThis.KeyboardEvent) => {
      if (e.key === 'Escape' && !e.defaultPrevented) cancelRef.current();
    };
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, []);

  return (
    <div role="group" aria-label="Merekam pesan suara" className="flex items-center gap-2">
      <p role="status" className="sr-only">
        {starting
          ? 'Menunggu izin mikrofon'
          : armed
            ? 'Tekan buang sekali lagi untuk membuang rekaman'
            : 'Sedang merekam'}
      </p>
      <button
        onClick={() => cancelRef.current()}
        aria-label={armed ? 'Yakin buang rekaman' : 'Buang rekaman'}
        title="Buang rekaman (Esc)"
        className={`flex h-11 shrink-0 items-center justify-center gap-1.5 rounded-full font-semibold transition ${
          armed
            ? 'bg-danger px-3.5 text-surface'
            : 'w-11 text-danger hover:bg-danger-soft'
        }`}
      >
        <Icon name="hapus" />
        {armed && <span className="text-[14px]">Buang?</span>}
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
            {elapsed < LISTENING_HINT_MS && (
              <span className="min-w-0 flex-1 truncate text-[14px] text-muted">Mendengarkan…</span>
            )}
            <span
              aria-hidden
              className={`h-6 min-w-0 flex-1 items-center justify-end gap-[3px] overflow-hidden ${
                elapsed < LISTENING_HINT_MS ? 'hidden' : 'flex'
              }`}
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

      <PillButton icon="kirim" compact onClick={onSend} disabled={starting} aria-label="Kirim pesan suara">
        Kirim
      </PillButton>
    </div>
  );
}
