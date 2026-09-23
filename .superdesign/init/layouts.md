# Shared Layouts

The app is a single-screen shell, not a routed multi-page app. `App.tsx` renders one flex row: `Sidebar` (conversation list) + `ChatPanel` (active conversation) + at most one right-hand panel (`ProfilePanel` / `AkunPanel` / `PreferensiPanel` / `SearchPanel`).

Responsive rule (important): on narrow screens the sidebar and the chat panel **alternate** inside the same column — driven by `activeId`, not by a separate view state. `Sidebar` takes `hiddenOnMobile={activeId !== null}`; `ChatPanel` hides itself on mobile when no conversation is open.

## App

- File: `web/src/App.tsx`

```tsx
import { useEffect, useState } from 'react';
import { api } from './api';
import { registerServiceWorker } from './push';
import { useStore } from './store';
import { pantauSistem } from './tema';
import { useSocket } from './useSocket';
import AuthPage from './components/AuthPage';
import Sidebar, { type PanelAktif } from './components/Sidebar';
import ChatPanel from './components/ChatPanel';
import ProfilePanel from './components/ProfilePanel';
import AkunPanel from './components/AkunPanel';
import PreferensiPanel from './components/PreferensiPanel';
import SearchPanel from './components/SearchPanel';
import Icon from './components/Icon';
import Tanda from './components/Tanda';

export default function App() {
  const me = useStore(s => s.me);
  const setMe = useStore(s => s.setMe);
  const activeId = useStore(s => s.activeId);
  const loadConfig = useStore(s => s.loadConfig);
  const loadConversations = useStore(s => s.loadConversations);
  const [checking, setChecking] = useState(true);
  /**
   * Panel kanan yang sedang terbuka: profil, akun, atau preferensi.
   *
   * Satu nilai, bukan tiga saklar. Ketiganya menempati kolom yang sama, jadi
   * keadaan "dua terbuka sekaligus" bukan sesuatu yang perlu bisa diwakili —
   * dan yang tidak bisa diwakili tidak perlu dijaga supaya tidak terjadi.
   */
  const [panel, setPanel] = useState<PanelAktif | undefined>(undefined);
  /**
   * Panel pencarian: undefined = tertutup, null = semua percakapan, string =
   * satu percakapan. Panel kanan dan panel pencarian menempati kolom yang sama,
   * jadi membuka yang satu menutup yang lain.
   */
  const [search, setSearch] = useState<string | null | undefined>(undefined);
  /**
   * Hasil membuka tautan verifikasi email.
   *
   * Ditangani SEBELUM sesi diperiksa, dan sengaja tidak menuntut login:
   * tautannya diklik dari kotak masuk, sering di perangkat yang berbeda dari
   * tempat akunnya dipakai. Menuntut login lebih dulu berarti menuntut orang
   * memasukkan password justru pada jalur yang ada untuk membuktikan hal lain.
   */
  const [verified, setVerified] = useState<string | null>(null);

  // Tautan verifikasi. Parameternya dibuang setelah dipakai supaya reload
  // berikutnya tidak mencoba memakai token yang sudah habis sekali pakainya —
  // dan gagal dengan pesan yang membingungkan.
  useEffect(() => {
    const token = new URLSearchParams(location.search).get('verify');
    if (!token) return;

    history.replaceState(null, '', location.pathname);
    api
      .verifyEmail(token)
      .then(got => setVerified(`Alamat ${got.email} terverifikasi.`))
      .catch(err => setVerified(err instanceof Error ? err.message : 'Verifikasi gagal.'));
  }, []);

  // Pemantau tema dipasang untuk SELURUH aplikasi, bukan di dalam panel akun
  // tempat saklarnya berada: ponsel yang dijadwalkan masuk mode malam berganti
  // sendiri pada jam tertentu, dan panel itu hampir selalu tertutup saat jam itu
  // tiba. Yang dipantau bukan saklarnya, melainkan sistemnya.
  useEffect(() => pantauSistem(), []);

  // Kemampuan server ditanyakan SEKALI, dan tidak menunggu sesi: halaman masuk
  // sudah membutuhkannya untuk memutuskan apakah "Lupa password?" pantas
  // ditawarkan. Sengaja tidak ikut menahan `checking` di bawah — layar yang
  // menahan seluruh aplikasi demi daftar tombol adalah layar yang menukar
  // sesuatu yang penting dengan sesuatu yang tidak.
  useEffect(() => {
    void loadConfig();
  }, [loadConfig]);

  // Cek sesi yang masih hidup dari cookie sebelum menampilkan apa pun, supaya
  // reload halaman tidak memaksa login ulang.
  useEffect(() => {
    api
      .me()
      .then(user => {
        setMe(user);
        return loadConversations();
      })
      .catch(() => {})
      .finally(() => setChecking(false));
  }, [setMe, loadConversations]);

  const socket = useSocket(me !== null);

  // Service worker didaftarkan begitu ada sesi, TERPISAH dari permintaan izin
  // notifikasi. Mendaftar tidak menampilkan apa pun ke orang; yang akan
  // memunculkan dialog izin hanyalah tombol di sidebar. Memisahkan keduanya
  // membuat penekanan tombol itu terasa seketika, karena worker-nya sudah siap.
  useEffect(() => {
    if (!me) return;
    void registerServiceWorker();
  }, [me]);

  // Mengklik notifikasi membuka percakapannya.
  //
  // Service worker mengirim pesan alih-alih menavigasi, karena aplikasi ini
  // tidak punya rute per percakapan dan navigasi akan memuat ulang seluruh
  // halaman — termasuk memutus lalu menyambung kembali WebSocket-nya.
  useEffect(() => {
    if (!me || !('serviceWorker' in navigator)) return;

    const onMessage = (e: MessageEvent) => {
      const data = e.data as { type?: string; conversationId?: string } | null;
      if (data?.type === 'open-conversation' && data.conversationId) {
        void useStore.getState().openConversation(data.conversationId);
      }
    };

    navigator.serviceWorker.addEventListener('message', onMessage);
    return () => navigator.serviceWorker.removeEventListener('message', onMessage);
  }, [me]);

  // Notifikasi yang diklik saat tidak ada tab terbuka membuka jendela baru
  // dengan ?c=<id>. Parameternya dibuang setelah dipakai supaya reload
  // berikutnya tidak melompat lagi ke percakapan yang sama.
  useEffect(() => {
    if (!me) return;

    const wanted = new URLSearchParams(location.search).get('c');
    if (!wanted) return;

    history.replaceState(null, '', location.pathname);
    void useStore.getState().openConversation(wanted);
  }, [me]);

  if (checking) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-3">
        <Tanda />
        <p className="text-sm text-muted">Memuat…</p>
      </div>
    );
  }

  if (!me) return <AuthPage notice={verified} />;

  return (
    <div className="mx-auto flex h-full max-w-[1480px] flex-col overflow-hidden border-line lg:border-x">
      {verified && (
        <p className="flex items-center justify-between gap-3 border-b border-line bg-ok-soft px-4 py-2.5 text-sm text-ink">
          {verified}
          <button
            onClick={() => setVerified(null)}
            aria-label="Tutup kabar"
            className="grid size-7 shrink-0 place-items-center rounded-lg text-muted transition hover:bg-surface hover:text-ink"
          >
            <Icon name="tutup" size={16} />
          </button>
        </p>
      )}
      {/*
        Di layar sempit, daftar percakapan dan isi percakapan BERGANTIAN mengisi
        satu kolom yang sama — bukan berbagi lebar yang tidak cukup untuk
        keduanya. Sebelumnya sidebar 288px tetap berdiri di layar 360px dan
        menyisakan 72px untuk gelembung pesan, yang artinya aplikasi ini tidak
        bisa dipakai di benda yang paling sering dipakai membukanya.

        Yang menentukan siapa yang tampil adalah activeId, bukan state tampilan
        tersendiri: keduanya akan selalu berbeda pada akhirnya, dan yang kalah
        adalah tombol kembali yang tidak mengembalikan apa-apa.
      */}
      <div className="flex min-h-0 flex-1">
        <Sidebar
          hiddenOnMobile={activeId !== null}
          panel={panel}
          onPanel={p => {
            setSearch(undefined);
            setPanel(v => (v === p ? undefined : p));
          }}
          searchOpen={search !== undefined}
          onSearch={() => {
            setPanel(undefined);
            setSearch(v => (v === undefined ? null : undefined));
          }}
        />
        <ChatPanel
          send={socket.send}
          onSearch={id => {
            setPanel(undefined);
            setSearch(id);
          }}
        />
        {panel === 'profil' && <ProfilePanel onClose={() => setPanel(undefined)} />}
        {panel === 'akun' && <AkunPanel onClose={() => setPanel(undefined)} />}
        {panel === 'preferensi' && <PreferensiPanel onClose={() => setPanel(undefined)} />}
        {search !== undefined && (
          <SearchPanel scope={search} onScope={setSearch} onClose={() => setSearch(undefined)} />
        )}
      </div>
    </div>
  );
}
```

