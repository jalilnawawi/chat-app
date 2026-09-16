import { useEffect, useState, type FormEvent } from 'react';
import { ApiError, api } from '../api';
import { useStore } from '../store';

/**
 * Halaman masuk, daftar, dan pemulihan akun.
 *
 * Empat mode di satu berkas, bukan empat rute. Aplikasi ini memang tidak punya
 * router — lihat catatan di App.tsx — tapi alasannya lebih dari itu: keempatnya
 * adalah satu percakapan yang sama dengan orang yang sedang berdiri di depan
 * pintu, dan berpindah di antaranya tidak boleh memuat ulang halaman.
 *
 * Mode `reset` tidak pernah dipilih orang. Dia muncul sendiri saat halaman
 * dibuka lewat tautan pemulihan dari kotak masuk.
 */
type Mode = 'login' | 'register' | 'forgot' | 'reset';

export default function AuthPage({ notice }: { notice?: string | null }) {
  const setMe = useStore(s => s.setMe);
  const loadConversations = useStore(s => s.loadConversations);
  // Tanpa SMTP, "Lupa password?" mengantar ke jalan buntu — dan orang yang lupa
  // password adalah orang yang paling tidak punya cadangan kesabaran.
  const mail = useStore(s => s.config?.mail ?? false);

  const [mode, setMode] = useState<Mode>('login');
  const [username, setUsername] = useState('');
  const [displayName, setDisplayName] = useState('');
  const [password, setPassword] = useState('');
  const [email, setEmail] = useState('');
  const [resetToken, setResetToken] = useState('');
  const [error, setError] = useState('');
  const [info, setInfo] = useState('');
  const [busy, setBusy] = useState(false);

  // Tautan pemulihan membawa tokennya di query string. Parameternya dibuang
  // setelah dibaca supaya reload berikutnya tidak menampilkan formulir
  // pemulihan dengan token yang sudah habis sekali pakainya.
  useEffect(() => {
    const token = new URLSearchParams(location.search).get('reset');
    if (!token) return;

    history.replaceState(null, '', location.pathname);
    setResetToken(token);
    setMode('reset');
  }, []);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setError('');
    setInfo('');
    setBusy(true);
    try {
      switch (mode) {
        case 'login':
          setMe(await api.login(username, password));
          await loadConversations();
          break;
        case 'register':
          setMe(await api.register(username, displayName, password));
          await loadConversations();
          break;
        case 'forgot': {
          // Jawaban server selalu sama, terdaftar atau tidak — dan kalimat yang
          // ditampilkan di sini harus ikut seragam. Menampilkan "alamat tidak
          // ditemukan" di client membatalkan seluruh gunanya.
          const got = await api.forgotPassword(email);
          setInfo(got.status);
          break;
        }
        case 'reset':
          await api.resetPassword(resetToken, password);
          setPassword('');
          setMode('login');
          setInfo('Password sudah diganti. Silakan masuk dengan yang baru.');
          break;
      }
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Gagal, coba lagi');
    } finally {
      setBusy(false);
    }
  }

  const pindah = (next: Mode) => {
    setMode(next);
    setError('');
    setInfo('');
  };

  return (
    <div className="flex h-full items-center justify-center p-6">
      <form
        onSubmit={submit}
        className="w-full max-w-sm rounded-2xl border border-line bg-surface p-8 shadow-sm"
      >
        <h1 className="text-xl font-semibold tracking-tight">{JUDUL[mode]}</h1>
        <p className="mt-1 text-sm text-muted">{KETERANGAN[mode]}</p>

        {notice && (
          <p className="mt-4 rounded-lg bg-accent-soft px-3 py-2 text-sm">{notice}</p>
        )}

        <div className="mt-6 space-y-3">
          {(mode === 'login' || mode === 'register') && (
            <Field label="Username" value={username} onChange={setUsername} autoFocus />
          )}
          {mode === 'register' && (
            <Field label="Nama tampilan" value={displayName} onChange={setDisplayName} />
          )}
          {mode === 'forgot' && (
            <Field label="Email" value={email} onChange={setEmail} type="email" autoFocus />
          )}
          {mode !== 'forgot' && (
            <Field
              label={mode === 'reset' ? 'Password baru' : 'Password'}
              value={password}
              onChange={setPassword}
              type="password"
              autoFocus={mode === 'reset'}
            />
          )}
        </div>

        {info && (
          <p className="mt-4 rounded-lg bg-emerald-500/10 px-3 py-2 text-sm text-emerald-700 dark:text-emerald-400">
            {info}
          </p>
        )}
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
          {busy ? 'Memproses…' : TOMBOL[mode]}
        </button>

        <div className="mt-4 flex flex-col gap-2 text-center text-sm">
          {mode === 'login' && (
            <>
              <button type="button" onClick={() => pindah('register')} className={tautan}>
                Belum punya akun? Daftar
              </button>
              {mail && (
                <button type="button" onClick={() => pindah('forgot')} className={tautan}>
                  Lupa password?
                </button>
              )}
            </>
          )}
          {mode !== 'login' && (
            <button type="button" onClick={() => pindah('login')} className={tautan}>
              {mode === 'register' ? 'Sudah punya akun? Masuk' : 'Kembali ke halaman masuk'}
            </button>
          )}
        </div>
      </form>
    </div>
  );
}

const tautan = 'text-muted transition hover:text-ink';

const JUDUL: Record<Mode, string> = {
  login: 'Masuk',
  register: 'Buat akun',
  forgot: 'Pulihkan akun',
  reset: 'Pasang password baru',
};

const KETERANGAN: Record<Mode, string> = {
  login: 'Lanjutkan percakapan kamu.',
  register: 'Sekali daftar, langsung bisa chat.',
  // Syaratnya dikatakan di muka. Orang yang alamatnya belum pernah
  // diverifikasi akan menunggu surat yang tidak akan pernah datang, dan
  // menunggu tanpa tahu sebabnya adalah bentuk kegagalan yang paling lama
  // dirasakan.
  forgot: 'Kami kirim tautan ke alamat email yang sudah terverifikasi.',
  reset: 'Semua perangkat akan dikeluarkan setelah ini.',
};

const TOMBOL: Record<Mode, string> = {
  login: 'Masuk',
  register: 'Daftar',
  forgot: 'Kirim tautan',
  reset: 'Simpan password baru',
};

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
