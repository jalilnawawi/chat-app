/**
 * Bentuk gelombang pesan suara: 40 batang, satu karakter per batang.
 *
 * Ada di modulnya sendiri karena dua ujung harus setuju tanpa pernah saling
 * melihat — perekam yang menyusunnya dan pemutar yang menggambarnya — dan
 * ujung ketiga, CHECK di 0010_bentuk_gelombang.sql, hanya bisa menolak yang
 * bentuknya salah. Satu tempat yang menyebut jumlah batang dan abjadnya
 * membuat ketiganya tidak bisa berselisih diam-diam.
 */

/** Berapa batang yang digambar, sama dengan CHECK di 0010_bentuk_gelombang.sql. */
export const WAVEFORM_BARS = 40;

/**
 * base64url, 64 karakter, indeksnya adalah nilainya.
 *
 * Bukan base64 biasa: `+` dan `/` berubah arti di dalam query string, dan
 * bentuk gelombang dikirim lewat query — dengan alasan yang sama dengan durasi
 * dan ukuran gambar.
 */
const ABJAD = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_';

/**
 * Meringkas tingkat suara sepanjang rekaman jadi 40 batang.
 *
 * Yang diambil per petak adalah PUNCAKnya, bukan rata-ratanya. Rata-rata
 * meratakan satu kalimat pendek di tengah keheningan sampai tidak terlihat,
 * dan justru itulah yang paling ingin dilihat orang sebelum menggeser.
 *
 * Tidak dinormalkan ke batang tertinggi. Rekaman yang pelan memang terlihat
 * pelan, dan mikrofon yang dibisukan dari perangkatnya tetap terlihat sebagai
 * garis datar — sama seperti di bilah perekam, dan karena alasan yang sama.
 */
export function encodeWaveform(levels: number[]): string | null {
  const n = levels.length;
  if (n === 0) return null;

  let out = '';
  for (let i = 0; i < WAVEFORM_BARS; i++) {
    const from = Math.floor((i * n) / WAVEFORM_BARS);
    // Sekurang-kurangnya satu contoh per batang: rekaman yang lebih pendek
    // dari 40 petak (di bawah 4 detik) punya petak kosong, dan petak kosong
    // digambar sebagai keheningan yang tidak pernah ada.
    const to = Math.max(from + 1, Math.floor(((i + 1) * n) / WAVEFORM_BARS));
    let puncak = 0;
    for (let j = from; j < to && j < n; j++) puncak = Math.max(puncak, levels[j]!);
    out += ABJAD[Math.min(63, Math.max(0, Math.round(puncak * 63)))];
  }
  return out;
}

/**
 * Membaca 40 karakter jadi 40 angka 0..1, atau null bila bentuknya salah.
 *
 * Kolom ini berisi apa yang dikirim client, dan client bisa berupa apa saja.
 * Yang salah bentuk tidak menggagalkan apa pun — pemutarnya kembali ke
 * penggeser polos, persis seperti rekaman audio yang diunggah dari disk.
 */
export function decodeWaveform(peaks: string | undefined): number[] | null {
  if (!peaks || peaks.length !== WAVEFORM_BARS) return null;

  const out: number[] = [];
  for (const c of peaks) {
    const v = ABJAD.indexOf(c);
    if (v < 0) return null;
    out.push(v / 63);
  }
  return out;
}
