import { useStore } from '../store';
import { formatBytes } from './AttachmentList';
import type { Upload } from '../types';

/**
 * Satu array kosong yang dipakai bersama, bukan `[]` baru tiap kali.
 *
 * Selector zustand dibandingkan dengan Object.is. Menulis `s.uploads[id] ?? []`
 * menghasilkan array BARU pada setiap pembacaan, sehingga hasilnya tidak pernah
 * dianggap sama dengan sebelumnya — komponen ini akan render ulang tanpa henti
 * sampai React menyerah dengan "Maximum update depth exceeded". Bentuk kesalahan
 * ini tidak terlihat di typecheck maupun di test; dia baru muncul saat
 * halamannya benar-benar dibuka.
 */
const KOSONG: Upload[] = [];

/**
 * Berkas yang sudah dipilih tapi pesannya belum dikirim.
 *
 * Tampil di atas kolom tulis, bukan di dalam daftar pesan: ini masih bagian
 * dari apa yang sedang disusun, bukan sesuatu yang sudah terjadi. Menampilkan
 * unggahan yang belum terkirim sebagai gelembung akan membuat orang mengira
 * lawan bicaranya sudah melihatnya.
 */
export default function UploadStrip({ conversationId }: { conversationId: string }) {
  const uploads = useStore(s => s.uploads[conversationId] ?? KOSONG);
  const removeUpload = useStore(s => s.removeUpload);
  const retryUpload = useStore(s => s.retryUpload);

  if (uploads.length === 0) return null;

  return (
    <div className="flex flex-wrap gap-2 border-t border-line bg-surface px-3 pt-3">
      {uploads.map(u => (
        <div
          key={u.key}
          className={`relative flex w-40 items-center gap-2 rounded-lg border px-2 py-1.5 ${
            u.status === 'failed' ? 'border-red-500/60 bg-red-500/5' : 'border-line bg-canvas'
          }`}
        >
          {u.previewUrl ? (
            <img
              src={u.previewUrl}
              alt=""
              className="size-9 shrink-0 rounded object-cover"
            />
          ) : (
            <span className="grid size-9 shrink-0 place-items-center rounded bg-line/50 text-base">
              📎
            </span>
          )}

          <div className="min-w-0 flex-1">
            <p className="truncate text-xs font-medium">{u.name}</p>
            <UploadStatus upload={u} onRetry={() => retryUpload(conversationId, u.key)} />
          </div>

          <button
            onClick={() => removeUpload(conversationId, u.key)}
            aria-label={`Buang ${u.name}`}
            className="absolute -top-1.5 -right-1.5 grid size-5 place-items-center rounded-full border border-line bg-surface text-xs text-muted transition hover:text-ink"
          >
            ×
          </button>
        </div>
      ))}
    </div>
  );
}

function UploadStatus({ upload, onRetry }: { upload: Upload; onRetry: () => void }) {
  if (upload.status === 'failed') {
    if (!upload.retriable) {
      return <p className="text-[11px] text-red-500">{upload.error ?? 'gagal'}</p>;
    }
    return (
      <button onClick={onRetry} className="text-[11px] text-red-500 underline">
        {upload.error ?? 'gagal'} — coba lagi
      </button>
    );
  }

  if (upload.status === 'ready') {
    return <p className="text-[11px] text-muted">{formatBytes(upload.size)}</p>;
  }

  return (
    <div className="mt-1 h-1 overflow-hidden rounded-full bg-line">
      {/* Bilah kemajuan yang sebenarnya, bukan animasi yang berputar tanpa
          tahu apa-apa. Untuk berkas sepuluh megabyte, bedanya adalah antara
          "sedang jalan" dan "mungkin sudah mati". */}
      <div
        className="h-full bg-accent transition-[width] duration-150"
        style={{ width: `${Math.round(upload.progress * 100)}%` }}
      />
    </div>
  );
}
