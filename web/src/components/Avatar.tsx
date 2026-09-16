import { avatarURL } from '../api';
import type { StatusKind } from '../types';

/**
 * Foto profil, dengan huruf pertama nama sebagai cadangan.
 *
 * Dipakai di sidebar, kepala percakapan, daftar anggota, dan panel akun — dan
 * itu seluruh alasan dia jadi komponen sendiri. Empat tempat yang menggambar
 * lingkaran berisi huruf dengan caranya masing-masing adalah empat tempat yang
 * suatu hari akan berbeda ukurannya.
 */
export default function Avatar({
  name,
  url,
  size = 36,
  grup = false,
  dot,
}: {
  name: string;
  url?: string;
  size?: number;
  /**
   * Grup digambar sebagai persegi membulat, orang sebagai lingkaran.
   *
   * Bentuk, bukan warna atau ikon kecil di pojok: bentuk terbaca dari sudut
   * mata, pada ukuran berapa pun, dan tidak menuntut orang menghafal arti
   * sebuah lambang lebih dulu.
   */
  grup?: boolean;
  /** Titik keadaan di pojok kanan bawah; tidak digambar bila undefined. */
  dot?: DotKind;
}) {
  const rona = ronaDari(name);
  const bentuk = grup ? 'rounded-[30%]' : 'rounded-full';

  return (
    <span
      className={`relative inline-grid shrink-0 place-items-center font-bold ${bentuk}`}
      style={{
        width: size,
        height: size,
        fontSize: Math.round(size * 0.4),
        background: `oklch(var(--avatar-l) var(--avatar-c) ${rona})`,
        color: `oklch(var(--avatar-ink-l) var(--avatar-ink-c) ${rona})`,
      }}
    >
      {url ? (
        <img
          src={avatarURL(url)}
          alt=""
          // Alamatnya memuat id unggahan, jadi dia berubah tiap foto diganti —
          // itu yang membuat `loading="lazy"` dan cache setahun di sisi server
          // aman dipakai bersamaan: tidak ada alamat yang isinya pernah basi.
          loading="lazy"
          className={`size-full object-cover ${bentuk}`}
        />
      ) : (
        <span aria-hidden>{name.charAt(0).toUpperCase()}</span>
      )}

      {dot && (
        <span
          title={dotTitles[dot]}
          className={`absolute right-0 bottom-0 rounded-full border-2 border-surface ${dotColors[dot]}`}
          style={{ width: Math.max(9, size * 0.28), height: Math.max(9, size * 0.28) }}
        />
      )}
    </span>
  );
}

/**
 * Rona tetap untuk sebuah nama.
 *
 * Tujuh rona yang sudah dipilih agar rukun dengan palet, bukan nilai acak dari
 * seluruh lingkaran warna — yang terakhir itu cepat atau lambat menghasilkan
 * lingkaran yang berkelahi dengan teal di sebelahnya.
 *
 * Gunanya bukan hiasan: di grup berisi belasan orang tanpa foto, warna adalah
 * hal pertama yang dikenali mata sebelum hurufnya sempat dibaca. Karena
 * dihitung dari nama, orang yang sama selalu mendapat warna yang sama di setiap
 * perangkat, tanpa sekali pun perlu disimpan.
 */
const RONA = [196, 72, 152, 25, 285, 330, 248];

function ronaDari(name: string): number {
  let jumlah = 0;
  for (let i = 0; i < name.length; i++) jumlah = (jumlah * 31 + name.charCodeAt(i)) % 100003;
  return RONA[jumlah % RONA.length]!;
}

export type DotKind = 'online' | 'busy' | 'away';

const dotColors: Record<DotKind, string> = {
  online: 'bg-ok',
  busy: 'bg-danger',
  away: 'bg-call',
};

const dotTitles: Record<DotKind, string> = {
  online: 'Online',
  busy: 'Sedang sibuk',
  away: 'Sedang tidak di tempat',
};

/**
 * Menggabungkan presence dan status jadi SATU titik.
 *
 * Aturannya: status yang dinyatakan orang dengan sengaja selalu menang, dan
 * presence baru mengisi sisanya. Itu bukan pilihan sembarangan — seseorang yang
 * memasang "sedang rapat" lalu tetap membuka tabnya tidak sedang mengatakan
 * "sapa saya"; kalau presence yang menang, satu-satunya cara statusnya terlihat
 * adalah dengan menutup aplikasinya.
 *
 * Yang offline tanpa status tidak punya titik sama sekali. Titik abu-abu untuk
 * "tidak ada apa-apa" cuma menambah benda di layar yang artinya ketiadaan.
 */
export function dotFor(online: boolean, status?: StatusKind): DotKind | undefined {
  if (status === 'busy') return 'busy';
  if (status === 'away') return 'away';
  return online ? 'online' : undefined;
}

/** Kalimat status untuk baris keterangan, atau kosong bila tidak ada apa-apa. */
export function statusLabel(status?: StatusKind, text?: string, expiresAt?: string): string {
  const dasar = text || (status && status !== 'available' ? dotTitles[status as DotKind] : '');
  if (!dasar) return '';
  const sampai = untilLabel(expiresAt);
  return sampai ? `${dasar} · ${sampai}` : dasar;
}

/**
 * "sampai 13.00", dirender dalam jam LOKAL pembacanya.
 *
 * Server menyimpan instan absolut dan tidak pernah tahu zona waktu siapa pun.
 * Itu keputusan yang harus punya jawaban sebelum baris pertama ditulis:
 * "sampai jam 13.00" di jam siapa — dan jawabannya, jam orang yang membacanya.
 *
 * Yang sudah lewat menghasilkan kosong. Server sudah menyaringnya saat membaca,
 * tapi halaman yang terbuka berjam-jam tidak bertanya lagi — dan status yang
 * batas waktunya sudah lewat sementara halamannya masih terbuka adalah persis
 * kasus yang paling mungkin terjadi.
 */
export function untilLabel(expiresAt?: string): string {
  if (!expiresAt) return '';
  const at = new Date(expiresAt);
  if (Number.isNaN(at.getTime()) || at.getTime() <= Date.now()) return '';
  return (
    'sampai ' +
    at.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' })
  );
}
