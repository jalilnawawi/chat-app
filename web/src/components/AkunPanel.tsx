import { useEffect, useState } from 'react';
import { ApiError, api } from '../api';
import { unsubscribeThisDevice } from '../push';
import { useStore } from '../store';
import type { Session } from '../types';
import Icon from './Icon';
import PanelShell, { PanelSection } from './ui/PanelShell';
import PillButton from './ui/PillButton';
import Field, { Kabar } from './ui/Field';

/**
 * Panel akun: siapa yang bisa masuk ke sini, dan dari mana.
 *
 * Email, password, dan daftar perangkat adalah satu urusan — kunci rumah — dan
 * mereka dikumpulkan di panel sendiri justru karena itu. Sebelumnya mereka
 * berbagi satu gulir dengan foto profil dan pemilih tema, dengan bobot visual
 * yang persis sama, sehingga "ganti password" dan "mode gelap" terbaca
 * sama-sama ringan.
 *
 * "Keluar" berakhir di sini, bukan sebagai ikon di pojok sidebar: dia tindakan
 * dengan akibat terbesar di seluruh aplikasi, dan tempatnya di antara tindakan
 * berakibat besar lainnya.
 */
export default function AkunPanel({ onClose }: { onClose: () => void }) {
  const me = useStore(s => s.me);
  if (!me) return null;

  return (
    <PanelShell title="Akun" closeLabel="Tutup panel akun" onClose={onClose} footer={<Keluar />}>
      <Email />
      <Password />
      <Perangkat />
    </PanelShell>
  );
}

// ---------- email ----------

function Email() {
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
      <PanelSection title="Email">
        {me.email ? (
          <p className="truncate text-[15px]">{me.email}</p>
        ) : (
          <p className="text-[15px] text-muted">Belum diisi.</p>
        )}
        <p className="mt-1.5 text-[12px] leading-relaxed text-muted">
          Verifikasi email dan pemulihan password tidak aktif di server ini.
        </p>
      </PanelSection>
    );
  }

  return (
    <PanelSection
      title="Email"
      badge={
        me.email &&
        (me.emailVerified ? (
          <span className="rounded-full bg-ok-soft px-2 py-0.5 text-[11.5px] font-semibold text-ok">
            terverifikasi
          </span>
        ) : (
          // Bahaya, bukan mangga. Mangga sudah punya satu arti di aplikasi ini
          // — sebutan dan panggilan — dan isyarat yang dipakai untuk dua hal
          // berhenti jadi isyarat. Lagipula keadaan ini memang bahaya: selama
          // dia berlaku, akun ini tidak punya jalan pulang.
          <span className="rounded-full bg-danger-soft px-2 py-0.5 text-[11.5px] font-semibold text-danger">
            belum terverifikasi
          </span>
        ))
      }
    >
      <form
        onSubmit={e => {
          e.preventDefault();
          void simpan();
        }}
      >
        <Field
          label="Alamat email"
          type="email"
          autoComplete="email"
          value={email}
          disabled={busy}
          placeholder="kamu@contoh.com"
          onChange={e => setEmail(e.target.value)}
        />

        {/* Kolom password baru muncul saat alamatnya memang berubah. Memintanya
            sepanjang waktu membuat orang mengetik password untuk membaca layar. */}
        {berubah && (
          <Field
            label="Password kamu saat ini"
            type="password"
            autoComplete="current-password"
            value={password}
            disabled={busy}
            onChange={e => setPassword(e.target.value)}
          />
        )}

        <div className="mt-2 flex flex-wrap gap-2">
          {berubah && (
            <PillButton type="submit" size="sm" disabled={busy || !password}>
              Simpan email
            </PillButton>
          )}
          {me.email && !me.emailVerified && !berubah && (
            <PillButton tone="outline" size="sm" disabled={busy} onClick={() => void kirimUlang()}>
              Kirim ulang tautan
            </PillButton>
          )}
        </div>
      </form>

      {/* Alasannya dikatakan, bukan cuma lencananya ditampilkan: tanpa kalimat
          ini, "belum terverifikasi" terlihat seperti kerapian yang bisa
          diabaikan — sampai hari orangnya lupa password. */}
      {me.email && !me.emailVerified && (
        <p className="mt-2 text-[12px] leading-relaxed text-muted">
          Sebelum terverifikasi, alamat ini tidak bisa dipakai memulihkan akun.
        </p>
      )}

      <Kabar tone="ok" text={note} />
      <Kabar tone="bahaya" text={error} />
    </PanelSection>
  );
}