## Sidebar

- File: `web/src/components/Sidebar.tsx`

```tsx
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
```

## ChatHeader

- File: `web/src/components/ChatHeader.tsx`

```tsx
import { useStore } from '../store';
import Avatar, { dotFor, statusLabel } from './Avatar';
import IconButton from './ui/IconButton';
import { conversationTitle } from './Sidebar';
import type { Conversation } from '../types';

/** Kepala percakapan: siapa, keadaannya, cari, dan kelola grup. */
export default function ChatHeader({
  conversation,
  memberCount,
  panelOpen,
  onTogglePanel,
  onSearch,
}: {
  conversation: Conversation;
  memberCount: number;
  panelOpen: boolean;
  onTogglePanel: () => void;
  onSearch: () => void;
}) {
  const closeConversation = useStore(s => s.closeConversation);
  const online = useStore(s => s.online);
  const statuses = useStore(s => s.statuses);

  const peerOnline = conversation.peer ? online.has(conversation.peer.id) : false;
  // Status dibaca dari peta siaran, bukan dari salinan di dalam `peer`: yang
  // kedua membeku pada saat daftar percakapan diambil.
  const peerStatus = conversation.peer ? statuses[conversation.peer.id] : undefined;
  const title = conversationTitle(conversation);

  return (
    <header className="flex items-center gap-2 border-b border-line bg-surface px-2 py-2.5 md:px-4 md:py-3">
      {/* Hanya ada di layar sempit, tempat daftar percakapan benar-benar pergi
          saat sebuah percakapan dibuka. Di layar lebar keduanya bersebelahan,
          dan tombol kembali tidak mengembalikan apa pun. */}
      <IconButton
        icon="kembali"
        label="Kembali ke daftar percakapan"
        className="md:hidden"
        onClick={closeConversation}
      />

      <Avatar
        name={title}
        url={conversation.peer?.avatarUrl}
        size={40}
        grup={conversation.type === 'group'}
        dot={conversation.type === 'direct' ? dotFor(peerOnline, peerStatus?.status) : undefined}
      />
      <div className="min-w-0 flex-1">
        <h2 className="truncate text-[15px] font-bold">{title}</h2>
        <p className="truncate text-[13px] text-muted">
          {conversation.type === 'group'
            ? `${memberCount} anggota`
            : // Status yang dipasang orangnya menggantikan Online/Offline.
              // Yang pertama dinyatakan dengan sengaja; yang kedua cuma kabar
              // tentang apakah tabnya kebetulan terbuka — dan "Online" di
              // sebelah orang yang baru saja menulis "sedang rapat" adalah
              // undangan untuk mengganggunya.
              statusLabel(peerStatus?.status, peerStatus?.text, peerStatus?.expiresAt) ||
              (peerOnline ? 'Online' : 'Offline')}
        </p>
      </div>

      <IconButton icon="cari" label="Cari di percakapan ini" onClick={onSearch} />

      {/* Hanya grup yang punya pengelolaan. DM tidak bisa ditambahi orang —
          lihat catatan kebocoran di server/internal/store/group.go — jadi
          tombolnya memang tidak ada di sana, bukan ada tapi menolak. */}
      {conversation.type === 'group' && (
        <IconButton icon="anggota" label="Kelola grup" pressed={panelOpen} onClick={onTogglePanel} />
      )}
    </header>
  );
}
```

