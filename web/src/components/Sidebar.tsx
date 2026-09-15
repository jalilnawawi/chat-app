import { useState } from 'react';
import { api } from '../api';
import { unsubscribeThisDevice, usePush } from '../push';
import { useStore } from '../store';
import type { Conversation } from '../types';
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
  if (last.body) return last.body;

  const atts = last.attachments ?? [];
  if (atts.length === 0) return '';
  if (atts.length > 1) return `📎 ${atts.length} lampiran`;
  return atts[0]!.mime.startsWith('image/') ? '📷 Gambar' : `📎 ${atts[0]!.name}`;
}

export default function Sidebar() {
  const me = useStore(s => s.me);
  const conversations = useStore(s => s.conversations);
  const activeId = useStore(s => s.activeId);
  const online = useStore(s => s.online);
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
      <header className="flex items-center justify-between border-b border-line px-4 py-3">
        <div className="min-w-0">
          <p className="truncate text-sm font-semibold">{me?.displayName}</p>
          <p className="flex items-center gap-1.5 text-xs text-muted">
            <span
              className={`inline-block size-1.5 rounded-full ${connected ? 'bg-emerald-500' : 'bg-amber-500'}`}
            />
            {connected ? 'Tersambung' : 'Menyambungkan ulang…'}
          </p>
        </div>
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
          return (
            <button
              key={c.id}
              onClick={() => void openConversation(c.id)}
              className={`mb-0.5 flex w-full items-center gap-3 rounded-lg px-3 py-2.5 text-left transition ${
                c.id === activeId ? 'bg-accent-soft' : 'hover:bg-canvas'
              }`}
            >
              <span className="relative grid size-9 shrink-0 place-items-center rounded-full bg-canvas text-sm font-medium">
                {conversationTitle(c).charAt(0).toUpperCase()}
                {c.type === 'direct' && isOnline && (
                  <span className="absolute right-0 bottom-0 size-2.5 rounded-full border-2 border-surface bg-emerald-500" />
                )}
              </span>

              <span className="min-w-0 flex-1">
                <span className="block truncate text-sm font-medium">{conversationTitle(c)}</span>
                <span className="block truncate text-xs text-muted">{preview(c)}</span>
              </span>

              {c.unread > 0 && (
                <span className="rounded-full bg-accent px-1.5 py-0.5 text-[11px] font-medium text-white">
                  {c.unread > 99 ? '99+' : c.unread}
                </span>
              )}
            </button>
          );
        })}
      </nav>

      {dialogOpen && <NewChatDialog onClose={() => setDialogOpen(false)} />}
    </aside>
  );
}
