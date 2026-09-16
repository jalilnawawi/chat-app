import { useEffect, useRef, useState } from 'react';
import { ApiError, api } from '../api';
import { useStore } from '../store';
import type { Session, StatusKind } from '../types';
import Avatar, { untilLabel } from './Avatar';

/**
 * Panel kelola akun: foto, nama, status, email, password, dan perangkat.
 *
 * Satu hal yang menentukan susunannya: yang MENGUBAH KREDENSIAL diletakkan di
 * bawah, di balik kolom password saat ini, terpisah dari yang cuma mengubah
 * tampilan. Foto dan nama adalah pilihan; email dan password adalah kunci
 * rumah, dan keduanya tidak layak ditawarkan dengan bentuk tombol yang sama.
 */
export default function AccountPanel({ onClose }: { onClose: () => void }) {
  const me = useStore(s => s.me);
  if (!me) return null;

  return (
    <aside className="flex w-80 shrink-0 flex-col border-l border-line bg-surface">
      <header className="flex items-center justify-between border-b border-line px-4 py-3">
        <h3 className="text-sm font-semibold">Akun kamu</h3>
        <button
          onClick={onClose}
          aria-label="Tutup panel akun"
          className="rounded px-1.5 py-0.5 text-muted transition hover:text-ink"
        >
          ✕
        </button>
      </header>

      <div className="flex-1 overflow-y-auto">
        <ProfileSection />
        <StatusSection />
        <EmailSection />
        <PasswordSection />
        <SessionSection />
      </div>
    </aside>
  );
}

// ---------- foto & nama ----------

function ProfileSection() {
  const me = useStore(s => s.me)!;
  // Server yang belum menjawab dianggap MATI. Menampilkan tombol lalu
  // menariknya kembali sepersekian detik kemudian lebih buruk daripada
  // menampilkannya sedikit terlambat.
  const avatars = useStore(s => s.config?.avatars ?? false);
  const uploadAvatar = useStore(s => s.uploadAvatar);
  const removeAvatar = useStore(s => s.removeAvatar);
  const updateProfile = useStore(s => s.updateProfile);

  const [name, setName] = useState(me.displayName);
  const [progress, setProgress] = useState<number | null>(null);
  const [error, setError] = useState<string | null>(null);
  const fileInput = useRef<HTMLInputElement>(null);

  // Nama ikut berubah kalau kita menggantinya di tab lain — tapi tidak selagi
  // kolomnya sedang diketik di sini. Menimpa kolom yang sedang diketik adalah
  // cara tercepat membuat orang kehilangan kalimatnya.
  useEffect(() => {
    setName(current => (document.activeElement?.id === 'nama-tampilan' ? current : me.displayName));
  }, [me.displayName]);

  async function pick(file: File | undefined) {
    if (!file) return;
    setError(null);
    setProgress(0);
    try {
      await uploadAvatar(file, setProgress);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Unggahan gagal');
    } finally {
      setProgress(null);
    }
  }

  const saveName = () => {
    const next = name.trim();
    if (!next || next === me.displayName) return;
    void updateProfile(next).catch(err =>
      setError(err instanceof ApiError ? err.message : 'Gagal menyimpan nama'),
    );
  };

  return (
    <section className="border-b border-line px-4 py-4">
      <div className="flex items-center gap-3">
        <Avatar name={me.displayName} url={me.avatarUrl} size={56} />
        <div className="min-w-0 flex-1">
          <p className="truncate text-sm font-medium">@{me.username}</p>

          {/* Tombolnya hanya ada kalau server ini memang bisa menerimanya.
              Tombol yang selalu tampil tapi dijawab "foto profil tidak aktif di
              server ini" mengajari orang bahwa pesan kesalahan di aplikasi ini
              boleh diabaikan — dan pelajaran itu terbawa ke pesan kesalahan
              yang benar-benar penting. */}
          {avatars ? (
            <div className="mt-1 flex gap-2">
              <button
                onClick={() => fileInput.current?.click()}
                disabled={progress !== null}
                className="rounded-md border border-line px-2 py-1 text-xs transition hover:border-accent disabled:opacity-50"
              >
                {progress === null ? 'Ganti foto' : `${Math.round(progress * 100)}%`}
              </button>
              {me.avatarUrl && (
                <button
                  onClick={() => void removeAvatar()}
                  disabled={progress !== null}
                  className="rounded-md px-2 py-1 text-xs text-muted transition hover:text-ink disabled:opacity-50"
                >
                  Hapus
                </button>
              )}
            </div>
          ) : (
            <p className="mt-1 text-[11px] text-muted">Foto profil tidak aktif di server ini.</p>
          )}
        </div>
      </div>

      {avatars && (
        <input
          ref={fileInput}
          type="file"
          accept="image/png,image/jpeg,image/gif,image/webp"
          hidden
          onChange={e => {
            void pick(e.target.files?.[0]);
            // Dikosongkan supaya memilih BERKAS YANG SAMA lagi tetap memicu
            // change — yang dilakukan orang setelah unggahannya gagal.
            e.target.value = '';
          }}
        />
      )}

      <label className="mt-4 block">
        <span className="text-xs font-medium text-muted">Nama tampilan</span>
        <input
          id="nama-tampilan"
          value={name}
          maxLength={60}
          onChange={e => setName(e.target.value)}
          onBlur={saveName}
          onKeyDown={e => {
            if (e.key === 'Enter') {
              e.preventDefault();
              (e.target as HTMLInputElement).blur();
            }
            if (e.key === 'Escape') setName(me.displayName);
          }}
          className="mt-1 w-full rounded-lg border border-line bg-canvas px-3 py-2 text-sm outline-none focus:border-accent"
        />
      </label>

      {/* Username sengaja tidak bisa diganti, dan itu dikatakan di muka alih-alih
          ditunggu sampai orang mencari tombolnya. */}
      <p className="mt-1.5 text-[11px] text-muted">Username tidak bisa diganti.</p>

      {error && <p className="mt-2 text-xs text-red-500">{error}</p>}
    </section>
  );
}

