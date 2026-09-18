import { useState } from 'react';
import { useStore } from '../store';
import type { Conversation } from '../types';
import Avatar, { dotFor } from './Avatar';
import Icon from './Icon';
import NewChatDialog from './NewChatDialog';
import StatusMenu from './StatusMenu';
import IconButton from './ui/IconButton';
import { attachmentsText } from '../format';

/** Panel yang menempati kolom kanan; undefined berarti tidak ada yang terbuka. */
export type PanelAktif = 'profil' | 'akun' | 'preferensi';

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
      case 'message.pinned':
        return `${ev.actor.name} menyematkan sebuah pesan`;
      case 'message.unpinned':
        return `${ev.actor.name} melepas sematan`;
    }
  }

  if (last.body) return last.forwarded ? `Diteruskan: ${last.body}` : last.body;

  return attachmentsText(last.attachments ?? []);
}

/**
 * Jam pada baris percakapan.
 *
 * Hari ini menampilkan jamnya, kemarin menyebut namanya, sisanya tanggal. Yang
 * dicari orang di daftar ini bukan "kapan persisnya", melainkan "masih hangat
 * atau sudah lama" — dan tiga bentuk itu sudah menjawabnya tanpa satu pun baris
 * yang lebih panjang dari empat huruf.
 */
function waktuRingkas(iso: string): string {
  const at = new Date(iso);
  if (Number.isNaN(at.getTime())) return '';

  const hariIni = new Date();
  const sama = (a: Date, b: Date) => a.toDateString() === b.toDateString();
  if (sama(at, hariIni)) {
    return at.toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit' });
  }

  const kemarin = new Date(hariIni);
  kemarin.setDate(kemarin.getDate() - 1);
  if (sama(at, kemarin)) return 'Kemarin';

  return at.toLocaleDateString('id-ID', { day: 'numeric', month: 'short' });
}

