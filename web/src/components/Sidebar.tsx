import { useState } from 'react';
import { api } from '../api';
import { unsubscribeThisDevice, usePush } from '../push';
import { useStore } from '../store';
import type { Conversation } from '../types';
import Avatar, { dotFor, statusLabel } from './Avatar';
import NewChatDialog from './NewChatDialog';

/** Judul percakapan: grup pakai nama grup, DM pakai nama lawan bicara. */
export function conversationTitle(c: Conversation): string {
  return c.type === 'group' ? (c.title ?? 'Grup') : (c.peer?.displayName ?? 'Percakapan');
}

/**
 * Baris pratinjau di bawah judul percakapan.
 *
 * Pesan yang isinya hanya lampiran akan menghasilkan baris kosong kalau
 * body-nya dipakai begitu saja — dan baris kosong terbaca sebagai "belum ada
 * apa-apa", padahal baru saja ada foto yang masuk.
 */
function preview(c: Conversation): string {
  const last = c.lastMessage;
  if (!last) return 'Belum ada pesan';
  if (last.deletedAt) return 'Pesan dihapus';

  // Catatan sistem tidak punya body. Tanpa cabang ini, grup yang perubahan
  // terakhirnya adalah "Budi keluar" menampilkan baris kosong — yang terbaca
  // sebagai "belum ada apa-apa", padahal baru saja ada kejadian.
  if (last.kind === 'system' && last.systemEvent) {
    const ev = last.systemEvent;
    const siapa = (ev.targets ?? []).map(t => t.name).join(', ');
    switch (ev.type) {
      case 'member.added':
        return `${ev.actor.name} menambahkan ${siapa}`;
      case 'member.removed':
        return `${ev.actor.name} mengeluarkan ${siapa}`;
      case 'member.left':
        return `${ev.actor.name} keluar dari grup`;
      case 'title.changed':
        return `Judul grup jadi "${ev.title}"`;
      case 'owner.changed':
        return `${siapa} jadi pemilik grup`;
    }
  }

  if (last.body) return last.body;

  const atts = last.attachments ?? [];
  if (atts.length === 0) return '';
  if (atts.length > 1) return `📎 ${atts.length} lampiran`;
  const a = atts[0]!;
  if (a.mime.startsWith('image/')) return '📷 Gambar';
  if (a.mime.startsWith('video/')) return '🎬 Video';
  if (a.mime.startsWith('audio/')) return '🎵 Rekaman suara';
  return `📎 ${a.name}`;
}

