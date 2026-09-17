import type { Ref } from 'react';
import DeleteNotice, { DeleteFailed, type WaitingDelete } from './DeleteNotice';
import Icon from './Icon';
import PillButton from './ui/PillButton';

/**
 * Kabar singkat. `action` memberi satu tombol (misalnya "Urungkan" untuk
 * sematan yang masih ditahan); `sticky` membuatnya bertahan sampai pemiliknya
 * menutupnya sendiri, alih-alih hilang setelah beberapa detik.
 */
export type Flash = {
  tone: 'ok' | 'warn';
  text: string;
  action?: { label: string; onClick: () => void };
  sticky?: boolean;
};

/**
 * Kabar di atas kolom tulis — paling banyak SATU pada satu waktu.
 *
 * Tanpa aturan ini, hapus yang gagal, hapus yang menunggu, dan kabar singkat
 * bisa bertumpuk di atas laci lampiran dan kutipan balasan, dan kolom tulis
 * terdorong jauh ke bawah justru saat orang ingin menulis. Urutannya menurut
 * apa yang paling butuh tindakan:
 *
 *   1. Hapus yang ditolak server — pesannya belum hilang, dan orangnya perlu
 *      memutuskan.
 *   2. Hapus yang menunggu — jalan kembalinya hanya beberapa detik.
 *   3. Kabar singkat ("Teks disalin.") — tidak butuh apa-apa.
 *
 * Yang tidak tampil tetap diumumkan ke pembaca layar oleh ChatPanel.
 *
 * Kabar yang TIDAK ada di sini sengaja: bilah koneksi dan galat sematan
 * menempel di bawah kepala percakapan, karena keduanya tentang percakapan,
 * bukan tentang apa yang sedang ditulis; "sedang mengetik" tinggal di wilayah
 * statusnya sendiri; laci lampiran dan kutipan balasan adalah bagian kolom
 * tulis.
 */
export default function NoticeStack({
  failed,
  waiting,
  flash,
  undoRef,
  onRetry,
  onKeep,
  onUndo,
  onPause,
  onResume,
}: {
  failed: { id: string; error: string } | null;
  waiting: WaitingDelete[];
  flash: Flash | null;
  undoRef: Ref<HTMLButtonElement>;
  onRetry: (id: string) => void;
  onKeep: (id: string) => void;
  onUndo: () => void;
  onPause: () => void;
  onResume: () => void;
}) {
  if (failed) {
    return (
      <DeleteFailed
        error={failed.error}
        onRetry={() => onRetry(failed.id)}
        onKeep={() => onKeep(failed.id)}
      />
    );
  }
  if (waiting.length > 0) {
    return (
      <DeleteNotice
        waiting={waiting}
        undoRef={undoRef}
        onUndo={onUndo}
        onPause={onPause}
        onResume={onResume}
      />
    );
  }
  if (flash) {
    return (
      <div
        className={`flex items-center gap-2.5 border-t border-line bg-surface px-4 text-[14px] text-ink ${
          flash.action ? 'py-1.5' : 'py-2.5'
        }`}
      >
        <Icon
          name={flash.action ? 'sematan' : flash.tone === 'ok' ? 'terkirim' : 'peringatan'}
          size={18}
          className={flash.action ? 'text-muted' : flash.tone === 'ok' ? 'text-ok' : 'text-danger'}
        />
        {/* Teksnya sudah diumumkan ChatPanel lewat wilayah status. */}
        <p aria-hidden className="min-w-0 flex-1 truncate">
          {flash.text}
        </p>
        {flash.action && (
          <PillButton tone="outline" size="sm" onClick={flash.action.onClick}>
            {flash.action.label}
          </PillButton>
        )}
      </div>
    );
  }
  return null;
}
