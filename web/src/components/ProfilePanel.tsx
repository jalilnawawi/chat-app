import { useEffect, useRef, useState } from 'react';
import { ApiError } from '../api';
import { useStore } from '../store';
import type { StatusKind } from '../types';
import Avatar, { untilLabel } from './Avatar';
import PanelShell, { PanelSection } from './ui/PanelShell';
import PillButton from './ui/PillButton';
import Field, { KOLOM, Kabar } from './ui/Field';

/**
 * Panel profil: bagaimana orang lain melihat kamu.
 *
 * Foto, nama, dan status tinggal bersama karena ketiganya menjawab satu
 * pertanyaan yang sama. Yang dulu duduk di antara mereka — tema terang/gelap —
 * dan yang dulu menyusul di bawahnya — email dan password — sudah pindah ke
 * panelnya sendiri: ketiganya urusan yang berbeda, dengan frekuensi yang
 * berbeda, dan satu-satunya alasan mereka pernah bersebelahan adalah karena
 * sama-sama "setelan".
 */
export default function ProfilePanel({ onClose }: { onClose: () => void }) {
  const me = useStore(s => s.me);
  if (!me) return null;

  return (
    <PanelShell title="Profil" closeLabel="Tutup panel profil" onClose={onClose}>
      <FotoDanNama />
      <Status />
    </PanelShell>
  );
}

// ---------- foto & nama ----------

