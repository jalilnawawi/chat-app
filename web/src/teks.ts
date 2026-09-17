/**
 * Kalimat antarmuka yang dipakai di lebih dari satu tempat.
 *
 * Satu tempat untuk kata yang sama: "Pesan suara" di bilah sematan, di
 * kutipan, dan di daftar percakapan tidak boleh berubah jadi "Rekaman suara"
 * di salah satunya. Ini juga titik awal terjemahan — kalimat yang tersebar di
 * JSX adalah kalimat yang terlewat saat aplikasi ini diterjemahkan.
 *
 * Lambang jenis lampiran dipisah dari katanya. Kata diterjemahkan; lambang
 * tidak, dan menempelkannya ke dalam kalimat membuat penerjemah harus menjaga
 * emoji di posisi yang benar untuk setiap bahasa.
 */
import type { SystemEvent } from './types';

export type AttachmentKind = 'image' | 'video' | 'voice' | 'audio' | 'file';

export const KIND_LABEL: Record<AttachmentKind, string> = {
  image: 'Gambar',
  video: 'Video',
  voice: 'Pesan suara',
  audio: 'Audio',
  file: 'Lampiran',
};

export const KIND_ICON: Record<AttachmentKind, string> = {
  image: '📷',
  video: '🎬',
  voice: '🎤',
  audio: '🎵',
  file: '📎',
};

/** "📷 Gambar" — lambang dan kata, dirakit di satu tempat. */
export const kindText = (kind: AttachmentKind, detail?: string) =>
  `${KIND_ICON[kind]} ${detail ? `${KIND_LABEL[kind]} ${detail}` : KIND_LABEL[kind]}`;

export const T = {
  deletedMessage: 'Pesan ini dihapus',
  deletingMessage: 'Menghapus…',
  someone: 'Seseorang',
  you: 'Kamu',
  youLower: 'kamu',
  message: 'Pesan',
  attachmentsCount: (n: number) => `${KIND_ICON.file} ${n} lampiran`,
  deletedRun: (n: number) => `${n} pesan dihapus`,
  mentionsYou: 'menyebut kamu',
  reactions: (n: number) => `${n} reaksi`,
  noActionForDeleted: 'Tidak ada tindakan untuk pesan yang sudah dihapus.',
  deleting: 'Menghapus untuk semua orang…',
  deletingMany: (n: number) => `Menghapus ${n} pesan untuk semua orang…`,
  deleteScheduled: (seconds: number) =>
    `Menghapus pesan untuk semua orang. Urungkan tersedia selama ${seconds} detik.`,
  deleteUndone: 'Hapus dibatalkan.',
  undo: 'Urungkan',
  paused: 'dijeda',
  secondsLeft: (n: number) => `${n} detik`,
  copied: 'Teks disalin.',
  copyBlocked: 'Browser ini tidak mengizinkan menyalin teks.',
  copyFailed: 'Teks tidak bisa disalin di browser ini.',
  newMessages: (n: number) => (n > 0 ? `${n} pesan baru` : 'Ke pesan terbaru'),
  writePlaceholder: 'Tulis pesan…',
  replyingToSelf: 'Membalas pesanmu',
  replyingTo: (name: string) => `Membalas ${name}`,
  cancelReply: 'Batalkan balasan',
  dropToAttach: 'Lepaskan untuk melampirkan',
  reconnecting: 'Menyambungkan ulang…',
  reconnectingBody: 'Pesan yang kamu tulis tetap tersimpan dan terkirim begitu tersambung.',
  loadOlder: 'Muat pesan lama',
  loadNewer: 'Muat pesan berikutnya',
  closeError: 'Tutup pesan kesalahan',
  notSent: (reason?: string) => `Tidak terkirim — ${reason ?? 'menunggu koneksi'}`,
  rewrite: 'Tulis ulang',
  resend: 'Kirim ulang',
  send: 'Kirim',
  uploading: 'Mengunggah…',
  record: 'Rekam',
  pinning: 'Menyematkan pesan…',
  unpinning: 'Melepas sematan…',
  pinUndone: 'Sematan dibatalkan.',
  pinFailed: 'Sematan gagal diubah',
  attachmentsDropped: 'Lampiran pesan itu dilepas; pilih lagi bila masih perlu.',
  sendHint:
    'Enter kirim · Shift+Enter baris baru · ↑ sunting pesan terakhir · Esc ke riwayat',
  sendHintTouch: 'Tombol kirim mengirim pesan; Enter membuat baris baru.',
  sendHintSr:
    'Enter untuk mengirim, Shift dan Enter untuk baris baru, panah atas di kolom kosong untuk menyunting pesan terakhir, Escape di kolom kosong untuk pindah ke riwayat.',
  voiceFailed: (reason: string) => `Tidak bisa diputar — ${reason}`,
} as const;

/** Salam menurut jam setempat — "pagi" di aplikasi ini hidup di waktu, bukan di warna. */
export function salam(date = new Date()): string {
  const h = date.getHours();
  if (h >= 4 && h < 11) return 'Selamat pagi';
  if (h >= 11 && h < 15) return 'Selamat siang';
  if (h >= 15 && h < 18) return 'Selamat sore';
  return 'Selamat malam';
}

/**
 * Menyusun kalimat untuk sebuah catatan sistem.
 *
 * Kalimatnya dirakit DI SINI, bukan disimpan di server. Yang tersimpan cuma
 * kejadiannya — siapa melakukan apa kepada siapa — sehingga bahasanya bisa
 * berubah, atau diterjemahkan, tanpa menulis ulang riwayat siapa pun.
 *
 * Namanya diambil dari catatan itu sendiri, bukan dari daftar anggota: orang
 * yang dikeluarkan sudah tidak ada di sana, dan "Budi mengeluarkan (tidak
 * dikenal)" gagal justru pada satu hal yang ingin diketahui orang.
 *
 * Kejadian yang dilakukan pembaca sendiri ditulis dari sudut pandangnya:
 * "Bu Sri menyematkan…" di layar Bu Sri terbaca seperti orang lain yang
 * kebetulan bernama sama.
 */
export function systemText(ev: SystemEvent, meId?: string): string {
  const actor = ev.actor.id === meId ? T.you : ev.actor.name;
  const targets = (ev.targets ?? []).map(t => (t.id === meId ? T.youLower : t.name)).join(', ');

  switch (ev.type) {
    case 'member.added':
      return `${actor} menambahkan ${targets}`;
    case 'member.removed':
      return `${actor} mengeluarkan ${targets}`;
    case 'member.left':
      return `${actor} keluar dari grup`;
    case 'title.changed':
      return `${actor} mengganti judul grup jadi "${ev.title}"`;
    case 'owner.changed':
      return `${actor} menjadikan ${targets} pemilik grup`;
    case 'message.pinned':
      return `${actor} menyematkan sebuah pesan`;
    case 'message.unpinned':
      return `${actor} melepas sematan sebuah pesan`;
    default:
      return 'Grup diperbarui';
  }
}
