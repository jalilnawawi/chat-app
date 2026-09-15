import { attachmentURL } from '../api';
import type { Attachment } from '../types';

/** Ukuran berkas dalam satuan yang biasa dipakai orang, bukan byte mentah. */
export function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

const isImage = (a: Attachment) => a.mime.startsWith('image/');

/**
 * Lampiran di dalam gelembung pesan.
 *
 * Gambar ditampilkan langsung, berkas lain sebagai baris yang bisa diunduh.
 * Pembedaannya mengikuti apa yang AMAN dirender server — hanya empat tipe
 * gambar yang disajikan inline, selebihnya dipaksa terunduh — jadi mencoba
 * menampilkan yang lain sebagai gambar hanya akan menghasilkan ikon rusak.
 */
export default function AttachmentList({
  attachments,
  mine,
}: {
  attachments: Attachment[];
  mine: boolean;
}) {
  if (attachments.length === 0) return null;

  const images = attachments.filter(isImage);
  const others = attachments.filter(a => !isImage(a));

  return (
    <div className="flex flex-col gap-1.5">
      {images.length > 0 && (
        <div className={`grid gap-1.5 ${images.length > 1 ? 'grid-cols-2' : 'grid-cols-1'}`}>
          {images.map(a => (
            <a
              key={a.id}
              href={attachmentURL(a.url)}
              target="_blank"
              rel="noreferrer"
              className="block overflow-hidden rounded-lg"
            >
              <img
                src={attachmentURL(a.url)}
                alt={a.name}
                // Ruangnya dipesan dari ukuran yang ikut tersimpan saat
                // diunggah. Tanpa ini, tiap gambar yang selesai dimuat
                // mendorong daftar pesan — tepat saat orang sedang membaca.
                style={
                  a.width && a.height ? { aspectRatio: `${a.width} / ${a.height}` } : undefined
                }
                // Gambar di riwayat lama tidak perlu ikut diambil sampai
                // benar-benar tergulir ke layar.
                loading="lazy"
                decoding="async"
                className="max-h-72 w-full bg-black/5 object-cover"
              />
            </a>
          ))}
        </div>
      )}

      {others.map(a => (
        <a
          key={a.id}
          href={attachmentURL(a.url)}
          // download memberi tahu browser ini unduhan, sekaligus mengembalikan
          // nama aslinya. Server sudah memaksa hal yang sama lewat
          // Content-Disposition; yang di sini supaya klik terasa benar bahkan
          // sebelum jawaban server tiba.
          download={a.name}
          className={`flex items-center gap-2.5 rounded-lg px-2.5 py-2 transition ${
            mine ? 'bg-black/15 hover:bg-black/25' : 'bg-canvas hover:bg-line/50'
          }`}
        >
          <span className="text-lg leading-none">📎</span>
          <span className="min-w-0 flex-1">
            <span className="block truncate text-sm">{a.name}</span>
            <span className={`block text-xs ${mine ? 'text-white/70' : 'text-muted'}`}>
              {formatBytes(a.size)}
            </span>
          </span>
        </a>
      ))}
    </div>
  );
}