// ---------- status ----------

const STATUS_PILIHAN: { value: StatusKind; label: string; dot: string }[] = [
  { value: 'available', label: 'Tersedia', dot: 'bg-emerald-500' },
  { value: 'busy', label: 'Sibuk', dot: 'bg-red-500' },
  { value: 'away', label: 'Tidak di tempat', dot: 'bg-amber-500' },
];

/**
 * Pilihan durasi cepat.
 *
 * Nilainya dihitung DI SINI, di browser orangnya, dan dikirim sebagai instan
 * absolut. "Sampai akhir hari" hanya punya arti di zona waktu pemakainya —
 * server tidak pernah tahu itu, dan tidak seharusnya perlu tahu.
 */
const DURASI: { label: string; until: () => string | null }[] = [
  { label: 'Tanpa batas', until: () => null },
  { label: '30 menit', until: () => new Date(Date.now() + 30 * 60_000).toISOString() },
  { label: '1 jam', until: () => new Date(Date.now() + 60 * 60_000).toISOString() },
  {
    label: 'Sampai akhir hari',
    until: () => {
      const d = new Date();
      d.setHours(23, 59, 0, 0);
      // Sudah lewat pukul 23.59 berarti "akhir hari" tinggal beberapa detik lagi
      // — dan server menolak batas waktu yang sudah lewat. Jatuh ke tanpa batas.
      return d.getTime() > Date.now() ? d.toISOString() : null;
    },
  },
];