export default function Sidebar({
  accountOpen,
  onToggleAccount,
}: {
  accountOpen: boolean;
  onToggleAccount: () => void;
}) {
  const me = useStore(s => s.me);
  const conversations = useStore(s => s.conversations);
  const activeId = useStore(s => s.activeId);
  const online = useStore(s => s.online);
  const statuses = useStore(s => s.statuses);
  const connected = useStore(s => s.connected);
  const openConversation = useStore(s => s.openConversation);
  const reset = useStore(s => s.reset);

  const [dialogOpen, setDialogOpen] = useState(false);
  const push = usePush();

  async function logout() {
    // Langganan notifikasi dicabut SEBELUM sesinya dibuang — pencabutan itu
    // sendiri butuh sesi yang masih berlaku. Tanpa ini, langganan orang ini
    // tetap hidup di komputer yang dipakai bergantian, dan pratinjau pesannya
    // terus muncul di layar kunci orang berikutnya.
    await unsubscribeThisDevice();
    await api.logout().catch(() => {});
    reset();
  }

  return (
    <aside className="flex w-72 shrink-0 flex-col border-r border-line bg-surface">
      <header className="flex items-center justify-between gap-2 border-b border-line px-4 py-3">
        {/* Kepala sidebar adalah satu tombol: menekan foto atau nama sendiri
            membuka panel akun. Itu tempat yang sudah dicari orang lebih dulu,
            jauh sebelum mereka mencari ikon roda gigi. */}
        <button
          onClick={onToggleAccount}
          aria-pressed={accountOpen}
          title="Kelola akun kamu"
          className={`flex min-w-0 flex-1 items-center gap-2.5 rounded-lg px-1.5 py-1 text-left transition ${
            accountOpen ? 'bg-accent-soft' : 'hover:bg-canvas'
          }`}
        >
          <Avatar
            name={me?.displayName ?? '?'}
            url={me?.avatarUrl}
            size={32}
            dot={dotFor(connected, me?.status)}
          />
          <span className="min-w-0">
            <span className="block truncate text-sm font-semibold">{me?.displayName}</span>
            <span className="block truncate text-xs text-muted">
              {/* Status yang dipasang sendiri menggantikan keterangan koneksi:
                  yang pertama dinyatakan orangnya dengan sengaja, yang kedua
                  cuma kabar tentang jaringan. */}
              {statusLabel(me?.status, me?.statusText, me?.statusExpiresAt) ||
                (connected ? 'Tersambung' : 'Menyambungkan ulang…')}
            </span>
          </span>
        </button>
        <div className="flex shrink-0 items-center gap-0.5">
          {/* Tombol notifikasi hanya muncul kalau memang ada yang bisa
              dilakukan: browser mendukungnya DAN server menyalakannya. Tombol
              yang selalu ada tapi kadang tidak berefek lebih buruk daripada
              tombol yang tidak ada. */}
          {push.supported && push.available && (
            <button
              onClick={push.toggle}
              disabled={push.busy || push.blocked}
              title={
                push.blocked
                  ? 'Izin notifikasi diblokir di pengaturan browser'
                  : push.enabled
                    ? 'Matikan notifikasi'
                    : 'Nyalakan notifikasi saat aplikasi ditutup'
              }
              aria-label={push.enabled ? 'Matikan notifikasi' : 'Nyalakan notifikasi'}
              className="rounded-md px-1.5 py-1 text-sm transition hover:bg-canvas disabled:opacity-40"
            >
              {push.enabled ? '🔔' : '🔕'}
            </button>
          )}
          <button
            onClick={logout}
            className="rounded-md px-2 py-1 text-xs text-muted transition hover:bg-canvas hover:text-ink"
          >
            Keluar
          </button>
        </div>
      </header>

      <div className="px-3 py-2">
        <button
          onClick={() => setDialogOpen(true)}
          className="w-full rounded-lg border border-line px-3 py-2 text-sm text-muted transition hover:border-accent hover:text-ink"
        >
          + Percakapan baru
        </button>
      </div>

      <nav className="flex-1 overflow-y-auto px-2 pb-2">
        {conversations.length === 0 && (
          <p className="px-2 py-8 text-center text-sm text-muted">Belum ada percakapan.</p>
        )}

        {conversations.map(c => {
          const isOnline = c.peer ? online.has(c.peer.id) : false;
          // Status datang dari peta siaran, bukan dari salinan di dalam `peer`:
          // yang kedua membeku pada saat daftar percakapan diambil, dan daftar
          // itu tidak diambil ulang setiap kali seseorang memasang statusnya.
          const peerStatus = c.peer ? statuses[c.peer.id] : undefined;
          return (
            <button
              key={c.id}
              onClick={() => void openConversation(c.id)}
              className={`mb-0.5 flex w-full items-center gap-3 rounded-lg px-3 py-2.5 text-left transition ${
                c.id === activeId ? 'bg-accent-soft' : 'hover:bg-canvas'
              }`}
            >
              <Avatar
                name={conversationTitle(c)}
                url={c.peer?.avatarUrl}
                size={36}
                // Grup tidak punya titik keadaan: "online" untuk sekumpulan
                // orang tidak punya arti yang bisa dijelaskan.
                dot={c.type === 'direct' ? dotFor(isOnline, peerStatus?.status) : undefined}
              />

              <span className="min-w-0 flex-1">
                <span className="block truncate text-sm font-medium">{conversationTitle(c)}</span>
                <span className="block truncate text-xs text-muted">{preview(c)}</span>
              </span>

              <span className="flex shrink-0 items-center gap-1">
                {/* Penanda sebutan berdiri SENDIRI, di samping badge belum
                    dibaca — bukan menggantikannya.

                    Keduanya menjawab pertanyaan yang berbeda: "ada berapa yang
                    belum kubaca" dan "apakah ada yang memanggilku". Yang kedua
                    bertahan walau yang pertama sudah nol, karena membuka ruang
                    sekilas bukan berarti sudah melihat panggilannya. */}
                {c.mentionSeq > c.mentionAckSeq && (
                  <span
                    title="Ada yang menyebut kamu"
                    className="grid size-5 place-items-center rounded-full bg-amber-400 text-[11px] font-bold text-amber-950"
                  >
                    @
                  </span>
                )}
                {c.unread > 0 && (
                  <span className="rounded-full bg-accent px-1.5 py-0.5 text-[11px] font-medium text-white">
                    {c.unread > 99 ? '99+' : c.unread}
                  </span>
                )}
              </span>
            </button>
          );
        })}
      </nav>

      {dialogOpen && <NewChatDialog onClose={() => setDialogOpen(false)} />}
    </aside>
  );
}