## PanelShell

- File: `web/src/components/ui/PanelShell.tsx`

```tsx
import { useEffect, useId, useRef, type ReactNode } from 'react';
import { useMediaQuery } from '../../useMediaQuery';
import IconButton from './IconButton';

/** Di bawah lebar ini panel MENUTUPI layar, bukan berbagi dengannya. */
const OVERLAY = '(max-width: 1023px)';

/**
 * Kerangka panel kanan — profil, akun, preferensi, kelola grup, pencarian.
 *
 * Sebelumnya tiap panel menggambar kerangkanya sendiri, dan itu bukan sekadar
 * pengulangan: panel yang MENUTUPI seluruh layar adalah dialog, dan tidak satu
 * pun dari mereka mengatakannya. Fokus tertinggal di belakang panel, Escape
 * tidak menutup, dan Tab berjalan keluar ke percakapan yang sedang tertutup.
 * Diperbaiki di satu tempat, semua panel ikut sembuh.
 *
 * Yang menentukan dia dialog atau bukan adalah LEBAR, bukan jenis panelnya: di
 * layar lebar dia kolom biasa di sebelah percakapan, dan mengurung fokus di
 * dalam kolom yang tidak menutupi apa pun justru menjebak orangnya.
 *
 * Batasnya lg (1024px), bukan md: di 768px, sidebar 320 dan panel 320
 * menyisakan 128px untuk gelembung pesan — lebar yang sudah pernah membuat
 * aplikasi ini tidak terpakai di ponsel, dan tidak jadi lebih baik hanya karena
 * layarnya tablet.
 */
export default function PanelShell({
  title,
  onClose,
  closeLabel,
  width = 'w-80',
  toolbar,
  footer,
  children,
}: {
  title: string;
  onClose: () => void;
  /** Dibaca pembaca layar sebagai nama tombol tutup; judulnya sendiri terlalu pendek. */
  closeLabel: string;
  /** Lebar kolom di layar lebar. Pencarian butuh lebih dari setelan. */
  width?: 'w-80' | 'w-96';
  /** Bagian yang TIDAK ikut tergulir — kotak pencarian, penyaring. */
  toolbar?: ReactNode;
  footer?: ReactNode;
  children: ReactNode;
}) {
  const overlay = useMediaQuery(OVERLAY);
  const box = useRef<HTMLElement>(null);
  const headingId = useId();

  // Fokus masuk saat panel terbuka dan KEMBALI ke tempat asalnya saat ditutup.
  // Tanpa yang kedua, orang yang menutup panel dengan Escape kehilangan
  // jejaknya: fokus melompat ke awal halaman, dan Tab berikutnya berjalan dari
  // sana, bukan dari tombol yang baru saja dia tekan.
  useEffect(() => {
    const asal = document.activeElement as HTMLElement | null;
    // Panel yang sudah menaruh fokusnya sendiri tidak diganggu: kotak
    // pencarian yang langsung siap diketik adalah seluruh gunanya.
    if (!box.current?.contains(document.activeElement)) box.current?.focus();
    return () => asal?.focus();
  }, []);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && !e.defaultPrevented) {
        onClose();
        return;
      }

      // Fokus dikurung HANYA selagi panel menutupi layar. Di kolom lebar,
      // mengurungnya berarti memenjarakan orang di dalam setelan yang cuma
      // ingin dia lihat sekilas.
      if (e.key !== 'Tab' || !overlay || !box.current) return;

      const bisa = box.current.querySelectorAll<HTMLElement>(
        'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])',
      );
      const awal = bisa[0];
      const akhir = bisa[bisa.length - 1];
      if (!awal || !akhir) return;

      if (e.shiftKey && document.activeElement === awal) {
        e.preventDefault();
        akhir.focus();
      } else if (!e.shiftKey && document.activeElement === akhir) {
        e.preventDefault();
        awal.focus();
      }
    };

    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, [onClose, overlay]);

  return (
    <aside
      ref={box}
      tabIndex={-1}
      role={overlay ? 'dialog' : undefined}
      aria-modal={overlay ? true : undefined}
      aria-labelledby={headingId}
      // Kelasnya ditulis utuh, bukan disusun dari potongan: pemindai Tailwind
      // membaca berkas ini sebagai teks, dan kelas yang baru terbentuk saat
      // program berjalan tidak pernah ikut tercetak ke CSS.
      className={`fixed inset-0 z-30 flex w-full flex-col bg-surface outline-none lg:static lg:z-auto lg:shrink-0 lg:border-l lg:border-line ${
        width === 'w-96' ? 'lg:w-96' : 'lg:w-80'
      }`}
    >
      <header className="flex items-center justify-between gap-2 border-b border-line px-4 py-2.5">
        <h2 id={headingId} className="truncate text-[15px] font-bold">
          {title}
        </h2>
        <IconButton icon="tutup" label={closeLabel} onClick={onClose} />
      </header>

      {toolbar}

      <div className="flex-1 overflow-y-auto overscroll-contain">{children}</div>

      {footer}
    </aside>
  );
}

/**
 * Satu bagian di dalam panel, dengan judulnya sendiri.
 *
 * Judulnya heading sungguhan, bukan paragraf tebal: pembaca layar yang
 * melompat antar-heading sebelumnya menemukan satu benda di seluruh panel
 * setelan, dan harus menyusuri dua puluh kontrol untuk tahu ada apa saja di
 * dalamnya.
 */
export function PanelSection({
  title,
  badge,
  last = false,
  children,
}: {
  title: string;
  /** Lencana kecil di samping judul — keadaan yang menempel pada bagian ini. */
  badge?: ReactNode;
  /** Bagian terakhir tidak diberi garis bawah: di sana tidak ada apa-apa lagi. */
  last?: boolean;
  children: ReactNode;
}) {
  return (
    <section className={`px-4 py-4 ${last ? '' : 'border-b border-line'}`}>
      <h3 className="mb-2 flex items-center gap-2 text-[13px] font-semibold text-muted">
        {title}
        {badge}
      </h3>
      {children}
    </section>
  );
}
```