// ---------- password ----------

function Password() {
  const loadSessions = useStore(s => s.loadSessions);

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
      // Daftar di bawah dimuat ulang, bukan dibiarkan. Kalimat di atas baru saja
      // mengabarkan perangkat lain sudah keluar; daftar yang masih menampilkan
      // mereka sebagai aktif membantah kabar itu di layar yang sama.
      void loadSessions().catch(() => {});
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Gagal mengganti password');
    } finally {
      setBusy(false);
    }
  }

  return (
    <PanelSection title="Ganti password">
      {/* Akibatnya dikatakan SEBELUM tombolnya ditekan, dan sebelum kolomnya
          diisi. Perangkat lain yang tiba-tiba keluar tanpa penjelasan terlihat
          seperti aplikasi yang rusak, padahal itu justru bagian yang paling
          penting dari fitur ini. */}
      <p className="mb-2 text-[12px] leading-relaxed text-muted">
        Semua perangkat lain akan dikeluarkan, termasuk yang sedang terbuka.
      </p>

      <form
        onSubmit={e => {
          e.preventDefault();
          void simpan();
        }}
      >
        <Field
          label="Password saat ini"
          type="password"
          autoComplete="current-password"
          value={current}
          disabled={busy}
          onChange={e => setCurrent(e.target.value)}
        />
        <Field
          label="Password baru"
          type="password"
          autoComplete="new-password"
          value={next}
          disabled={busy}
          hint="Minimal 8 karakter."
          onChange={e => setNext(e.target.value)}
        />

        <PillButton
          type="submit"
          size="sm"
          className="mt-2"
          disabled={busy || !current || next.length < 8}
        >
          Ganti password
        </PillButton>
      </form>

      <Kabar tone="ok" text={note} />
      <Kabar tone="bahaya" text={error} />
    </PanelSection>
  );
}

// ---------- perangkat ----------

