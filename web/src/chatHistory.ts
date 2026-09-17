import { excerpt, hariSama, jam } from './format';
import { T, systemText } from './teks';
import type { Message } from './types';

/**
 * Jeda yang memutus sebuah rentetan pesan.
 *
 * Dua pesan dari orang yang sama dengan jarak setengah jam bukan satu tarikan
 * napas, walau tidak ada siapa pun yang menyela di antaranya. Menempelkannya
 * jadi satu blok menyembunyikan jeda yang justru punya arti.
 */
export const RUN_GAP_MS = 5 * 60_000;

/** Satu baris di riwayat: sebuah pesan, atau sederet pesan terhapus yang dilipat. */
export type HistoryRow =
  | { kind: 'message'; message: Message; firstOfRun: boolean; newDay: boolean }
  | { kind: 'deleted'; messages: Message[]; newDay: boolean };

/** Deretan pesan terhapus sepanjang ini atau lebih dilipat jadi satu baris. */
const FOLD_DELETED_FROM = 2;

/**
 * Menyusun riwayat jadi baris yang siap digambar.
 *
 * Di sini diputuskan tiga hal yang sebelumnya dihitung ulang di dalam JSX:
 * di mana hari berganti, pesan mana yang membuka rentetan, dan pesan terhapus
 * mana yang dilipat. Pesan terhapus yang berurutan dari orang yang sama, di
 * hari yang sama, tidak perlu masing-masing mendapat gelembung setinggi
 * pesan sungguhan — yang penting diketahui cuma bahwa ada yang dihapus, dan
 * berapa.
 */
export function buildHistory(list: Message[]): HistoryRow[] {
  const rows: HistoryRow[] = [];
  for (let i = 0; i < list.length; i++) {
    const m = list[i]!;
    const prev = list[i - 1];
    // Hari baru selalu memutus rentetan: pesan pertama sesudah tengah malam
    // adalah pembuka, walau pengirimnya orang yang sama.
    const newDay = !prev || !hariSama(prev.createdAt, m.createdAt);

    if (m.deletedAt && m.kind !== 'system') {
      let j = i;
      while (
        j + 1 < list.length &&
        list[j + 1]!.deletedAt &&
        list[j + 1]!.kind !== 'system' &&
        list[j + 1]!.senderId === m.senderId &&
        hariSama(list[j + 1]!.createdAt, m.createdAt)
      ) {
        j++;
      }
      if (j - i + 1 >= FOLD_DELETED_FROM) {
        rows.push({ kind: 'deleted', messages: list.slice(i, j + 1), newDay });
        i = j;
        continue;
      }
    }

    const firstOfRun =
      newDay ||
      prev!.senderId !== m.senderId ||
      prev!.kind === 'system' ||
      Boolean(prev!.deletedAt) ||
      new Date(m.createdAt).getTime() - new Date(prev!.createdAt).getTime() > RUN_GAP_MS;
    rows.push({ kind: 'message', message: m, firstOfRun, newDay });
  }
  return rows;
}

/** Id yang mewakili sebuah baris — untuk roving focus dan `data-msg`. */
export const rowId = (row: HistoryRow) =>
  row.kind === 'message' ? row.message.id : row.messages[0]!.id;

/**
 * Label satu baris riwayat untuk pembaca layar: siapa, kapan, apa — ditambah
 * yang terlihat di sekitar gelembungnya.
 *
 * `aria-label` pada sebuah artikel MENGGANTIKAN isinya bagi pembaca layar, jadi
 * apa pun yang tidak ditulis di sini tidak terdengar saat orang berpindah
 * pesan dengan panah: isi catatan sistem, bahwa pesan itu memanggilnya, bahwa
 * ada lampiran di samping teksnya, dan berapa reaksi yang sudah diberikan.
 */
export function rowLabel(
  row: HistoryRow,
  nameOf: (id: string) => string,
  meId: string,
  deleting: (id: string) => boolean,
): string {
  if (row.kind === 'deleted') {
    const first = row.messages[0]!;
    const who = first.senderId === meId ? T.you : nameOf(first.senderId);
    return `${who}, ${jam(row.messages.at(-1)!.createdAt)}: ${T.deletedRun(row.messages.length)}`;
  }
  const m = row.message;
  if (m.kind === 'system' && m.systemEvent) {
    return `${jam(m.createdAt)}: ${systemText(m.systemEvent, meId)}`;
  }
  const who = m.senderId === meId ? T.you : nameOf(m.senderId);
  if (m.deletedAt || deleting(m.id)) return `${who}, ${jam(m.createdAt)}: ${T.deletedMessage}`;

  const parts = [excerpt(m).slice(0, 120)];
  if (m.body && m.attachments.length > 0) parts.push(T.attachmentsCount(m.attachments.length));
  const callsMe = m.senderId !== meId && (m.mentionsAll || m.mentions.includes(meId));
  if (callsMe) parts.push(T.mentionsYou);
  const reactions = m.reactions.reduce((n, r) => n + r.count, 0);
  if (reactions > 0) parts.push(T.reactions(reactions));
  return `${who}, ${jam(m.createdAt)}: ${parts.join('. ')}`;
}
