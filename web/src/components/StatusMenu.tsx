import { useEffect, useRef, useState } from 'react';
import { useStore } from '../store';
import { statusLabel } from './Avatar';
import Icon from './Icon';
import { STATUS_PILIHAN } from './ProfilePanel';

/**
 * Pemilih status cepat di kepala sidebar.
 *
 * Status adalah satu-satunya isi panel profil yang diganti berkali-kali sehari,
 * dan baris yang menampilkannya sudah ada di sini sejak awal — cuma tidak bisa
 * ditekan. Membuatnya bisa ditekan lebih murah daripada memindahkan seluruh
 * panel lebih dekat.
 *
 * Yang ada di sini HANYA tiga status. Kabar dan durasi tidak ikut: keduanya
 * butuh mengetik dan memilih, dan papan sekecil ini akan jadi panel kedua yang
 * separuh lengkap. Jalan ke sana ada di baris terakhir.
 */
export default function StatusMenu({ onOpenProfile }: { onOpenProfile: () => void }) {
  const me = useStore(s => s.me);
  const connected = useStore(s => s.connected);
  const setStatus = useStore(s => s.setStatus);

  const [open, setOpen] = useState(false);
  const box = useRef<HTMLDivElement>(null);
  const trigger = useRef<HTMLButtonElement>(null);

  // Klik di luar dan Escape menutup papan, dan fokus kembali ke tombolnya.
  // Papan yang menempel di layar setelah orangnya berpindah adalah papan yang
  // menutupi daftar percakapan tanpa diminta.
  useEffect(() => {
    if (!open) return;
    const onDown = (e: PointerEvent) => {
      if (!box.current?.contains(e.target as Node)) setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== 'Escape') return;
      // Dihentikan di sini supaya panel di belakangnya tidak ikut tertutup oleh
      // satu tekanan Escape yang sama.
      e.stopPropagation();
      setOpen(false);
      trigger.current?.focus();
    };
    document.addEventListener('pointerdown', onDown);
    document.addEventListener('keydown', onKey, true);
    return () => {
      document.removeEventListener('pointerdown', onDown);
      document.removeEventListener('keydown', onKey, true);
    };
  }, [open]);

  if (!me) return null;

  const pasang = (value: (typeof STATUS_PILIHAN)[number]['value']) => {
    setOpen(false);
    trigger.current?.focus();
    // Batas waktu yang sudah dipasang TIDAK ikut terhapus di sini, sama seperti
    // di panel profil: mengganti status bukan mencabut jadwalnya.
    void setStatus(value, me.statusText ?? '', me.statusExpiresAt ?? null).catch(() => {});
  };

  const sekarang = STATUS_PILIHAN.find(p => p.value === me.status);
  // Status yang dipasang sendiri menggantikan keterangan koneksi: yang pertama
  // dinyatakan orangnya dengan sengaja, yang kedua cuma kabar tentang jaringan.
  const teks =
    statusLabel(me.status, me.statusText, me.statusExpiresAt) ||
    (connected ? 'Tersambung' : 'Menyambungkan ulang…');

  return (
    <div ref={box} className="relative">
      <button
        ref={trigger}
        onClick={() => setOpen(v => !v)}
        aria-expanded={open}
        aria-haspopup="menu"
        aria-label={`Status kamu: ${teks}. Ganti status`}
        className={`flex min-h-10 w-full items-center gap-2 rounded-xl px-2 text-left transition ${
          open ? 'bg-canvas' : 'hover:bg-canvas'
        }`}
      >
        <span className={`size-2.5 shrink-0 rounded-full ${sekarang?.dot ?? 'bg-line-strong'}`} />
        <span className="min-w-0 flex-1 truncate text-[13px] text-muted">{teks}</span>
        <Icon name="bawah" size={14} className="text-muted" />
      </button>

      {open && (
        <div
          role="menu"
          aria-label="Ganti status"
          className="absolute inset-x-0 top-full z-20 mt-1 overflow-hidden rounded-2xl border border-line bg-surface shadow-pop"
        >
          {STATUS_PILIHAN.map(p => (
            <button
              key={p.value}
              role="menuitemradio"
              aria-checked={me.status === p.value}
              onClick={() => pasang(p.value)}
              className={`flex min-h-11 w-full items-center gap-2.5 px-3 text-left text-sm transition hover:bg-canvas ${
                me.status === p.value ? 'font-bold text-accent-text' : ''
              }`}
            >
              <span className={`size-2.5 shrink-0 rounded-full ${p.dot}`} />
              {p.label}
            </button>
          ))}

          <button
            role="menuitem"
            onClick={() => {
              setOpen(false);
              onOpenProfile();
            }}
            className="flex min-h-11 w-full items-center justify-between gap-2 border-t border-line px-3 text-left text-[13px] font-medium text-muted transition hover:bg-canvas hover:text-ink"
          >
            Atur kabar dan durasi
            <span aria-hidden="true">›</span>
          </button>
        </div>
      )}
    </div>
  );
}