function StatusSection() {
  const me = useStore(s => s.me)!;
  const setStatus = useStore(s => s.setStatus);

  const [text, setText] = useState(me.statusText ?? '');
  const [durasi, setDurasi] = useState(0);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => setText(me.statusText ?? ''), [me.statusText]);

  const simpan = (status: StatusKind, isi: string, index: number) => {
    setError(null);
    void setStatus(status, isi, DURASI[index]!.until()).catch(err =>
      setError(err instanceof ApiError ? err.message : 'Gagal memasang status'),
    );
  };

  const berlaku = untilLabel(me.statusExpiresAt);

  return (
    <section className="border-b border-line px-4 py-4">
      <p className="mb-2 text-xs font-medium text-muted">Status</p>

      <div className="flex gap-1.5">
        {STATUS_PILIHAN.map(p => (
          <button
            key={p.value}
            onClick={() => simpan(p.value, text.trim(), durasi)}
            className={`flex flex-1 items-center justify-center gap-1.5 rounded-lg border px-2 py-1.5 text-xs transition ${
              me.status === p.value ? 'border-accent bg-accent-soft' : 'border-line hover:border-accent'
            }`}
          >
            <span className={`inline-block size-2 rounded-full ${p.dot}`} />
            {p.label}
          </button>
        ))}
      </div>

      <input
        value={text}
        maxLength={120}
        placeholder="Sedang apa? (opsional)"
        onChange={e => setText(e.target.value)}
        onBlur={() => {
          if (text.trim() !== (me.statusText ?? '')) simpan(me.status, text.trim(), durasi);
        }}
        onKeyDown={e => {
          if (e.key === 'Enter') (e.target as HTMLInputElement).blur();
        }}
        className="mt-2 w-full rounded-lg border border-line bg-canvas px-3 py-2 text-sm outline-none focus:border-accent"
      />

      <div className="mt-2 flex flex-wrap gap-1.5">
        {DURASI.map((d, i) => (
          <button
            key={d.label}
            onClick={() => {
              setDurasi(i);
              simpan(me.status, text.trim(), i);
            }}
            className={`rounded-full border px-2.5 py-1 text-[11px] transition ${
              durasi === i ? 'border-accent bg-accent-soft' : 'border-line hover:border-accent'
            }`}
          >
            {d.label}
          </button>
        ))}
      </div>

      {berlaku && <p className="mt-2 text-[11px] text-muted">Berlaku {berlaku}.</p>}

      {/* Dikatakan di muka, bukan ditemukan sendiri: ini satu-satunya hal yang
          membuat status bukan sekadar hiasan. */}
      {me.status === 'busy' && (
        <p className="mt-1.5 text-[11px] text-muted">
          Notifikasi diredam selama sibuk — kecuali kalau ada yang menyebut namamu.
        </p>
      )}

      {error && <p className="mt-2 text-xs text-red-500">{error}</p>}
    </section>
  );
}

// ---------- email ----------

