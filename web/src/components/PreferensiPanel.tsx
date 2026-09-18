import { useState } from 'react';
import { usePush } from '../push';
import Icon, { type IconName } from './Icon';
import { pilihTema, temaTersimpan, type Tema } from '../tema';
import PanelShell, { PanelSection } from './ui/PanelShell';
import { Kabar } from './ui/Field';

/**
 * Panel preferensi: setelan yang melekat pada PERANGKAT INI, bukan pada akun.
 *
 * Tema dan notifikasi tinggal bersama karena keduanya punya sifat yang sama dan
 * sering disalahpahami dengan cara yang sama: keduanya tidak ikut pindah ke
 * komputer lain. Selama tema duduk di dalam panel berjudul "Akun kamu", satu
 * kalimat kecil di bawahnya harus menanggung seluruh beban menjelaskan itu.
 * Sekarang judul panelnya yang menjelaskan, dan kalimat itu hanya menegaskan.
 */
export default function PreferensiPanel({ onClose }: { onClose: () => void }) {
  return (
    <PanelShell title="Preferensi" closeLabel="Tutup panel preferensi" onClose={onClose}>
      <Tampilan />
      <Notifikasi />
    </PanelShell>
  );
}

// ---------- tampilan ----------

/**
 * Tiga pilihan, bukan saklar dua posisi.
 *
 * "Ikut sistem" berdiri sebagai pilihan tersendiri karena dia satu-satunya yang
 * ikut berubah saat perangkatnya masuk mode malam sore hari. Saklar dua posisi
 * memaksa orang membekukan salah satu warna selamanya — dan yang paling sering
 * dipilih orang justru yang ketiga ini.
 */
const TAMPILAN: { value: Tema; label: string; ikon: IconName }[] = [
  { value: 'sistem', label: 'Ikut sistem', ikon: 'perangkat' },
  { value: 'terang', label: 'Terang', ikon: 'matahari' },
  { value: 'gelap', label: 'Gelap', ikon: 'bulan' },
];

function Tampilan() {
  // Nilai awalnya dibaca dari penyimpanan, bukan dari atribut yang terpasang di
  // halaman: yang terpasang sudah diterjemahkan jadi terang atau gelap, dan
  // "ikut sistem" tidak bisa dibaca kembali dari sana.
  const [tema, setTema] = useState<Tema>(temaTersimpan);

  return (
    <PanelSection title="Tampilan">
      <div className="flex gap-1.5">
        {TAMPILAN.map(t => (
          <button
            key={t.value}
            onClick={() => {
              pilihTema(t.value);
              setTema(t.value);
            }}
            aria-pressed={tema === t.value}
            className={`flex flex-1 flex-col items-center gap-1.5 rounded-xl border px-2 py-2.5 text-[12px] font-medium transition ${
              tema === t.value
                ? 'border-accent bg-accent-soft text-accent-text'
                : 'border-line-strong hover:border-accent'
            }`}
          >
            <Icon name={t.ikon} size={18} />
            {t.label}
          </button>
        ))}
      </div>

      <p className="mt-2 text-[12px] leading-relaxed text-muted">
        {tema === 'sistem'
          ? 'Ikut pengaturan terang atau gelap di perangkat ini.'
          : 'Berlaku di perangkat ini saja.'}
      </p>
    </PanelSection>
  );
}

// ---------- notifikasi ----------

function Notifikasi() {
  const push = usePush();

  // Ketiga sebab "tidak bisa" dikatakan apa adanya, dan masing-masing dengan
  // kalimatnya sendiri. Satu kalimat untuk ketiganya memaksa orang menebak yang
  // mana yang berlaku padanya — dan yang bisa dia perbaiki sendiri cuma satu.
  const halangan = !push.supported
    ? 'Browser ini tidak mendukung notifikasi.'
    : !push.available
      ? 'Notifikasi tidak aktif di server ini.'
      : push.blocked
        ? 'Izin notifikasi diblokir di pengaturan browser. Buka pengaturan situs di browser ini untuk mengizinkannya lagi.'
        : null;

  return (
    <PanelSection title="Notifikasi" last>
      {halangan ? (
        <p className="text-[13px] leading-relaxed text-muted">{halangan}</p>
      ) : (
        <>
          <button
            onClick={push.toggle}
            disabled={push.busy}
            aria-pressed={push.enabled}
            className={`flex min-h-11 w-full items-center gap-2.5 rounded-xl border px-3 text-left text-[13px] font-medium transition ${
              push.enabled
                ? 'border-accent bg-accent-soft text-accent-text'
                : 'border-line-strong hover:border-accent'
            }`}
          >
            <Icon name={push.enabled ? 'lonceng' : 'lonceng-mati'} size={18} />
            <span className="flex-1">
              {push.enabled ? 'Notifikasi menyala' : 'Notifikasi mati'}
            </span>
            <span className="text-[12px] font-semibold">
              {push.busy ? '…' : push.enabled ? 'Matikan' : 'Nyalakan'}
            </span>
          </button>

          <p className="mt-2 text-[12px] leading-relaxed text-muted">
            Dikirim hanya saat aplikasi ini tertutup, dan hanya ke perangkat ini.
          </p>
        </>
      )}

      <Kabar tone="bahaya" text={push.error} />
    </PanelSection>
  );
}
