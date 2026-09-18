import { useEffect, useId, useRef, type ReactNode } from 'react';
import { useMediaQuery } from '../../useMediaQuery';
import IconButton from './IconButton';

/** Di bawah lebar ini panel MENUTUPI layar, bukan berbagi dengannya. */
const OVERLAY = '(max-width: 1023px)';

/**
 * Kerangka panel kanan — profil, akun, preferensi, kelola grup, pencarian.
 *
 * Sebelumnya tiap panel menggambar kerangkanya sendiri, dan itu bukan sekadar
 * pengulangan: panel yang MENUTUPI seluruh layar adalah dialog, dan tidak satu
 * pun dari mereka mengatakannya. Fokus tertinggal di belakang panel, Escape
 * tidak menutup, dan Tab berjalan keluar ke percakapan yang sedang tertutup.
 * Diperbaiki di satu tempat, semua panel ikut sembuh.
 *
 * Yang menentukan dia dialog atau bukan adalah LEBAR, bukan jenis panelnya: di
 * layar lebar dia kolom biasa di sebelah percakapan, dan mengurung fokus di
 * dalam kolom yang tidak menutupi apa pun justru menjebak orangnya.
 *
 * Batasnya lg (1024px), bukan md: di 768px, sidebar 320 dan panel 320
 * menyisakan 128px untuk gelembung pesan — lebar yang sudah pernah membuat
 * aplikasi ini tidak terpakai di ponsel, dan tidak jadi lebih baik hanya karena
 * layarnya tablet.
 */
export default function PanelShell({
  title,
  onClose,
  closeLabel,
  width = 'w-80',
  toolbar,
  footer,
  children,
}: {
  title: string;
  onClose: () => void;
  /** Dibaca pembaca layar sebagai nama tombol tutup; judulnya sendiri terlalu pendek. */
  closeLabel: string;
  /** Lebar kolom di layar lebar. Pencarian butuh lebih dari setelan. */
  width?: 'w-80' | 'w-96';
  /** Bagian yang TIDAK ikut tergulir — kotak pencarian, penyaring. */
  toolbar?: ReactNode;
  footer?: ReactNode;
  children: ReactNode;
}) {
  const overlay = useMediaQuery(OVERLAY);
  const box = useRef<HTMLElement>(null);
  const headingId = useId();

  // Fokus masuk saat panel terbuka dan KEMBALI ke tempat asalnya saat ditutup.
  // Tanpa yang kedua, orang yang menutup panel dengan Escape kehilangan
  // jejaknya: fokus melompat ke awal halaman, dan Tab berikutnya berjalan dari
  // sana, bukan dari tombol yang baru saja dia tekan.
  useEffect(() => {
    const asal = document.activeElement as HTMLElement | null;
    // Panel yang sudah menaruh fokusnya sendiri tidak diganggu: kotak
    // pencarian yang langsung siap diketik adalah seluruh gunanya.
    if (!box.current?.contains(document.activeElement)) box.current?.focus();
    return () => asal?.focus();
  }, []);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && !e.defaultPrevented) {
        onClose();
        return;
      }

      // Fokus dikurung HANYA selagi panel menutupi layar. Di kolom lebar,
      // mengurungnya berarti memenjarakan orang di dalam setelan yang cuma
      // ingin dia lihat sekilas.
      if (e.key !== 'Tab' || !overlay || !box.current) return;

      const bisa = box.current.querySelectorAll<HTMLElement>(
        'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])',
      );
      const awal = bisa[0];
      const akhir = bisa[bisa.length - 1];
      if (!awal || !akhir) return;

      if (e.shiftKey && document.activeElement === awal) {
        e.preventDefault();
        akhir.focus();
      } else if (!e.shiftKey && document.activeElement === akhir) {
        e.preventDefault();
        awal.focus();
      }
    };

    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, [onClose, overlay]);

  return (
    <aside
      ref={box}
      tabIndex={-1}
      role={overlay ? 'dialog' : undefined}
      aria-modal={overlay ? true : undefined}
      aria-labelledby={headingId}
      // Kelasnya ditulis utuh, bukan disusun dari potongan: pemindai Tailwind
      // membaca berkas ini sebagai teks, dan kelas yang baru terbentuk saat
      // program berjalan tidak pernah ikut tercetak ke CSS.
      className={`fixed inset-0 z-30 flex w-full flex-col bg-surface outline-none lg:static lg:z-auto lg:shrink-0 lg:border-l lg:border-line ${
        width === 'w-96' ? 'lg:w-96' : 'lg:w-80'
      }`}
    >
      <header className="flex items-center justify-between gap-2 border-b border-line px-4 py-2.5">
        <h2 id={headingId} className="truncate text-[15px] font-bold">
          {title}
        </h2>
        <IconButton icon="tutup" label={closeLabel} onClick={onClose} />
      </header>

      {toolbar}

      <div className="flex-1 overflow-y-auto overscroll-contain">{children}</div>

      {footer}
    </aside>
  );
}

/**
 * Satu bagian di dalam panel, dengan judulnya sendiri.
 *
 * Judulnya heading sungguhan, bukan paragraf tebal: pembaca layar yang
 * melompat antar-heading sebelumnya menemukan satu benda di seluruh panel
 * setelan, dan harus menyusuri dua puluh kontrol untuk tahu ada apa saja di
 * dalamnya.
 */
export function PanelSection({
  title,
  badge,
  last = false,
  children,
}: {
  title: string;
  /** Lencana kecil di samping judul — keadaan yang menempel pada bagian ini. */
  badge?: ReactNode;
  /** Bagian terakhir tidak diberi garis bawah: di sana tidak ada apa-apa lagi. */
  last?: boolean;
  children: ReactNode;
}) {
  return (
    <section className={`px-4 py-4 ${last ? '' : 'border-b border-line'}`}>
      <h3 className="mb-2 flex items-center gap-2 text-[13px] font-semibold text-muted">
        {title}
        {badge}
      </h3>
      {children}
    </section>
  );
}
