import { useEffect, useState } from 'react';
import { ApiError, api } from '../api';
import { useStore } from '../store';
import type { Conversation, User } from '../types';
import Avatar from './Avatar';

/**
 * Panel kelola grup.
 *
 * Satu aturan yang menentukan seluruh tampilan di bawah: **pemilik mengelola,
 * anggota bisa keluar.** Tombol yang tidak boleh ditekan seseorang tidak
 * ditampilkan kepadanya — bukan ditampilkan lalu ditolak server.
 *
 * Itu bukan sekadar kerapian. Tombol yang selalu ada tapi kadang gagal membuat
 * orang belajar bahwa pesan kesalahan di aplikasi ini boleh diabaikan, dan
 * pelajaran itu terbawa ke pesan kesalahan yang benar-benar penting. Server
 * tetap memeriksa semuanya sendiri: yang di sini adalah tampilan, bukan
 * pengamanan.
 */
export default function GroupPanel({
  conversation,
  onClose,
}: {
  conversation: Conversation;
  onClose: () => void;
}) {
  const me = useStore(s => s.me);
  const members = useStore(s => s.members[conversation.id] ?? []);
  const renameGroup = useStore(s => s.renameGroup);
  const addMembers = useStore(s => s.addMembers);
  const removeMember = useStore(s => s.removeMember);
  const transferOwnership = useStore(s => s.transferOwnership);
  const leaveGroup = useStore(s => s.leaveGroup);

  const [title, setTitle] = useState(conversation.title ?? '');
  const [query, setQuery] = useState('');
  const [found, setFound] = useState<User[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const iAmOwner = members.some(m => m.userId === me?.id && m.role === 'owner');

  // Judul ikut berubah kalau orang lain yang menggantinya selagi panel terbuka,
  // ASALKAN kita tidak sedang mengetiknya sendiri — menimpa kolom yang sedang
  // diketik adalah cara tercepat membuat orang kehilangan kalimatnya.
  useEffect(() => {
    if (!busy) setTitle(conversation.title ?? '');
  }, [conversation.title, busy]);

  useEffect(() => {
    const q = query.trim();
    if (q.length < 2) {
      setFound([]);
      return;
    }
    // Ditunda sebentar: tiap ketukan tombol yang langsung jadi permintaan
    // berarti pencarian yang jawabannya sudah basi sebelum sampai.
    const t = setTimeout(() => {
      void api
        .searchUsers(q)
        .then(users => setFound(users.filter(u => !members.some(m => m.userId === u.id))))
        .catch(() => setFound([]));
    }, 250);
    return () => clearTimeout(t);
  }, [query, members]);

  async function run(action: () => Promise<void>) {
    setBusy(true);
    setError(null);
    try {
      await action();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Gagal, coba lagi');
    } finally {
      setBusy(false);
    }
  }

  const saveTitle = () => {
    const next = title.trim();
    if (!next || next === conversation.title) return;
    void run(() => renameGroup(conversation.id, next));
  };

  return (
    <aside className="flex w-80 shrink-0 flex-col border-l border-line bg-surface">
      <header className="flex items-center justify-between border-b border-line px-4 py-3">
        <h3 className="text-sm font-semibold">Kelola grup</h3>
        <button
          onClick={onClose}
          aria-label="Tutup panel grup"
          className="rounded px-1.5 py-0.5 text-muted transition hover:text-ink"
        >
          ✕
        </button>
      </header>

      <div className="flex-1 overflow-y-auto">
        <section className="border-b border-line px-4 py-3">
          <label className="mb-1 block text-xs font-medium text-muted">Judul</label>
          {iAmOwner ? (
            <input
              value={title}
              disabled={busy}
              maxLength={100}
              onChange={e => setTitle(e.target.value)}
              onBlur={saveTitle}
              onKeyDown={e => {
                if (e.key === 'Enter') {
                  e.preventDefault();
                  saveTitle();
                }
                if (e.key === 'Escape') setTitle(conversation.title ?? '');
              }}
              className="w-full rounded-lg border border-line bg-canvas px-3 py-2 text-sm outline-none focus:border-accent disabled:opacity-50"
            />
          ) : (
            <p className="text-sm">{conversation.title}</p>
          )}
        </section>

        {iAmOwner && (
          <section className="border-b border-line px-4 py-3">
            <label className="mb-1 block text-xs font-medium text-muted">Tambah anggota</label>
            <input
              value={query}
              disabled={busy}
              placeholder="Cari nama atau username…"
              onChange={e => setQuery(e.target.value)}
              className="w-full rounded-lg border border-line bg-canvas px-3 py-2 text-sm outline-none focus:border-accent disabled:opacity-50"
            />
            {found.length > 0 && (
              <ul className="mt-1.5 overflow-hidden rounded-lg border border-line">
                {found.map(u => (
                  <li key={u.id}>
                    <button
                      disabled={busy}
                      onClick={() =>
                        void run(async () => {
                          await addMembers(conversation.id, [u.id]);
                          setQuery('');
                          setFound([]);
                        })
                      }
                      className="block w-full px-3 py-2 text-left text-sm transition hover:bg-canvas disabled:opacity-50"
                    >
                      <span className="font-medium">{u.displayName}</span>
                      <span className="ml-2 text-xs text-muted">@{u.username}</span>
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </section>
        )}

        <section className="px-4 py-3">
          <p className="mb-2 text-xs font-medium text-muted">{members.length} anggota</p>
          <ul className="flex flex-col gap-0.5">
            {members.map(m => {
              const isMe = m.userId === me?.id;
              return (
                <li
                  key={m.userId}
                  className="group flex items-center gap-2 rounded-lg px-2 py-1.5 hover:bg-canvas"
                >
                  <Avatar name={m.displayName} url={m.avatarUrl} size={28} />
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-sm">
                      {m.displayName}
                      {isMe && <span className="ml-1 text-xs text-muted">(kamu)</span>}
                    </span>
                    {m.role === 'owner' && (
                      <span className="block text-[11px] text-accent">Pemilik</span>
                    )}
                  </span>

                  {/* Aksi terhadap anggota lain hanya untuk pemilik, dan tidak
                      pernah terhadap pemilik itu sendiri: pemilik yang bisa
                      mengeluarkan dirinya sendiri lewat sini akan meninggalkan
                      grup tanpa pemilik. Jalan keluarnya ada di bawah. */}
                  {iAmOwner && !isMe && m.role !== 'owner' && (
                    <span className="flex shrink-0 gap-1 opacity-0 transition group-hover:opacity-100 [@media(hover:none)]:opacity-100">
                      <button
                        disabled={busy}
                        title={`Jadikan ${m.displayName} pemilik grup`}
                        onClick={() =>
                          void run(() => transferOwnership(conversation.id, m.userId))
                        }
                        className="rounded px-1.5 py-0.5 text-xs text-muted transition hover:text-ink disabled:opacity-50"
                      >
                        jadikan pemilik
                      </button>
                      <button
                        disabled={busy}
                        title={`Keluarkan ${m.displayName}`}
                        onClick={() => void run(() => removeMember(conversation.id, m.userId))}
                        className="rounded px-1.5 py-0.5 text-xs text-red-500 transition hover:underline disabled:opacity-50"
                      >
                        keluarkan
                      </button>
                    </span>
                  )}
                </li>
              );
            })}
          </ul>
        </section>
      </div>

      {error && (
        <p className="border-t border-line px-4 py-2 text-xs text-red-500">{error}</p>
      )}

      <footer className="border-t border-line px-4 py-3">
        <button
          disabled={busy}
          onClick={() => void run(() => leaveGroup(conversation.id))}
          className="w-full rounded-lg border border-line px-3 py-2 text-sm text-red-500 transition hover:border-red-500 disabled:opacity-50"
        >
          Keluar dari grup
        </button>
        {iAmOwner && members.length > 1 && (
          // Dikatakan di muka, bukan dijelaskan setelah orangnya terlanjur
          // keluar: kepemilikan yang berpindah diam-diam adalah kejutan, dan
          // kejutan pada tindakan yang tidak bisa dibatalkan selalu terasa
          // seperti kesalahan aplikasi.
          <p className="mt-1.5 text-center text-[11px] text-muted">
            Kepemilikan pindah ke anggota terlama
          </p>
        )}
      </footer>
    </aside>
  );
}
