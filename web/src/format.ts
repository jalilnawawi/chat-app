/**
 * Cara aplikasi ini menuliskan waktu, durasi, dan isi pesan dalam satu baris.
 *
 * Tinggal di luar komponen karena dipakai di mana-mana — daftar percakapan,
 * bilah sematan, kutipan, pencarian, riwayat — dan sebelumnya masing-masing
 * diimpor dari komponen yang kebetulan pertama kali membutuhkannya.
 */
import { KIND_ICON, T, kindText, type AttachmentKind } from './teks';
import type { Attachment, Message } from './types';

/** "0:07", "12:40" — panjang rekaman seperti yang biasa dibaca orang. */
export function formatDuration(ms: number): string {
  const total = Math.max(0, Math.round(ms / 1000));
  const m = Math.floor(total / 60);
  const s = total % 60;
  return `${m}:${String(s).padStart(2, '0')}`;
}

/** "08.15" — jam dan menit menurut kebiasaan Indonesia. */
export const jam = (iso: string) =>
  new Date(iso).toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit' });

export const hariSama = (a: string, b: string) =>
  new Date(a).toDateString() === new Date(b).toDateString();

/**
 * Penanda hari di tengah riwayat.
 *
 * Jam saja tidak cukup begitu percakapan melewati tengah malam: "08.15" di
 * bawah "23.40" terbaca seperti tujuh jam yang sama, padahal di antaranya ada
 * satu malam penuh.
 */
export function labelHari(iso: string): string {
  const at = new Date(iso);
  if (Number.isNaN(at.getTime())) return '';

  const hariIni = new Date();
  if (hariSama(at.toISOString(), hariIni.toISOString())) return 'Hari ini';

  const kemarin = new Date(hariIni);
  kemarin.setDate(kemarin.getDate() - 1);
  if (hariSama(at.toISOString(), kemarin.toISOString())) return 'Kemarin';

  return at.toLocaleDateString('id-ID', {
    weekday: 'long',
    day: 'numeric',
    month: 'long',
    ...(at.getFullYear() === hariIni.getFullYear() ? {} : { year: 'numeric' }),
  });
}

/** Jenis sebuah lampiran — pesan suara dibedakan dari audio lain lewat durasinya. */
export function attachmentKind(a: Attachment): AttachmentKind {
  if (a.mime.startsWith('image/')) return 'image';
  if (a.mime.startsWith('video/')) return 'video';
  if (a.durationMs !== undefined) return 'voice';
  if (a.mime.startsWith('audio/')) return 'audio';
  return 'file';
}

/** Satu baris untuk sederet lampiran: jumlahnya, atau jenis dan namanya. "" bila kosong. */
export function attachmentsText(atts: Attachment[]): string {
  if (atts.length > 1) return T.attachmentsCount(atts.length);
  const a = atts[0];
  if (!a) return '';
  const kind = attachmentKind(a);
  if (kind === 'voice') return kindText('voice', formatDuration(a.durationMs!));
  if (kind === 'file') return `${KIND_ICON.file} ${a.name}`;
  return kindText(kind);
}

/** Satu baris untuk sebuah pesan: teksnya, atau apa yang dilampirkannya. */
export function excerpt(m: Message): string {
  if (m.body) return m.body;
  return attachmentsText(m.attachments) || T.message;
}

/**
 * Kata untuk kutipan yang isinya hanya lampiran. Pratinjau balasan hanya
 * membawa jenis kasarnya (tanpa durasi), jadi audio dianggap pesan suara —
 * jenis audio yang paling sering dibalas.
 */
export function replyKindText(kind: string | undefined): string {
  switch (kind) {
    case 'image':
      return kindText('image');
    case 'video':
      return kindText('video');
    case 'audio':
      return kindText('voice');
    default:
      return kindText('file');
  }
}
