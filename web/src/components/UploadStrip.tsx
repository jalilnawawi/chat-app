import { useStore } from '../store';
import { formatBytes } from './AttachmentList';
import Icon from './Icon';
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
          className={`relative flex w-44 items-center gap-2.5 rounded-xl border px-2.5 py-2 ${
            u.status === 'failed' ? 'border-danger bg-danger-soft' : 'border-line bg-canvas'
          }`}
        >
          {u.previewUrl ? (
            <img
              src={u.previewUrl}
              alt=""
              className="size-10 shrink-0 rounded-lg object-cover"
            />
          ) : (
            <span className="grid size-10 shrink-0 place-items-center rounded-lg bg-line text-muted">
              <Icon name="klip" size={18} />
            </span>
          )}

          <div className="min-w-0 flex-1">
            <p className="truncate text-[13px] font-semibold">{u.name}</p>
            <UploadStatus upload={u} onRetry={() => retryUpload(conversationId, u.key)} />
          </div>

          <button
            onClick={() => removeUpload(conversationId, u.key)}
            aria-label={`Buang ${u.name}`}
            className="absolute -top-2 -right-2 grid size-6 place-items-center rounded-full border border-line bg-surface text-muted shadow-pop transition hover:text-ink"
          >
            <Icon name="tutup" size={13} />
          </button>
        </div>
      ))}
    </div>
  );
}

function UploadStatus({ upload, onRetry }: { upload: Upload; onRetry: () => void }) {
  if (upload.status === 'failed') {
    if (!upload.retriable) {
      return <p className="text-[11.5px] text-danger">{upload.error ?? 'gagal'}</p>;
    }
    return (
      <button onClick={onRetry} className="text-[11.5px] font-medium text-danger underline underline-offset-2">
        {upload.error ?? 'gagal'} — coba lagi
      </button>
    );
  }

  if (upload.status === 'ready') {
    return <p className="text-[11.5px] text-muted">{formatBytes(upload.size)}</p>;
  }

  return (
    <div className="mt-1.5 h-1.5 overflow-hidden rounded-full bg-line">
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
