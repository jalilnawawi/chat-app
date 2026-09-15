import { useEffect, useState } from 'react';
import { api } from './api';
import { registerServiceWorker } from './push';
import { useStore } from './store';
import { useSocket } from './useSocket';
import AuthPage from './components/AuthPage';
import Sidebar from './components/Sidebar';
import ChatPanel from './components/ChatPanel';

export default function App() {
  const me = useStore(s => s.me);
  const setMe = useStore(s => s.setMe);
  const loadConversations = useStore(s => s.loadConversations);
  const [checking, setChecking] = useState(true);

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

  if (!me) return <AuthPage />;

  return (
    <div className="mx-auto flex h-full max-w-6xl overflow-hidden border-line sm:border-x">
      <Sidebar />
      <ChatPanel send={socket.send} />
    </div>
  );
}
