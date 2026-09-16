/**
 * Pilihan tampilan: ikut sistem, terang, atau gelap.
 *
 * Tiga keadaan, bukan dua. "Ikut sistem" bukan sekadar nilai awal yang kebetulan
 * dipakai sebelum orang memilih — dia pilihan tersendiri, dan satu-satunya yang
 * ikut berubah saat ponsel berpindah ke mode malam pada jam enam sore. Saklar
 * dua posisi memaksa orang membekukan salah satunya selamanya.
 *
 * Disimpan di perangkat, bukan di akun. Layar yang dipakai siang hari di kantor
 * dan layar yang dipakai di kamar sebelum tidur adalah dua layar yang berbeda,
 * walau akunnya satu.
 */
export type Tema = 'sistem' | 'terang' | 'gelap';

const KUNCI = 'tema';

/**
 * Warna bilah alamat ponsel, satu untuk tiap tema.
 *
 * Nilainya dieja di sini karena `<meta>` tidak bisa membaca variabel CSS, dan
 * karena oklch() belum aman dipakai di sana. Keduanya adalah --color-canvas
 * pada index.css; kalau yang di sana berubah, yang di sini ikut.
 */
const BILAH: Record<'terang' | 'gelap', string> = {
  terang: '#e7f8f7',
  gelap: '#0a1419',
};

const gelapDiSistem = () =>
  typeof window !== 'undefined' && window.matchMedia('(prefers-color-scheme: dark)').matches;

/** Pilihan yang tersimpan, atau "ikut sistem" bila belum pernah ada yang dipilih. */
export function temaTersimpan(): Tema {
  try {
    const nilai = localStorage.getItem(KUNCI);
    return nilai === 'terang' || nilai === 'gelap' ? nilai : 'sistem';
  } catch {
    // Mode penyamaran dan penyimpanan yang diblokir melempar di sini. Yang
    // hilang cuma ingatannya; aplikasinya tetap harus jalan.
    return 'sistem';
  }
}

/**
 * Menerjemahkan pilihan jadi keadaan yang benar-benar terpasang di halaman.
 *
 * Di sinilah "ikut sistem" berhenti jadi pilihan dan jadi salah satu dari dua
 * nilai — index.css hanya mengenal `terang` dan `gelap`.
 */
function terapkanTema(pilihan: Tema): void {
  const gelap = pilihan === 'gelap' || (pilihan === 'sistem' && gelapDiSistem());
  const akar = document.documentElement;

  akar.dataset.tema = gelap ? 'gelap' : 'terang';
  // Memberi tahu browser warna dasar halaman: yang ikut berubah karenanya
  // adalah bilah gulir, kolom isian bawaan, dan latar di balik halaman saat
  // digulir melewati ujungnya.
  akar.style.colorScheme = gelap ? 'dark' : 'light';

  document
    .querySelector('meta[name="theme-color"]')
    ?.setAttribute('content', BILAH[gelap ? 'gelap' : 'terang']);
}

/** Menyimpan pilihan sekaligus memasangnya. */
export function pilihTema(pilihan: Tema): void {
  try {
    if (pilihan === 'sistem') localStorage.removeItem(KUNCI);
    else localStorage.setItem(KUNCI, pilihan);
  } catch {
    // Tidak bisa diingat untuk kunjungan berikutnya, tapi masih bisa dipakai
    // sekarang. Menolak mengganti tema karena tidak bisa menyimpannya adalah
    // menolak mengerjakan bagian yang justru diminta orang.
  }
  terapkanTema(pilihan);
}

/**
 * Mengikuti sistem yang berganti selagi halaman terbuka.
 *
 * Bukan kasus langka: ponsel yang dijadwalkan masuk mode malam berganti sendiri
 * pada jam tertentu, dan halaman yang sedang terbuka saat itu akan berdiri
 * dengan warna kemarin sampai dimuat ulang. Hanya berlaku untuk yang memilih
 * "ikut sistem" — pilihan yang tegas tidak boleh ditimpa oleh jam berapa pun.
 */
export function pantauSistem(): () => void {
  const media = window.matchMedia('(prefers-color-scheme: dark)');
  const onChange = () => {
    if (temaTersimpan() === 'sistem') terapkanTema('sistem');
  };
  media.addEventListener('change', onChange);
  return () => media.removeEventListener('change', onChange);
}