function EmailSection() {
  const me = useStore(s => s.me)!;
  const mail = useStore(s => s.config?.mail ?? false);
  const setMe = useStore(s => s.setMe);

  const [email, setEmail] = useState(me.email ?? '');
  const [password, setPassword] = useState('');
  const [busy, setBusy] = useState(false);
  const [note, setNote] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const berubah = email.trim() !== (me.email ?? '');

  async function simpan() {
    setBusy(true);
    setError(null);
    setNote(null);
    try {
      setMe(await api.setEmail(email.trim(), password));
      setPassword('');
      setNote('Tautan verifikasi dikirim ke alamat itu.');
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Gagal menyimpan email');
    } finally {
      setBusy(false);
    }
  }

  async function kirimUlang() {
    setBusy(true);
    setError(null);
    setNote(null);
    try {
      await api.resendVerification();
      setNote('Tautan verifikasi dikirim ulang.');
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Gagal mengirim ulang');
    } finally {
      setBusy(false);
    }
  }

  // Tanpa SMTP, alamat email tidak bisa diverifikasi — dan alamat yang tidak
  // terverifikasi tidak bisa dipakai memulihkan akun sama sekali. Menawarkan
  // kolomnya berarti menawarkan pekerjaan yang tidak menghasilkan apa-apa.
  //
  // Alamat yang SUDAH tercatat tetap ditampilkan: dia milik orangnya, dan
  // menyembunyikannya karena server sedang tidak bisa mengirim surat berarti
  // menyembunyikan data seseorang dari dirinya sendiri.
  if (!mail) {
    return (
      <section className="border-b border-line px-4 py-4">
        <p className="mb-1 text-xs font-medium text-muted">Email</p>
        {me.email ? (
          <p className="truncate text-sm">{me.email}</p>
        ) : (
          <p className="text-sm text-muted">Belum diisi.</p>
        )}
        <p className="mt-1 text-[11px] text-muted">
          Verifikasi email dan pemulihan password tidak aktif di server ini.
        </p>
      </section>
    );
  }

  return (
    <section className="border-b border-line px-4 py-4">
      <p className="mb-2 flex items-center gap-2 text-xs font-medium text-muted">
        Email
        {me.email &&
          (me.emailVerified ? (
            <span className="rounded-full bg-emerald-500/15 px-2 py-0.5 text-[10px] text-emerald-600 dark:text-emerald-400">
              terverifikasi
            </span>
          ) : (
            <span className="rounded-full bg-amber-500/15 px-2 py-0.5 text-[10px] text-amber-600 dark:text-amber-400">
              belum terverifikasi
            </span>
          ))}
      </p>

      <input
        type="email"
        value={email}
        disabled={busy}
        placeholder="kamu@contoh.com"
        onChange={e => setEmail(e.target.value)}
        className="w-full rounded-lg border border-line bg-canvas px-3 py-2 text-sm outline-none focus:border-accent disabled:opacity-50"
      />

      {/* Kolom password baru muncul saat alamatnya memang berubah. Memintanya
          sepanjang waktu membuat orang mengetik password untuk membaca layar. */}
      {berubah && (
        <input
          type="password"
          value={password}
          disabled={busy}
          placeholder="Password kamu saat ini"
          onChange={e => setPassword(e.target.value)}
          className="mt-2 w-full rounded-lg border border-line bg-canvas px-3 py-2 text-sm outline-none focus:border-accent disabled:opacity-50"
        />
      )}

      <div className="mt-2 flex gap-2">
        {berubah && (
          <button
            onClick={() => void simpan()}
            disabled={busy || !password}
            className="rounded-lg bg-accent px-3 py-1.5 text-xs font-medium text-white transition hover:opacity-90 disabled:opacity-50"
          >
            Simpan email
          </button>
        )}
        {me.email && !me.emailVerified && !berubah && (
          <button
            onClick={() => void kirimUlang()}
            disabled={busy}
            className="rounded-lg border border-line px-3 py-1.5 text-xs transition hover:border-accent disabled:opacity-50"
          >
            Kirim ulang tautan
          </button>
        )}
      </div>

      {/* Alasannya dikatakan, bukan cuma lencananya ditampilkan: tanpa kalimat
          ini, "belum terverifikasi" terlihat seperti kerapian yang bisa
          diabaikan — sampai hari orangnya lupa password. */}
      {me.email && !me.emailVerified && (
        <p className="mt-2 text-[11px] text-muted">
          Sebelum terverifikasi, alamat ini tidak bisa dipakai memulihkan akun.
        </p>
      )}

      {note && <p className="mt-2 text-xs text-emerald-600 dark:text-emerald-400">{note}</p>}
      {error && <p className="mt-2 text-xs text-red-500">{error}</p>}
    </section>
  );
}

// ---------- password ----------

