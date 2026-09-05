import { useEffect, useState } from 'react';
import { api } from '../api';
import { useStore } from '../store';
import type { User } from '../types';

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

  return (
    <div
      className="fixed inset-0 z-10 flex items-center justify-center bg-black/40 p-4"
      onClick={onClose}
    >
      <div
        className="w-full max-w-md rounded-2xl border border-line bg-surface p-5 shadow-lg"
        onClick={e => e.stopPropagation()}
      >
        <h2 className="text-base font-semibold">Percakapan baru</h2>
        <p className="mt-1 text-xs text-muted">
          Pilih satu orang untuk chat pribadi, atau beberapa untuk membuat grup.
        </p>

        <input
          autoFocus
          value={query}
          onChange={e => setQuery(e.target.value)}
          placeholder="Cari nama atau username…"
          className="mt-4 w-full rounded-lg border border-line bg-canvas px-3 py-2 text-sm outline-none focus:border-accent"
        />

        {isGroup && (
          <input
            value={groupTitle}
            onChange={e => setGroupTitle(e.target.value)}
            placeholder="Nama grup"
            className="mt-2 w-full rounded-lg border border-line bg-canvas px-3 py-2 text-sm outline-none focus:border-accent"
          />
        )}

        <ul className="mt-3 max-h-64 overflow-y-auto">
          {results.length === 0 && (
            <li className="py-6 text-center text-sm text-muted">Tidak ada pengguna ditemukan.</li>
          )}
          {results.map(u => {
            const picked = selected.some(s => s.id === u.id);
            return (
              <li key={u.id}>
                <button
                  onClick={() => toggle(u)}
                  className={`flex w-full items-center gap-3 rounded-lg px-3 py-2 text-left transition ${
                    picked ? 'bg-accent-soft' : 'hover:bg-canvas'
                  }`}
                >
                  <span className="grid size-8 place-items-center rounded-full bg-canvas text-sm">
                    {u.displayName.charAt(0).toUpperCase()}
                  </span>
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-sm">{u.displayName}</span>
                    <span className="block truncate text-xs text-muted">@{u.username}</span>
                  </span>
                  {picked && <span className="text-xs text-accent">dipilih</span>}
                </button>
              </li>
            );
          })}
        </ul>

        <div className="mt-4 flex justify-end gap-2">
          <button onClick={onClose} className="rounded-lg px-3 py-2 text-sm text-muted hover:text-ink">
            Batal
          </button>
          <button
            onClick={start}
            disabled={selected.length === 0 || busy}
            className="rounded-lg bg-accent px-4 py-2 text-sm font-medium text-white transition hover:opacity-90 disabled:opacity-40"
          >
            {isGroup ? `Buat grup (${selected.length})` : 'Mulai chat'}
          </button>
        </div>
      </div>
    </div>
  );
}
