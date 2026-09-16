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
 * Tipe yang server sajikan sebagai `inline` dan browser bisa putar sendiri.
 *
 * Sengaja daftar izin, bukan tebakan dari awalan `video/`: server memakai
 * daftar yang sama untuk memutuskan Content-Disposition, dan tipe yang tidak
 * ada di sana akan dipaksa terunduh. Merender `<video>` untuknya cuma
 * menghasilkan pemutar hitam yang tidak pernah bisa jalan — lebih jujur
 * menampilkannya sebagai berkas yang bisa diunduh.
 */
const PLAYABLE = new Set([
  'video/mp4',
  'video/webm',
  'audio/mpeg',
  'audio/wave',
  'application/ogg',
]);

const isVideo = (a: Attachment) => a.mime.startsWith('video/') && PLAYABLE.has(a.mime);
const isAudio = (a: Attachment) =>
  PLAYABLE.has(a.mime) && (a.mime.startsWith('audio/') || a.mime === 'application/ogg');

/**
 * Lampiran di dalam gelembung pesan.
 *
 * Gambar ditampilkan langsung, rekaman diberi pemutar, berkas lain jadi baris
 * yang bisa diunduh. Pembedaannya mengikuti apa yang AMAN dirender server —
 * mencoba menampilkan yang lain hanya akan menghasilkan ikon rusak.
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
  const media = attachments.filter(a => !isImage(a) && (isVideo(a) || isAudio(a)));
  const others = attachments.filter(a => !isImage(a) && !isVideo(a) && !isAudio(a));

  return (
    <div className="flex flex-col gap-1.5">
      {images.length > 0 && (
        <div className={`grid gap-1.5 ${images.length > 1 ? 'grid-cols-2' : 'grid-cols-1'}`}>
          {images.map(a => (
            <a
              key={a.id}
              // Yang dibuka saat diklik tetap berkas ASLINYA. Turunan hanya
              // untuk ditampilkan di dalam gelembung — orang yang mengklik
              // sebuah foto sedang meminta melihatnya dengan jelas.
              href={attachmentURL(a.url)}
              target="_blank"
              rel="noreferrer"
              className="block overflow-hidden rounded-lg"
            >
              <img
                // Turunan bila ada, aslinya bila tidak. Foto dua belas
                // megapiksel dari ponsel sebelumnya diunduh utuh untuk
                // ditampilkan selebar tiga ratus piksel — oleh setiap anggota
                // percakapan, setiap kali percakapannya dibuka.
                src={attachmentURL(a.thumbUrl ?? a.url)}
                alt={a.name}
                // Ruangnya dipesan dari ukuran yang ikut tersimpan saat
                // diunggah. Tanpa ini, tiap gambar yang selesai dimuat
                // mendorong daftar pesan — tepat saat orang sedang membaca.
                //
                // Ukuran aslinya, bukan ukuran turunannya: perbandingan
                // sisinya sama, dan yang dipesan memang bentuk, bukan piksel.
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

      {media.map(a =>
        isVideo(a) ? (
          <video
            key={a.id}
            src={attachmentURL(a.url)}
            controls
            // Yang membuat ini hemat: browser hanya mengambil header berkasnya
            // lewat permintaan sepotong, cukup untuk tahu durasi dan ukuran
            // layar. Sisanya baru diambil saat tombol putar ditekan — dan
            // melompat ke menit kesepuluh mengambil menit kesepuluh saja,
            // bukan sembilan menit sebelumnya.
            preload="metadata"
            className="max-h-72 w-full rounded-lg bg-black"
          />
        ) : (
          <audio
            key={a.id}
            src={attachmentURL(a.url)}
            controls
            preload="metadata"
            className="w-full min-w-56"
          />
        ),
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
