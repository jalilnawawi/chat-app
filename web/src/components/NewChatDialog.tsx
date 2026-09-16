import { useEffect, useState } from 'react';
import { api } from '../api';
import { useStore } from '../store';
import type { User } from '../types';
import Avatar from './Avatar';
import Icon from './Icon';

export default function NewChatDialog({ onClose }: { onClose: () => void }) {
  const loadConversations = useStore(s => s.loadConversations);
  const openConversation = useStore(s => s.openConversation);

  const [query, setQuery] = useState('');
  const [results, setResults] = useState<User[]>([]);
  const [selected, setSelected] = useState<User[]>([]);
  const [groupTitle, setGroupTitle] = useState('');
  const [busy, setBusy] = useState(false);

  // Debounce supaya tiap ketikan tidak jadi satu permintaan ke server.
  useEffect(() => {
    const t = setTimeout(() => {
      api
        .searchUsers(query)
        .then(setResults)
        .catch(() => setResults([]));
    }, 200);
    return () => clearTimeout(t);
  }, [query]);

  const isGroup = selected.length > 1;

  function toggle(user: User) {
    setSelected(prev =>
      prev.some(u => u.id === user.id) ? prev.filter(u => u.id !== user.id) : [...prev, user],
    );
  }

  async function start() {
    if (selected.length === 0) return;
    setBusy(true);
    try {
      const res = isGroup
        ? await api.createGroup(groupTitle.trim() || 'Grup baru', selected.map(u => u.id))
        : await api.openDirect(selected[0]!.id);
      await loadConversations();
      await openConversation(res.conversationId);
      onClose();
    } finally {
      setBusy(false);
    }
  }

  // Escape menutup dialog. Jalan keluar yang sama dengan menekan di luar kotak,
  // untuk orang yang tangannya sedang di papan ketik — mereka baru saja
  // mengetik nama di kolom pencarian.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, [onClose]);

  return (
    <div
      className="fixed inset-0 z-30 flex items-end justify-center bg-ink/40 p-0 backdrop-blur-[2px] sm:items-center sm:p-4"
      onClick={onClose}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-label="Percakapan baru"
        // Di ponsel dia naik dari bawah dan menempel ke tepi layar; di layar
        // lebar dia mengapung di tengah. Bentuk yang sama di keduanya berarti
        // salah satunya selalu berdiri di tempat yang canggung.
        className="flex max-h-[85vh] w-full flex-col rounded-t-[24px] border border-line bg-surface p-5 shadow-pop sm:max-w-md sm:rounded-[24px]"
        onClick={e => e.stopPropagation()}
      >
        <h2 className="text-lg font-bold tracking-tight">Percakapan baru</h2>
        <p className="mt-1 text-[13px] leading-relaxed text-muted">
          Pilih satu orang untuk chat pribadi, atau beberapa untuk membuat grup.
        </p>

        <div className="relative mt-4">
          <span className="pointer-events-none absolute top-1/2 left-3.5 -translate-y-1/2 text-muted">
            <Icon name="cari" size={18} />
          </span>
          <input
            autoFocus
            value={query}
            onChange={e => setQuery(e.target.value)}
            placeholder="Cari nama atau username…"
            className="w-full rounded-xl border border-line-strong bg-canvas py-2.5 pr-3.5 pl-11 text-[15px] outline-none transition focus:bg-surface"
          />
        </div>

        {isGroup && (
          <input
            value={groupTitle}
            onChange={e => setGroupTitle(e.target.value)}
            placeholder="Nama grup"
            className="mt-2 w-full rounded-xl border border-line-strong bg-canvas px-3.5 py-2.5 text-[15px] outline-none transition focus:bg-surface"
          />
        )}

        <ul className="-mx-1 mt-3 flex-1 overflow-y-auto px-1">
          {results.length === 0 && (
            <li className="py-8 text-center text-sm text-muted">Tidak ada pengguna ditemukan.</li>
          )}
          {results.map(u => {
            const picked = selected.some(s => s.id === u.id);
            return (
              <li key={u.id}>
                <button
                  onClick={() => toggle(u)}
                  aria-pressed={picked}
                  className={`mb-0.5 flex w-full items-center gap-3 rounded-xl px-2.5 py-2 text-left transition ${
                    picked ? 'bg-accent-soft' : 'hover:bg-canvas'
                  }`}
                >
                  <Avatar name={u.displayName} url={u.avatarUrl} size={40} />
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-[15px] font-semibold">
                      {u.displayName}
                    </span>
                    <span className="block truncate text-[13px] text-muted">@{u.username}</span>
                  </span>
                  {/* Kotak centang, bukan kata "dipilih": bentuknya sendiri
                      sudah mengatakan bahwa ini bisa lebih dari satu. */}
                  <span
                    className={`grid size-6 shrink-0 place-items-center rounded-lg border-2 transition ${
                      picked
                        ? 'border-accent bg-accent text-accent-ink'
                        : 'border-line-strong text-transparent'
                    }`}
                  >
                    <Icon name="terbaca" size={13} />
                  </span>
                </button>
              </li>
            );
          })}
        </ul>

        <div className="mt-4 flex justify-end gap-2">
          <button
            onClick={onClose}
            className="rounded-xl px-4 py-2.5 text-sm font-medium text-muted transition hover:bg-canvas hover:text-ink"
          >
            Batal
          </button>
          <button
            onClick={start}
            disabled={selected.length === 0 || busy}
            className="rounded-xl bg-accent px-4 py-2.5 text-sm font-semibold text-accent-ink transition hover:brightness-110 disabled:opacity-40 disabled:hover:brightness-100"
          >
            {isGroup ? `Buat grup (${selected.length})` : 'Mulai chat'}
          </button>
        </div>
      </div>
    </div>
  );
}
