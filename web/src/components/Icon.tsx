/**
 * Ikon garis, satu berkas.
 *
 * Sebelumnya tempat-tempat ini diisi emoji — 📎, ⚙, 🔔, ✕. Emoji digambar oleh
 * sistem operasi, bukan oleh aplikasi: yang sama persis di layar satu orang
 * tampil berwarna dan gemuk di layar orang lain, ukurannya tidak bisa diatur,
 * dan tidak satu pun bisa mengikuti warna teks di sekitarnya. Untuk tombol —
 * benda yang harus terbaca seragam — itu terlalu banyak yang diserahkan kepada
 * kebetulan.
 *
 * Emoji TETAP dipakai di reaksi dan di pratinjau lampiran, dan itu bukan
 * inkonsistensi: di sana emoji adalah isinya, bukan lambang tombolnya.
 */
export type IconName =
  | 'kembali'
  | 'tulis'
  | 'cari'
  | 'klip'
  | 'kirim'
  | 'anggota'
  | 'tutup'
  | 'lonceng'
  | 'lonceng-mati'
  | 'keluar'
  | 'balas'
  | 'reaksi'
  | 'terbaca'
  | 'kamera'
  | 'matahari'
  | 'bulan'
  | 'perangkat'
  | 'teruskan'
  | 'sematan'
  | 'bawah'
  | 'mikrofon'
  | 'emoji'
  | 'hapus'
  | 'putar'
  | 'jeda'
  | 'tambah';

/** Jalur `d` tiap ikon. Semuanya digambar di kotak 24×24 yang sama. */
const JALUR: Record<IconName, string> = {
  kembali: 'M19 12H5m7-7-7 7 7 7',
  tulis: 'M4 20h4l10.5-10.5a2.12 2.12 0 0 0-3-3L5 17v3zM14.5 6.5l3 3',
  cari: 'M11 19a8 8 0 1 0 0-16 8 8 0 0 0 0 16zM21 21l-4.35-4.35',
  klip: 'M21.44 11.05l-8.49 8.49a6 6 0 0 1-8.49-8.49l8.49-8.49a4 4 0 0 1 5.66 5.66l-8.5 8.49a2 2 0 0 1-2.82-2.83l7.78-7.78',
  kirim: 'M4.5 12h6m-6.2-7.1 15.2 7.1-15.2 7.1 1.7-7.1-1.7-7.1z',
  anggota:
    'M16 20v-1.5a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4V20M9 10.5a3.5 3.5 0 1 0 0-7 3.5 3.5 0 0 0 0 7zM22 20v-1.5a4 4 0 0 0-3-3.87M16 3.63a4 4 0 0 1 0 7.75',
  tutup: 'M18 6 6 18M6 6l12 12',
  lonceng: 'M18 9a6 6 0 1 0-12 0c0 5-2.5 6-2.5 6h17S18 14 18 9M13.7 20a2 2 0 0 1-3.4 0',
  'lonceng-mati':
    'M13.7 20a2 2 0 0 1-3.4 0M18.6 14A5.5 5.5 0 0 0 18 9a6 6 0 0 0-9.3-5M5.9 6.1A6 6 0 0 0 6 9c0 5-2.5 6-2.5 6h13M2 2l20 20',
  keluar: 'M15 17l5-5-5-5M20 12H9M11 4H6a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h5',
  balas: 'M9 14 4 9l5-5M4 9h9a7 7 0 0 1 7 7v4',
  reaksi:
    'M20.9 13a9 9 0 1 1-7.9-9.9M8.5 14.5s1.3 1.7 3.5 1.7 3.5-1.7 3.5-1.7M9 9.5h.01M15 9.5h.01M19 2v6M22 5h-6',
  terbaca: 'M1.5 12.5 5 16l7.5-9M11 16l1 1 8.5-10',
  kamera:
    'M21 18V8a2 2 0 0 0-2-2h-2.5l-1.3-2h-6.4L7.5 6H5a2 2 0 0 0-2 2v10a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2zM12 16.5a4 4 0 1 0 0-8 4 4 0 0 0 0 8z',
  matahari:
    'M12 8a4 4 0 1 0 0 8 4 4 0 0 0 0-8M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4',
  bulan: 'M20.5 14.3A8.5 8.5 0 0 1 9.7 3.5a8.5 8.5 0 1 0 10.8 10.8z',
  perangkat: 'M20 4H4a1 1 0 0 0-1 1v10a1 1 0 0 0 1 1h16a1 1 0 0 0 1-1V5a1 1 0 0 0-1-1M8 20h8M12 16v4',
  // Cermin dari `balas`: panah yang sama, arah yang berlawanan.
  teruskan: 'M15 14l5-5-5-5M20 9h-9a7 7 0 0 0-7 7v4',
  sematan: 'M9 3.5h6M10 3.5l-.6 6.2L6.5 13v2h11v-2l-2.9-3.3L14 3.5M12 15v5.5',
  bawah: 'M12 5v14m-6-6 6 6 6-6',
  mikrofon: 'M12 3a3 3 0 0 0-3 3v6a3 3 0 0 0 6 0V6a3 3 0 0 0-3-3zM19 11a7 7 0 0 1-14 0M12 18v3M8.5 21h7',
  // `reaksi` tanpa tanda tambah: yang ini menyisipkan, bukan memberi.
  emoji:
    'M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18zM8.5 14.5s1.3 1.7 3.5 1.7 3.5-1.7 3.5-1.7M9 9.5h.01M15 9.5h.01',
  hapus: 'M4 7h16M10 11v6M14 11v6M5.5 7l1 12a2 2 0 0 0 2 1.8h7a2 2 0 0 0 2-1.8l1-12M9 7V4.5h6V7',
  putar: 'M7 4.5v15l12.5-7.5z',
  jeda: 'M8 5v14M16 5v14',
  tambah: 'M12 5v14M5 12h14',
};

export default function Icon({
  name,
  size = 20,
  className = '',
}: {
  name: IconName;
  size?: number;
  className?: string;
}) {
  return (
    <svg
      // Ikon di sini tidak pernah berdiri sendiri sebagai makna: tombolnya
      // selalu punya aria-label atau teks di sebelahnya. Jadi bagi pembaca
      // layar dia memang harus tidak ada, bukan dibacakan dua kali.
      aria-hidden="true"
      focusable="false"
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={1.75}
      strokeLinecap="round"
      strokeLinejoin="round"
      className={`shrink-0 ${className}`}
    >
      <path d={JALUR[name]} />
    </svg>
  );
}