function PasswordSection() {
  const [current, setCurrent] = useState('');
  const [next, setNext] = useState('');
  const [busy, setBusy] = useState(false);
  const [note, setNote] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  async function simpan() {
    setBusy(true);
    setError(null);
    setNote(null);
    try {
      await api.changePassword(current, next);
      setCurrent('');
      setNext('');
      setNote('Password diganti. Semua perangkat lain sudah dikeluarkan.');
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Gagal mengganti password');
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="border-b border-line px-4 py-4">
      <p className="mb-2 text-xs font-medium text-muted">Ganti password</p>

      <input
        type="password"
        value={current}
        disabled={busy}
        placeholder="Password saat ini"
        onChange={e => setCurrent(e.target.value)}
        className="w-full rounded-lg border border-line bg-canvas px-3 py-2 text-sm outline-none focus:border-accent disabled:opacity-50"
      />
      <input
        type="password"
        value={next}
        disabled={busy}
        placeholder="Password baru (min. 8 karakter)"
        onChange={e => setNext(e.target.value)}
        className="mt-2 w-full rounded-lg border border-line bg-canvas px-3 py-2 text-sm outline-none focus:border-accent disabled:opacity-50"
      />

      <button
        onClick={() => void simpan()}
        disabled={busy || !current || next.length < 8}
        className="mt-2 rounded-lg bg-accent px-3 py-1.5 text-xs font-medium text-white transition hover:opacity-90 disabled:opacity-50"
      >
        Ganti password
      </button>

      {/* Akibatnya dikatakan SEBELUM tombolnya ditekan. Perangkat lain yang
          tiba-tiba keluar tanpa penjelasan terlihat seperti aplikasi yang rusak,
          padahal itu justru bagian yang paling penting dari fitur ini. */}
      <p className="mt-1.5 text-[11px] text-muted">
        Semua perangkat lain akan dikeluarkan, termasuk yang sedang terbuka.
      </p>

      {note && <p className="mt-2 text-xs text-emerald-600 dark:text-emerald-400">{note}</p>}
      {error && <p className="mt-2 text-xs text-red-500">{error}</p>}
    </section>
  );
}

// ---------- perangkat ----------

function SessionSection() {
  const [sessions, setSessions] = useState<Session[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  const muat = () =>
    api
      .sessions()
      .then(setSessions)
      .catch(err => setError(err instanceof ApiError ? err.message : 'Gagal memuat perangkat'));

  useEffect(() => {
    void muat();
  }, []);

  async function cabut(id: string) {
    setError(null);
    try {
      await api.revokeSession(id);
      setSessions(list => (list ?? []).filter(s => s.id !== id));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Gagal mencabut perangkat');
    }
  }

  return (
    <section className="px-4 py-4">
      <p className="mb-2 text-xs font-medium text-muted">Perangkat yang sedang masuk</p>

      {sessions === null && <p className="text-xs text-muted">Memuat…</p>}

      <ul className="flex flex-col gap-1">
        {(sessions ?? []).map(s => (
          <li key={s.id} className="group flex items-start gap-2 rounded-lg px-2 py-1.5 hover:bg-canvas">
            <span className="min-w-0 flex-1">
              <span className="block truncate text-xs">
                {ringkasAgen(s.userAgent)}
                {s.current && <span className="ml-1 text-muted">(perangkat ini)</span>}
              </span>
              <span className="block text-[11px] text-muted">{terakhirDipakai(s)}</span>
            </span>

            {/* Sesi yang sedang dipakai tidak ditawari tombol cabut: menekannya
                cuma mengeluarkan orangnya dari layar yang sedang dia buka, dan
                itu sudah ada namanya sendiri — "Keluar". */}
            {!s.current && (
              <button
                onClick={() => void cabut(s.id)}
                className="shrink-0 rounded px-1.5 py-0.5 text-[11px] text-red-500 opacity-0 transition hover:underline group-hover:opacity-100 [@media(hover:none)]:opacity-100"
              >
                cabut
              </button>
            )}
          </li>
        ))}
      </ul>

      {error && <p className="mt-2 text-xs text-red-500">{error}</p>}
    </section>
  );
}

/**
 * Menyingkat User-Agent jadi sesuatu yang bisa dikenali orang.
 *
 * Bukan penguraian yang benar — User-Agent memang tidak bisa diurai dengan
 * benar — melainkan tebakan yang cukup untuk menjawab satu pertanyaan: mana di
 * antara baris-baris ini yang bukan perangkat saya. Urutannya penting: Edge
 * menyebut dirinya Chrome, dan Chrome menyebut dirinya Safari.
 */
function ringkasAgen(ua: string): string {
  if (!ua) return 'Perangkat tidak dikenal';

  const browser =
    /Edg\//.test(ua) ? 'Edge'
    : /OPR\/|Opera/.test(ua) ? 'Opera'
    : /Firefox\//.test(ua) ? 'Firefox'
    : /Chrome\//.test(ua) ? 'Chrome'
    : /Safari\//.test(ua) ? 'Safari'
    : 'Peramban lain';

  const sistem =
    /Android/.test(ua) ? 'Android'
    : /iPhone|iPad|iPod/.test(ua) ? 'iOS'
    : /Mac OS X/.test(ua) ? 'macOS'
    : /Windows/.test(ua) ? 'Windows'
    : /Linux/.test(ua) ? 'Linux'
    : '';

  return sistem ? `${browser} di ${sistem}` : browser;
}

function terakhirDipakai(s: Session): string {
  const at = new Date(s.lastSeenAt ?? s.createdAt);
  if (Number.isNaN(at.getTime())) return '';

  const menit = Math.round((Date.now() - at.getTime()) / 60_000);
  if (menit < 5) return 'aktif barusan';
  if (menit < 60) return `aktif ${menit} menit lalu`;
  if (menit < 60 * 24) return `aktif ${Math.round(menit / 60)} jam lalu`;
  return 'terakhir aktif ' + at.toLocaleDateString();
}