function Perangkat() {
  const sessions = useStore(s => s.sessions);
  const loadSessions = useStore(s => s.loadSessions);
  const revokeSession = useStore(s => s.revokeSession);
  const revokeOtherSessions = useStore(s => s.revokeOtherSessions);

  const [error, setError] = useState<string | null>(null);
  const [note, setNote] = useState<string | null>(null);
  const [memastikan, setMemastikan] = useState(false);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    void loadSessions().catch(err =>
      setError(err instanceof ApiError ? err.message : 'Gagal memuat perangkat'),
    );
  }, [loadSessions]);

  const lain = (sessions ?? []).filter(s => !s.current).length;

  async function cabut(id: string) {
    setError(null);
    setNote(null);
    try {
      await revokeSession(id);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Gagal mencabut perangkat');
    }
  }

  async function cabutSemua() {
    setBusy(true);
    setError(null);
    setNote(null);
    try {
      const jumlah = await revokeOtherSessions();
      setMemastikan(false);
      setNote(`${jumlah} perangkat dikeluarkan.`);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Gagal mengeluarkan perangkat lain');
    } finally {
      setBusy(false);
    }
  }

  return (
    <PanelSection title="Perangkat yang sedang masuk" last>
      {sessions === null && <p className="text-[13px] text-muted">Memuat…</p>}

      {sessions !== null && sessions.length === 0 && (
        <p className="text-[13px] leading-relaxed text-muted">
          Belum ada perangkat lain yang tercatat.
        </p>
      )}

      <ul className="flex flex-col gap-1">
        {(sessions ?? []).map(s => (
          <li
            key={s.id}
            className="group flex items-center gap-2 rounded-xl px-2 py-1.5 transition hover:bg-canvas"
          >
            <span className="min-w-0 flex-1">
              <span className="block truncate text-sm font-medium">
                {ringkasAgen(s.userAgent)}
                {s.current && <span className="ml-1 text-muted">(perangkat ini)</span>}
              </span>
              <span className="block text-[12px] text-muted">{terakhirDipakai(s)}</span>
            </span>

            {/* Sesi yang sedang dipakai tidak ditawari tombol cabut: menekannya
                cuma mengeluarkan orangnya dari layar yang sedang dia buka, dan
                itu sudah ada namanya sendiri — "Keluar". */}
            {!s.current && (
              <button
                onClick={() => void cabut(s.id)}
                aria-label={`Cabut ${ringkasAgen(s.userAgent)}`}
                // Terlihat saat disentuh kursor, saat difokus papan ketik, dan
                // selalu di layar sentuh. Sebelumnya cuma dua yang pertama
                // hilang: orang yang ber-Tab ke sini mendarat di tombol yang
                // sama sekali tidak terlihat — termasuk cincin fokusnya —
                // padahal tombol itu mengeluarkan sebuah perangkat.
                className="min-h-10 shrink-0 rounded-lg px-2.5 text-[12px] font-medium text-danger opacity-0 transition hover:bg-danger-soft focus-visible:opacity-100 group-hover:opacity-100 [@media(hover:none)]:opacity-100"
              >
                Cabut
              </button>
            )}
          </li>
        ))}
      </ul>

      {lain > 0 &&
        (memastikan ? (
          // Dua langkah, bukan "urungkan". Aturan di aplikasi ini memang lebih
          // suka membatalkan daripada bertanya, tapi sesi yang sudah dicabut
          // tidak bisa dikembalikan — tidak ada yang tersisa untuk diurungkan.
          <div className="mt-3 rounded-xl border border-danger bg-danger-soft px-3 py-2.5">
            <p className="text-[13px] leading-relaxed text-ink">
              Keluarkan {lain} perangkat lain? Masing-masing harus masuk lagi.
            </p>
            <div className="mt-2 flex gap-2">
              <PillButton
                tone="danger"
                size="sm"
                disabled={busy}
                onClick={() => void cabutSemua()}
              >
                Ya, keluarkan
              </PillButton>
              <PillButton tone="ghost" size="sm" disabled={busy} onClick={() => setMemastikan(false)}>
                Batal
              </PillButton>
            </div>
          </div>
        ) : (
          <PillButton
            tone="outline"
            size="sm"
            className="mt-3"
            onClick={() => setMemastikan(true)}
          >
            Keluarkan semua perangkat lain
          </PillButton>
        ))}

      <Kabar tone="ok" text={note} />
      <Kabar tone="bahaya" text={error} />
    </PanelSection>
  );
}

// ---------- keluar ----------

function Keluar() {
  const reset = useStore(s => s.reset);

  async function keluar() {
    // Langganan notifikasi dicabut SEBELUM sesinya dibuang — pencabutan itu
    // sendiri butuh sesi yang masih berlaku. Tanpa ini, langganan orang ini
    // tetap hidup di komputer yang dipakai bergantian, dan pratinjau pesannya
    // terus muncul di layar kunci orang berikutnya.
    await unsubscribeThisDevice();
    await api.logout().catch(() => {});
    reset();
  }

  return (
    <footer className="border-t border-line px-4 py-3">
      <button
        onClick={() => void keluar()}
        className="flex min-h-11 w-full items-center justify-center gap-2 rounded-xl border border-line-strong px-3 text-sm font-semibold text-danger transition hover:border-danger hover:bg-danger-soft"
      >
        <Icon name="keluar" size={18} />
        Keluar dari akun
      </button>
    </footer>
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