function FotoDanNama() {
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
  const [note, setNote] = useState<string | null>(null);
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
    setNote(null);
    setProgress(0);
    try {
      await uploadAvatar(file, setProgress);
      setNote('Foto profil diganti.');
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Unggahan gagal');
    } finally {
      setProgress(null);
    }
  }

  const saveName = () => {
    const next = name.trim();
    if (!next || next === me.displayName) return;
    setError(null);
    void updateProfile(next)
      .then(() => setNote('Nama tampilan disimpan.'))
      .catch(err => setError(err instanceof ApiError ? err.message : 'Gagal menyimpan nama'));
  };

  return (
    <PanelSection title="Foto dan nama">
      <div className="flex items-center gap-3">
        <Avatar name={me.displayName} url={me.avatarUrl} size={64} />
        <div className="min-w-0 flex-1">
          <p className="truncate text-[15px] font-bold">@{me.username}</p>

          {/* Tombolnya hanya ada kalau server ini memang bisa menerimanya.
              Tombol yang selalu tampil tapi dijawab "foto profil tidak aktif di
              server ini" mengajari orang bahwa pesan kesalahan di aplikasi ini
              boleh diabaikan — dan pelajaran itu terbawa ke pesan kesalahan
              yang benar-benar penting. */}
          {avatars ? (
            <div className="mt-1.5 flex flex-wrap gap-1.5">
              <PillButton
                tone="outline"
                size="sm"
                onClick={() => fileInput.current?.click()}
                disabled={progress !== null}
                // Angka persen dibacakan selagi berjalan: unggahan yang hanya
                // terlihat sebagai tombol yang membeku tidak bisa dibedakan
                // dari tombol yang macet.
                aria-busy={progress !== null}
              >
                {progress === null ? 'Ganti foto' : `${Math.round(progress * 100)}%`}
              </PillButton>
              {me.avatarUrl && (
                <PillButton
                  tone="ghost"
                  size="sm"
                  onClick={() => void removeAvatar()}
                  disabled={progress !== null}
                >
                  Hapus
                </PillButton>
              )}
            </div>
          ) : (
            <p className="mt-1.5 text-[12px] leading-relaxed text-muted">
              Foto profil tidak aktif di server ini.
            </p>
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

      <div className="mt-4">
        <Field
          id="nama-tampilan"
          label="Nama tampilan"
          value={name}
          maxLength={60}
          autoComplete="nickname"
          // Username sengaja tidak bisa diganti, dan itu dikatakan di muka
          // alih-alih ditunggu sampai orang mencari tombolnya.
          hint="Username tidak bisa diganti."
          onChange={e => setName(e.target.value)}
          onBlur={saveName}
          onKeyDown={e => {
            if (e.key === 'Enter') {
              e.preventDefault();
              (e.target as HTMLInputElement).blur();
            }
            // Dihentikan di sini: Escape di kolom ini membatalkan ketikan, dan
            // kerangka panel menutup diri hanya kalau tidak ada yang memakainya
            // lebih dulu.
            if (e.key === 'Escape') {
              e.preventDefault();
              setName(me.displayName);
            }
          }}
        />
      </div>

      <Kabar tone="ok" text={note} />
      <Kabar tone="bahaya" text={error} />
    </PanelSection>
  );
}

// ---------- status ----------

export const STATUS_PILIHAN: { value: StatusKind; label: string; dot: string }[] = [
  { value: 'available', label: 'Tersedia', dot: 'bg-ok' },
  { value: 'busy', label: 'Sibuk', dot: 'bg-danger' },
  { value: 'away', label: 'Tidak di tempat', dot: 'bg-call' },
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

function Status() {
  const me = useStore(s => s.me)!;
  const setStatus = useStore(s => s.setStatus);

  const [text, setText] = useState(me.statusText ?? '');
  const [error, setError] = useState<string | null>(null);

  useEffect(() => setText(me.statusText ?? ''), [me.statusText]);

  const simpan = (status: StatusKind, isi: string, sampai: string | null) => {
    setError(null);
    void setStatus(status, isi, sampai).catch(err =>
      setError(err instanceof ApiError ? err.message : 'Gagal memasang status'),
    );
  };

  const berlaku = untilLabel(me.statusExpiresAt);

  return (
    <PanelSection title="Status" last>
      <div className="flex gap-1.5">
        {STATUS_PILIHAN.map(p => (
          <button
            key={p.value}
            // Mengganti status TIDAK menyentuh batas waktunya. Dulu tombol ini
            // ikut mengirim durasi yang sedang tersorot di layar, dan karena
            // sorotan itu selalu mulai dari "Tanpa batas" setiap panel dibuka,
            // orang yang menekan "Sibuk" untuk kedua kalinya menghapus sendiri
            // batas waktu yang dia pasang pagi tadi.
            onClick={() => simpan(p.value, text.trim(), me.statusExpiresAt ?? null)}
            aria-pressed={me.status === p.value}
            className={`flex min-h-10 flex-1 items-center justify-center gap-1.5 rounded-xl border px-2 text-[13px] font-medium transition ${
              me.status === p.value
                ? 'border-accent bg-accent-soft text-accent-text'
                : 'border-line-strong hover:border-accent'
            }`}
          >
            <span className={`inline-block size-2.5 rounded-full ${p.dot}`} />
            {p.label}
          </button>
        ))}
      </div>

      <label htmlFor="kabar-status" className="mt-3 block text-[13px] font-semibold text-muted">
        Kabar status
      </label>
      <input
        id="kabar-status"
        value={text}
        maxLength={120}
        placeholder="Sedang apa? (opsional)"
        onChange={e => setText(e.target.value)}
        onBlur={() => {
          if (text.trim() !== (me.statusText ?? '')) {
            simpan(me.status, text.trim(), me.statusExpiresAt ?? null);
          }
        }}
        onKeyDown={e => {
          if (e.key === 'Enter') (e.target as HTMLInputElement).blur();
        }}
        className={`mt-1.5 ${KOLOM}`}
      />

      {/* Baris durasi adalah TINDAKAN, bukan pilihan yang tersimpan. Yang
          tersimpan cuma satu instan, dan instan itu hampir tidak pernah sama
          persis dengan salah satu tombol ini satu menit kemudian. Jadi keadaan
          yang berlaku dikatakan dengan kalimat, dan tombolnya mengubahnya. */}
      <p className="mt-3 text-[13px] text-muted">
        Berlaku <span className="font-semibold text-ink">{berlaku || 'tanpa batas'}</span>.
      </p>

      <div className="mt-1.5 flex flex-wrap gap-1.5">
        {DURASI.map(d => (
          <button
            key={d.label}
            onClick={() => simpan(me.status, text.trim(), d.until())}
            className="min-h-10 rounded-full border border-line-strong px-3 text-[12px] font-medium transition hover:border-accent hover:text-accent-text"
          >
            {d.label}
          </button>
        ))}
      </div>

      {/* Dikatakan di muka, bukan ditemukan sendiri: ini satu-satunya hal yang
          membuat status bukan sekadar hiasan. */}
      {me.status === 'busy' && (
        <p className="mt-2 text-[12px] leading-relaxed text-muted">
          Notifikasi diredam selama sibuk — kecuali kalau ada yang menyebut namamu.
        </p>
      )}

      <Kabar tone="bahaya" text={error} />
    </PanelSection>
  );
}