export default function Sidebar({
  hiddenOnMobile,
  panel,
  onPanel,
  searchOpen,
  onSearch,
}: {
  /** Layar sempit hanya memuat satu kolom; saat percakapan terbuka, ini yang mengalah. */
  hiddenOnMobile: boolean;
  panel: PanelAktif | undefined;
  /** Menekan panel yang sedang terbuka menutupnya. */
  onPanel: (p: PanelAktif) => void;
  searchOpen: boolean;
  onSearch: () => void;
}) {
  const me = useStore(s => s.me);
  const conversations = useStore(s => s.conversations);
  const activeId = useStore(s => s.activeId);
  const online = useStore(s => s.online);
  const statuses = useStore(s => s.statuses);
  const connected = useStore(s => s.connected);
  const openConversation = useStore(s => s.openConversation);

  const [dialogOpen, setDialogOpen] = useState(false);

  return (
    <aside
      className={`w-full shrink-0 flex-col border-line bg-surface md:flex md:w-80 md:border-r ${
        hiddenOnMobile ? 'hidden' : 'flex'
      }`}
    >
      {/* Kepala sidebar membuka TIGA panel, bukan satu.
          Foto dan nama sendiri membuka profil — tempat yang sudah dicari orang
          lebih dulu, jauh sebelum mereka mencari ikon. Dua ikon di sebelahnya
          untuk yang jarang: kunci akun, lalu preferensi perangkat ini.

          Baris status berdiri sendiri di bawahnya, dan itu bukan pilihan gaya:
          dia harus bisa ditekan, dan tombol di dalam tombol bukan sesuatu yang
          boleh ditulis. */}
      <header className="border-b border-line px-3 py-2.5">
        <div className="flex items-center gap-1">
          <button
            onClick={() => onPanel('profil')}
            aria-pressed={panel === 'profil'}
            title="Profil kamu"
            className={`flex min-h-10 min-w-0 flex-1 items-center gap-2.5 rounded-xl px-2 text-left transition ${
              panel === 'profil' ? 'bg-accent-soft' : 'hover:bg-canvas'
            }`}
          >
            <Avatar
              name={me?.displayName ?? '?'}
              url={me?.avatarUrl}
              size={36}
              dot={dotFor(connected, me?.status)}
            />
            <span className="min-w-0 truncate text-[15px] font-bold">{me?.displayName}</span>
          </button>

          <IconButton
            icon="kunci"
            label="Akun: email, password, perangkat"
            pressed={panel === 'akun'}
            onClick={() => onPanel('akun')}
          />
          <IconButton
            icon="setelan"
            label="Preferensi perangkat ini"
            pressed={panel === 'preferensi'}
            onClick={() => onPanel('preferensi')}
          />
        </div>

        <div className="mt-0.5">
          <StatusMenu onOpenProfile={() => onPanel('profil')} />
        </div>
      </header>

      <div className="flex gap-2 px-3 py-3">
        <button
          onClick={() => setDialogOpen(true)}
          className="flex flex-1 items-center justify-center gap-2 rounded-xl bg-accent-soft px-3 py-2.5 text-sm font-semibold text-accent-text transition hover:brightness-95"
        >
          <Icon name="tulis" size={18} />
          Percakapan baru
        </button>
        {/* Pencarian pesan berdiri di sebelahnya, bukan di dalam daftar:
            yang dicari di sini adalah ISI percakapan, dan kotak di atas
            daftar percakapan akan dikira menyaring nama. */}
        <button
          onClick={onSearch}
          aria-pressed={searchOpen}
          aria-label="Cari pesan"
          title="Cari pesan di semua percakapan"
          className={`grid size-11 shrink-0 place-items-center rounded-xl border transition ${
            searchOpen
              ? 'border-accent bg-accent-soft text-accent-text'
              : 'border-line-strong text-muted hover:text-ink'
          }`}
        >
          <Icon name="cari" />
        </button>
      </div>

      <nav className="flex-1 overflow-y-auto px-2 pb-3">
        {conversations.length === 0 && (
          <div className="px-4 py-10 text-center">
            <p className="text-sm font-semibold">Belum ada percakapan</p>
            <p className="mt-1.5 text-[13px] leading-relaxed text-muted">
              Cari nama teman lewat tombol di atas, lalu kirim pesan pertama.
            </p>
          </div>
        )}

        {conversations.map(c => {
          const isOnline = c.peer ? online.has(c.peer.id) : false;
          // Status datang dari peta siaran, bukan dari salinan di dalam `peer`:
          // yang kedua membeku pada saat daftar percakapan diambil, dan daftar
          // itu tidak diambil ulang setiap kali seseorang memasang statusnya.
          const peerStatus = c.peer ? statuses[c.peer.id] : undefined;
          const aktif = c.id === activeId;
          const dipanggil = c.mentionSeq > c.mentionAckSeq;
          return (
            <button
              key={c.id}
              onClick={() => void openConversation(c.id)}
              aria-current={aktif ? 'true' : undefined}
              className={`mb-0.5 flex w-full items-center gap-3 rounded-xl px-2.5 py-2.5 text-left transition ${
                aktif ? 'bg-accent-soft' : 'hover:bg-canvas'
              }`}
            >
              <Avatar
                name={conversationTitle(c)}
                url={c.peer?.avatarUrl}
                size={44}
                grup={c.type === 'group'}
                // Grup tidak punya titik keadaan: "online" untuk sekumpulan
                // orang tidak punya arti yang bisa dijelaskan.
                dot={c.type === 'direct' ? dotFor(isOnline, peerStatus?.status) : undefined}
              />

              <span className="min-w-0 flex-1">
                <span className="flex items-baseline gap-2">
                  <span
                    className={`min-w-0 flex-1 truncate text-[15px] ${
                      c.unread > 0 ? 'font-bold' : 'font-semibold'
                    }`}
                  >
                    {conversationTitle(c)}
                  </span>
                  <span className="shrink-0 text-[11.5px] text-muted">
                    {waktuRingkas(c.lastMessage?.createdAt ?? c.updatedAt)}
                  </span>
                </span>
                <span className="mt-0.5 flex items-center gap-2">
                  <span
                    className={`min-w-0 flex-1 truncate text-[13px] ${
                      c.unread > 0 ? 'text-ink' : 'text-muted'
                    }`}
                  >
                    {preview(c)}
                  </span>

                  {/* Penanda sebutan berdiri SENDIRI, di samping badge belum
                      dibaca — bukan menggantikannya.

                      Keduanya menjawab pertanyaan yang berbeda: "ada berapa yang
                      belum kubaca" dan "apakah ada yang memanggilku". Yang kedua
                      bertahan walau yang pertama sudah nol, karena membuka ruang
                      sekilas bukan berarti sudah melihat panggilannya. */}
                  {dipanggil && (
                    <span
                      title="Ada yang menyebut kamu"
                      className="grid size-[22px] shrink-0 place-items-center rounded-full bg-call pt-px text-[13px] leading-none font-extrabold text-call-ink"
                    >
                      @
                    </span>
                  )}
                  {c.unread > 0 && (
                    <span className="min-w-5 shrink-0 rounded-full bg-accent px-1.5 py-0.5 text-center text-[11px] font-bold text-accent-ink tabular-nums">
                      {c.unread > 99 ? '99+' : c.unread}
                    </span>
                  )}
                </span>
              </span>
            </button>
          );
        })}
      </nav>

      {dialogOpen && <NewChatDialog onClose={() => setDialogOpen(false)} />}
    </aside>
  );
}
