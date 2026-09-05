import { useEffect, useState } from 'react';
import { api } from './api';
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
