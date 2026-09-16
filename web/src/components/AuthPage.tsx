import { useEffect, useState, type FormEvent } from 'react';
import { ApiError, api } from '../api';
import { useStore } from '../store';
import Tanda from './Tanda';

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
    <div className="flex h-full items-center justify-center overflow-y-auto p-6">
      <div className="w-full max-w-sm py-8">
        <div className="mb-7 flex flex-col items-center gap-2.5">
          <Tanda size={58} />
          <p className="text-2xl font-extrabold tracking-tight">Chat</p>
        </div>

        <form
          onSubmit={submit}
          className="rounded-[20px] border border-line bg-surface p-7 shadow-pop"
        >
          <h1 className="text-xl font-bold tracking-tight">{JUDUL[mode]}</h1>
          <p className="mt-1.5 text-sm leading-relaxed text-muted">{KETERANGAN[mode]}</p>

          {notice && (
            <p className="mt-5 rounded-xl bg-ok-soft px-3.5 py-2.5 text-sm">{notice}</p>
          )}

          <div className="mt-6 space-y-3.5">
            {(mode === 'login' || mode === 'register') && (
              <Field
                label="Username"
                value={username}
                onChange={setUsername}
                autoFocus
                autoComplete="username"
              />
            )}
            {mode === 'register' && (
              <Field
                label="Nama tampilan"
                value={displayName}
                onChange={setDisplayName}
                autoComplete="name"
              />
            )}
            {mode === 'forgot' && (
              <Field
                label="Email"
                value={email}
                onChange={setEmail}
                type="email"
                autoFocus
                autoComplete="email"
              />
            )}
            {mode !== 'forgot' && (
              <Field
                label={mode === 'reset' ? 'Password baru' : 'Password'}
                value={password}
                onChange={setPassword}
                type="password"
                autoFocus={mode === 'reset'}
                // Pengelola password perlu tahu bedanya: yang satu mengisi yang
                // sudah tersimpan, yang satu menawarkan menyimpan yang baru.
                autoComplete={mode === 'login' ? 'current-password' : 'new-password'}
              />
            )}
          </div>

          {info && (
            <p className="mt-5 rounded-xl bg-ok-soft px-3.5 py-2.5 text-sm text-ink">{info}</p>
          )}
          {error && (
            <p className="mt-5 rounded-xl bg-danger-soft px-3.5 py-2.5 text-sm text-danger">
              {error}
            </p>
          )}

          <button
            type="submit"
            disabled={busy}
            className="mt-6 w-full rounded-xl bg-accent px-4 py-3 text-[15px] font-semibold text-accent-ink transition hover:brightness-110 active:scale-[0.99] disabled:opacity-50"
          >
            {busy ? 'Memproses…' : TOMBOL[mode]}
          </button>

          <div className="mt-5 flex flex-col items-center gap-2.5 text-sm">
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
    </div>
  );
}

const tautan = 'rounded text-muted underline-offset-4 transition hover:text-ink hover:underline';

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
  autoComplete,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  type?: string;
  autoFocus?: boolean;
  autoComplete?: string;
}) {
  return (
    <label className="block">
      <span className="text-[13px] font-semibold text-muted">{label}</span>
      <input
        type={type}
        value={value}
        autoFocus={autoFocus}
        autoComplete={autoComplete}
        onChange={e => onChange(e.target.value)}
        className="mt-1.5 w-full rounded-xl border border-line-strong bg-canvas px-3.5 py-2.5 text-[15px] outline-none transition focus:bg-surface"
      />
    </label>
  );
}
