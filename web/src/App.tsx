import { useEffect, useState } from 'react';
import { api } from './api';
import { registerServiceWorker } from './push';
import { useStore } from './store';
import { useSocket } from './useSocket';
import AuthPage from './components/AuthPage';
import Sidebar from './components/Sidebar';
import ChatPanel from './components/ChatPanel';
import AccountPanel from './components/AccountPanel';

export default function App() {
  const me = useStore(s => s.me);
  const setMe = useStore(s => s.setMe);
  const loadConfig = useStore(s => s.loadConfig);
  const loadConversations = useStore(s => s.loadConversations);
  const [checking, setChecking] = useState(true);
  const [accountOpen, setAccountOpen] = useState(false);
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
      <div className="flex h-full items-center justify-center text-muted">Memuat…</div>
    );
  }

  if (!me) return <AuthPage notice={verified} />;

  return (
    <div className="mx-auto flex h-full max-w-6xl flex-col overflow-hidden border-line sm:border-x">
      {verified && (
        <p className="flex items-center justify-between gap-3 border-b border-line bg-accent-soft px-4 py-2 text-sm">
          {verified}
          <button
            onClick={() => setVerified(null)}
            aria-label="Tutup kabar"
            className="shrink-0 rounded px-1.5 text-muted transition hover:text-ink"
          >
            ✕
          </button>
        </p>
      )}
      <div className="flex min-h-0 flex-1">
        <Sidebar accountOpen={accountOpen} onToggleAccount={() => setAccountOpen(v => !v)} />
        <ChatPanel send={socket.send} />
        {accountOpen && <AccountPanel onClose={() => setAccountOpen(false)} />}
      </div>
    </div>
  );
}
