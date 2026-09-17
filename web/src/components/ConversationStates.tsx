import Avatar from './Avatar';
import Tanda from './Tanda';
import { labelHari } from '../format';
import { T, salam } from '../teks';
import type { Message } from '../types';

/** Panel kanan saat belum ada percakapan yang dibuka (layar lebar saja). */
export function NoConversation() {
  // Di layar sempit tidak ada "sebelah kiri" — daftarnya sedang memenuhi
  // layar, dan panel ini tidak perlu ikut hadir untuk mengatakan bahwa dia
  // kosong.
  return (
    <section className="hidden flex-1 flex-col items-center justify-center gap-3 px-8 text-center md:flex">
      <Tanda size={56} />
      <p className="text-base font-bold">Belum ada yang dibuka</p>
      <p className="max-w-[26ch] text-sm leading-relaxed text-muted">
        Pilih sebuah percakapan di sebelah kiri, atau mulai yang baru.
      </p>
    </section>
  );
}

/**
 * Penanda hari di tengah riwayat — dan satu-satunya tanda khas "Pagi di Tepi
 * Air" di luar warna: garis cakrawala.
 *
 * Pilnya duduk di atas garis tipis yang memanjang ke kedua sisi, seperti
 * matahari di atas batas air. Garisnya memudar ke tepi (bukan terpotong),
 * supaya dia terbaca sebagai cakrawala, bukan pembatas tabel. Tidak ada warna
 * baru: garisnya Garis Pisah, pilnya Kertas. Kontras teks tetap 5,3:1.
 */
export function DayDivider({ iso }: { iso: string }) {
  return (
    <div className="my-5 flex items-center gap-3">
      <span aria-hidden className="h-px flex-1 bg-linear-to-r from-transparent to-line" />
      <span className="rounded-full border border-line bg-surface px-3 py-1 text-[12px] font-bold text-muted">
        {labelHari(iso)}
      </span>
      <span aria-hidden className="h-px flex-1 bg-linear-to-l from-transparent to-line" />
    </div>
  );
}

/**
 * Sederet pesan terhapus dari satu orang, dilipat jadi satu baris.
 *
 * Bentuknya sama dengan pesan terhapus tunggal — garis putus-putus, miring,
 * redup — tapi setinggi satu baris kecil. Id tiap pesan tetap ada sebagai
 * jangkar, supaya lompatan ke salah satunya tetap mendarat di sini.
 */
export function DeletedRun({
  messages,
  mine,
  gutter,
}: {
  messages: Message[];
  mine: boolean;
  /** Di grup, pesan orang lain bergeser selebar foto pengirim; yang ini ikut. */
  gutter: boolean;
}) {
  return (
    <div
      className={`mt-3 flex flex-col px-1 ${mine ? 'items-end' : 'items-start'} ${
        gutter && !mine ? 'pl-10' : ''
      }`}
    >
      {messages.map(m => (
        <span key={m.id} id={`msg-${m.id}`} aria-hidden />
      ))}
      {/* Tanpa baris jam: deretan yang dihapus bukan kejadian yang perlu
          dicari waktunya, dan jamnya tetap dibacakan lewat label barisnya. */}
      <p className="sasaran-fokus rounded-full border border-dashed border-line-strong px-3 py-1 text-[13px] text-muted italic">
        {T.deletedRun(messages.length)}
      </p>
    </div>
  );
}

/**
 * Riwayat yang sedang diambil.
 *
 * Bentuk gelembung yang diam, bukan kilau yang bergerak: yang perlu
 * disampaikan cuma "isinya sedang datang", dan itu tidak butuh gerak.
 */
export function HistorySkeleton() {
  const rows: { mine: boolean; width: string }[] = [
    { mine: false, width: 'w-48' },
    { mine: false, width: 'w-64' },
    { mine: true, width: 'w-40' },
    { mine: false, width: 'w-56' },
    { mine: true, width: 'w-60' },
  ];
  return (
    <div className="flex flex-col gap-2 pb-2">
      <p className="sr-only">Memuat pesan…</p>
      {rows.map((r, i) => (
        <div key={i} aria-hidden className={`flex ${r.mine ? 'justify-end' : 'justify-start'}`}>
          <div
            className={`h-10 max-w-[70%] rounded-bubble ${r.width} ${
              r.mine ? 'rounded-br-tail bg-accent-soft' : 'rounded-bl-tail bg-line/70'
            }`}
          />
        </div>
      ))}
    </div>
  );
}

/**
 * Percakapan yang belum berisi apa pun.
 *
 * Kanvas kosong tidak memberi tahu apakah riwayatnya belum dimuat, hilang,
 * atau memang belum ada. Yang ini mengatakannya, menunjukkan kepada siapa
 * pesan pertama akan pergi, dan mengajarkan cara mengirim — dengan bahasa
 * papan ketik di layar bertetikus, dan bahasa jari di layar sentuh.
 */
export function EmptyConversation({
  title,
  avatarUrl,
  group,
  canAttach,
  touch,
}: {
  title: string;
  avatarUrl?: string;
  group: boolean;
  canAttach: boolean;
  touch: boolean;
}) {
  const firstName = title.split(/\s+/)[0] ?? title;
  return (
    <div className="mx-auto flex max-w-sm flex-col items-center px-4 pb-6 text-center">
      <Avatar name={title} url={avatarUrl} size={64} grup={group} />
      <p className="mt-3 text-[17px] font-bold tracking-tight">{title}</p>
      {/* Salam menurut jam: "pagi" di aplikasi ini hidup di waktu, bukan di
          warna — dan percakapan yang masih kosong adalah tempat paling wajar
          untuk mengucapkannya. */}
      <p className="mt-1 text-[15px] leading-relaxed text-muted">
        {group
          ? `${salam()}. Belum ada pesan di grup ini — mulai dengan menyapa semua anggotanya.`
          : `${salam()}. Belum ada pesan — sapa ${firstName} dengan pesan pertamamu.`}
      </p>
      <ul className="mt-4 flex flex-wrap justify-center gap-x-4 gap-y-1.5 text-[13px] text-muted">
        {touch ? (
          <>
            {/* Tidak ada petunjuk tentang tindakan pesan di sini: belum ada
                pesan yang bisa ditindak, dan "tekan lama" tidak bekerja di
                semua ponsel. */}
            <li>Ketuk tombol kirim untuk mengirim</li>
            <li>Enter membuat baris baru</li>
          </>
        ) : (
          <>
            <li>
              <Kbd>Enter</Kbd> kirim
            </li>
            <li>
              <Kbd>Shift</Kbd> + <Kbd>Enter</Kbd> baris baru
            </li>
            {canAttach && <li>Tempel atau seret gambar untuk melampirkan</li>}
          </>
        )}
      </ul>
    </div>
  );
}

export function Kbd({ children }: { children: string }) {
  return (
    <kbd className="rounded-md border border-line-strong bg-surface px-1.5 py-0.5 font-sans text-[12px] font-semibold text-ink">
      {children}
    </kbd>
  );
}
