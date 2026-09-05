import { useState, type FormEvent } from 'react';
import { api } from '../api';
import { useStore } from '../store';

export default function AuthPage() {
  const setMe = useStore(s => s.setMe);
  const loadConversations = useStore(s => s.loadConversations);

  const [mode, setMode] = useState<'login' | 'register'>('login');
  const [username, setUsername] = useState('');
  const [displayName, setDisplayName] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setError('');
    setBusy(true);
    try {
      const user =
        mode === 'login'
          ? await api.login(username, password)
          : await api.register(username, displayName, password);
      setMe(user);
      await loadConversations();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Gagal masuk');
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex h-full items-center justify-center p-6">
      <form
        onSubmit={submit}
        className="w-full max-w-sm rounded-2xl border border-line bg-surface p-8 shadow-sm"
      >
        <h1 className="text-xl font-semibold tracking-tight">
          {mode === 'login' ? 'Masuk' : 'Buat akun'}
        </h1>
        <p className="mt-1 text-sm text-muted">
          {mode === 'login' ? 'Lanjutkan percakapan kamu.' : 'Sekali daftar, langsung bisa chat.'}
        </p>

        <div className="mt-6 space-y-3">
          <Field label="Username" value={username} onChange={setUsername} autoFocus />
          {mode === 'register' && (
            <Field label="Nama tampilan" value={displayName} onChange={setDisplayName} />
          )}
          <Field label="Password" value={password} onChange={setPassword} type="password" />
        </div>

        {error && (
          <p className="mt-4 rounded-lg bg-red-500/10 px-3 py-2 text-sm text-red-600 dark:text-red-400">
            {error}
          </p>
        )}

        <button
          type="submit"
          disabled={busy}
          className="mt-6 w-full rounded-lg bg-accent px-4 py-2.5 text-sm font-medium text-white transition hover:opacity-90 disabled:opacity-50"
        >
          {busy ? 'Memproses…' : mode === 'login' ? 'Masuk' : 'Daftar'}
        </button>

        <button
          type="button"
          onClick={() => {
            setMode(mode === 'login' ? 'register' : 'login');
            setError('');
          }}
          className="mt-4 w-full text-center text-sm text-muted transition hover:text-ink"
        >
          {mode === 'login' ? 'Belum punya akun? Daftar' : 'Sudah punya akun? Masuk'}
        </button>
      </form>
    </div>
  );
}

function Field({
  label,
  value,
  onChange,
  type = 'text',
  autoFocus,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  type?: string;
  autoFocus?: boolean;
}) {
  return (
    <label className="block">
      <span className="text-xs font-medium text-muted">{label}</span>
      <input
        type={type}
        value={value}
        autoFocus={autoFocus}
        onChange={e => onChange(e.target.value)}
        className="mt-1 w-full rounded-lg border border-line bg-canvas px-3 py-2 text-sm outline-none transition focus:border-accent"
      />
    </label>
  );
}
