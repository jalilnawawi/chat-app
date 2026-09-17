import { useEffect, useState, type Ref } from 'react';
import Icon from './Icon';
import PillButton from './ui/PillButton';
import { T } from '../teks';
import { DELETE_GRACE_MS, type PendingDelete } from '../store';

export type WaitingDelete = PendingDelete & { id: string; text: string };

/**
 * Kabar setelah menekan hapus, dengan jalan kembali.
 *
 * Hapus berlaku untuk semua orang. Permintaannya ditahan beberapa detik di
 * store, dan selama itu kabar ini yang menawarkan "Urungkan" — lebih ringan
 * daripada dialog "Yakin?" yang harus dijawab setiap kali, dan tetap
 * menyelamatkan ketukan yang meleset.
 *
 * Kalimatnya "menghapus", bukan "dihapus": server belum melakukannya, dan
 * hitung mundurnya mengatakan sampai kapan pilihan itu masih terbuka. Selama
 * kabarnya disentuh kursor atau difokus lewat papan ketik, hitung mundurnya
 * berhenti — membaca kalimat tidak boleh menghabiskan waktu untuk membatalkan.
 * Beberapa hapus sekaligus digabung jadi satu kabar dan satu tombol.
 *
 * Kabar ini tidak mengumumkan dirinya sendiri: ChatPanel memakai wilayah
 * status yang selalu terpasang, karena wilayah yang lahir bersama teksnya
 * sering dilewatkan pembaca layar.
 */
export default function DeleteNotice({
  waiting,
  undoRef,
  onUndo,
  onPause,
  onResume,
}: {
  waiting: WaitingDelete[];
  undoRef: Ref<HTMLButtonElement>;
  onUndo: () => void;
  onPause: () => void;
  onResume: () => void;
}) {
  const [now, setNow] = useState(() => Date.now());
  const paused = waiting.some(w => w.remaining !== undefined);
  useEffect(() => {
    if (paused) return;
    const t = setInterval(() => setNow(Date.now()), 250);
    return () => clearInterval(t);
  }, [paused]);

  const left = Math.max(
    0,
    ...waiting.map(w => (w.remaining !== undefined ? w.remaining : w.due - now)),
  );
  const secondsLeft = Math.ceil(left / 1000);
  const progress = Math.min(1, left / DELETE_GRACE_MS);
  const single = waiting.length === 1 ? waiting[0]!.text : '';

  return (
    <div
      className="relative border-t border-line bg-surface"
      onMouseEnter={onPause}
      onMouseLeave={onResume}
      onFocus={e => {
        // Fokus yang dipindah aplikasi ke "Urungkan" setelah hapus bukan
        // pilihan orangnya; menjeda karenanya berarti hapus tidak pernah
        // terjadi selama fokusnya tidak dipindah — dan "8 detik" jadi bohong.
        if (e.target.dataset.autofocus) {
          delete e.target.dataset.autofocus;
          return;
        }
        if (e.target.matches(':focus-visible')) onPause();
      }}
      onBlur={e => {
        if (!e.currentTarget.contains(e.relatedTarget as Node | null)) onResume();
      }}
    >
      {/* Sisa waktu sebagai garis yang memendek — digambar ulang tiap
          seperempat detik, bukan dianimasikan, jadi tetap jujur saat gerak
          dikurangi. */}
      <span
        aria-hidden
        className="absolute top-0 left-0 h-0.5 bg-danger"
        style={{ width: `${progress * 100}%` }}
      />
      <div className="flex items-center gap-3 px-4 py-1.5 text-[14px] text-ink">
        <Icon name="hapus" size={18} className="shrink-0 text-muted" />
        <p className="min-w-0 flex-1 truncate">
          <span className="font-semibold">
            {waiting.length > 1 ? T.deletingMany(waiting.length) : T.deleting}
          </span>{' '}
          {single && <span className="text-muted">“{single}”</span>}
        </p>
        <span className="shrink-0 text-[13px] text-muted tabular-nums" aria-hidden>
          {paused ? T.paused : T.secondsLeft(secondsLeft)}
        </span>
        <PillButton ref={undoRef} tone="outline" size="sm" onClick={onUndo}>
          {T.undo}
        </PillButton>
      </div>
    </div>
  );
}

/** Hapus yang ditolak server: sebabnya, coba lagi, atau biarkan pesannya. */
export function DeleteFailed({
  error,
  onRetry,
  onKeep,
}: {
  error: string;
  onRetry: () => void;
  onKeep: () => void;
}) {
  return (
    <div
      role="alert"
      className="flex flex-wrap items-center gap-x-3 gap-y-1 border-t border-line bg-danger-soft px-4 py-1.5 text-[14px] text-danger"
    >
      <Icon name="peringatan" size={18} className="shrink-0" />
      <p className="min-w-0 flex-1">Pesan gagal dihapus — {error}</p>
      <span className="flex shrink-0 items-center gap-1">
        <PillButton tone="ghost" size="sm" onClick={onKeep}>
          Biarkan
        </PillButton>
        <PillButton tone="danger" size="sm" icon="ulang" onClick={onRetry}>
          Coba lagi
        </PillButton>
      </span>
    </div>
  );
}
