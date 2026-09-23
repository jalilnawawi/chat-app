# Shared UI Components

Stack: React 19 + TypeScript, Vite 8, Tailwind CSS v4 (`@tailwindcss/vite`, CSS-first `@theme` — **no tailwind.config file**), zustand 5 for state. No component library (no shadcn/MUI/Radix) — every primitive is hand-written. Font: Plus Jakarta Sans Variable via `@fontsource-variable`.

Primitives live in `web/src/components/ui/`. Shared non-primitive atoms (`Icon`, `Avatar`, `Tanda`) live in `web/src/components/`.

Source language note: the codebase is written in Indonesian (identifiers, comments, UI copy).

## IconButton

- File: `web/src/components/ui/IconButton.tsx`

```tsx
import type { ButtonHTMLAttributes, Ref } from 'react';
import Icon, { type IconName } from '../Icon';

/**
 * Tombol ikon persegi-membulat — kepala percakapan, penutup kabar, pembatal
 * balasan. Satu bentuk, supaya ukuran sasaran dan warna hover tidak berbeda
 * sedikit-sedikit di setiap tempat yang menyalinnya.
 *
 * `size`: `md` 40px (44px di layar sentuh), `sm` 40px rapat untuk bilah kabar
 * yang tipis (marginnya ditarik supaya bilah tidak ikut meninggi).
 * Label wajib: tombol ini tidak punya teks yang terlihat.
 */
export default function IconButton({
  ref,
  icon,
  label,
  iconSize,
  size = 'md',
  pressed,
  tone = 'muted',
  className = '',
  type = 'button',
  ...rest
}: Omit<ButtonHTMLAttributes<HTMLButtonElement>, 'aria-label'> & {
  ref?: Ref<HTMLButtonElement>;
  icon: IconName;
  label: string;
  iconSize?: number;
  size?: 'md' | 'sm';
  /** Tombol berkeadaan (panel terbuka): diberi `aria-pressed` dan nada teal. */
  pressed?: boolean;
  tone?: 'muted' | 'danger';
}) {
  const base =
    size === 'md'
      ? 'size-10 [@media(pointer:coarse)]:size-11'
      : '-my-1.5 size-10';
  const color =
    pressed === true
      ? 'bg-accent-soft text-accent-text'
      : tone === 'danger'
        ? 'text-danger hover:bg-surface'
        : 'text-muted hover:bg-canvas hover:text-ink';
  return (
    <button
      ref={ref}
      type={type}
      aria-label={label}
      aria-pressed={pressed}
      title={rest.title ?? label}
      className={`grid shrink-0 place-items-center rounded-xl transition ${base} ${color} ${className}`}
      {...rest}
    >
      <Icon name={icon} size={iconSize ?? (size === 'md' ? 20 : 16)} />
    </button>
  );
}
```

## PillButton

- File: `web/src/components/ui/PillButton.tsx`

```tsx
import type { ButtonHTMLAttributes, ReactNode, Ref } from 'react';
import Icon, { type IconName } from '../Icon';

/**
 * Tombol berbentuk pil — satu tempat untuk bentuk yang sebelumnya disalin di
 * kolom tulis, bilah rekam, baris pesan gagal, kabar hapus, dan form sunting.
 *
 * Nada, bukan warna: `solid` adalah tindakan utama di kanvas, `outline` yang
 * tenang, `danger` untuk memperbaiki kegagalan, `ghost` untuk jalan keluar.
 * Varian `onTeal*` hidup di dalam gelembung sendiri, tempat teal sudah jadi
 * latarnya.
 *
 * Nonaktif tidak pernah dipudarkan: tombol utama yang nonaktif jadi bergaris
 * dan tenang, bukan bidang teal yang terbaca seperti tombol macet.
 */
export type PillTone =
  | 'solid'
  | 'outline'
  | 'quiet'
  | 'danger'
  | 'ghost'
  | 'onTealSolid'
  | 'onTealGhost';

const TONE: Record<PillTone, string> = {
  solid:
    'border-transparent bg-accent text-accent-ink hover:brightness-110 active:scale-95 disabled:border-line-strong disabled:bg-surface disabled:text-muted disabled:hover:brightness-100 disabled:active:scale-100',
  outline:
    'border-line-strong bg-surface text-accent-text hover:border-accent hover:bg-accent-soft active:scale-95',
  /** Bergaris dan redup — "Muat pesan lama", tindakan yang tidak mendesak. */
  quiet: 'border-line-strong bg-surface font-medium text-muted hover:border-accent hover:text-accent-text',
  danger: 'border-danger bg-surface text-danger hover:bg-danger-soft',
  ghost: 'border-transparent text-muted hover:bg-surface hover:text-ink',
  onTealSolid:
    'border-transparent bg-accent-ink text-accent hover:brightness-95 disabled:border-accent-ink disabled:bg-transparent disabled:text-accent-ink',
  onTealGhost: 'border-transparent text-accent-ink hover:bg-accent-deep',
};

const SIZE = {
  /** 44px — kolom tulis, bilah rekam, baris gagal. */
  md: 'h-11 px-4 text-[15px]',
  /** 40px — kabar di atas kolom tulis, form sunting. */
  sm: 'h-10 px-3.5 text-[14px]',
} as const;

export default function PillButton({
  ref,
  tone = 'solid',
  size = 'md',
  icon,
  iconSize,
  children,
  /** Label disembunyikan di layar sempit; ikonnya tetap. */
  compact = false,
  className = '',
  type = 'button',
  ...rest
}: ButtonHTMLAttributes<HTMLButtonElement> & {
  ref?: Ref<HTMLButtonElement>;
  tone?: PillTone;
  size?: keyof typeof SIZE;
  icon?: IconName;
  iconSize?: number;
  compact?: boolean;
  children?: ReactNode;
}) {
  return (
    <button
      ref={ref}
      type={type}
      className={`flex shrink-0 items-center justify-center gap-2 rounded-full border font-semibold transition ${SIZE[size]} ${
        compact ? 'px-3 sm:px-4' : ''
      } ${TONE[tone]} ${className}`}
      {...rest}
    >
      {icon && <Icon name={icon} size={iconSize ?? (size === 'md' ? 20 : 17)} />}
      {children !== undefined &&
        (compact ? <span className="hidden sm:inline">{children}</span> : children)}
    </button>
  );
}
```

## Field

- File: `web/src/components/ui/Field.tsx`

```tsx
import { useId, type InputHTMLAttributes, type Ref } from 'react';

/** Bentuk kolom isian di seluruh panel. Dipakai juga oleh kolom yang bukan `Field`. */
export const KOLOM =
  'w-full rounded-xl border border-line-strong bg-canvas px-3.5 py-2.5 text-[15px] outline-none transition focus:bg-surface disabled:border-line disabled:text-muted';

/**
 * Kolom isian berlabel.
 *
 * Label yang terlihat, bukan teks contoh. Teks contoh hilang pada ketukan
 * pertama: orang yang berhenti di tengah pengisian kehilangan satu-satunya
 * keterangan tentang kolom yang sedang dia isi, dan pembaca layar tidak pernah
 * mendapatkannya sama sekali. Untuk kolom password — tiga di antaranya
 * bersebelahan di panel akun — itu bedanya antara mengisi dan menebak.
 */
export default function Field({
  ref,
  label,
  hint,
  id,
  ...rest
}: InputHTMLAttributes<HTMLInputElement> & {
  ref?: Ref<HTMLInputElement>;
  label: string;
  /** Keterangan di bawah kolom — syarat, akibat, atau batas. */
  hint?: string;
}) {
  const auto = useId();
  const kolomId = id ?? auto;
  const hintId = hint ? `${kolomId}-ket` : undefined;

  return (
    <div className="mt-2 first:mt-0">
      <label htmlFor={kolomId} className="block text-[13px] font-semibold text-muted">
        {label}
      </label>
      <input
        ref={ref}
        id={kolomId}
        aria-describedby={hintId}
        className={`mt-1.5 ${KOLOM}`}
        {...rest}
      />
      {hint && (
        <p id={hintId} className="mt-1.5 text-[12px] leading-relaxed text-muted">
          {hint}
        </p>
      )}
    </div>
  );
}

/**
 * Kabar hasil — berhasil atau gagal.
 *
 * Wadahnya SELALU terpasang, juga saat belum ada kabarnya. Wadah yang baru
 * muncul bersama teksnya tidak dibacakan pembaca layar: yang diumumkan adalah
 * perubahan DI DALAM daerah hidup, dan daerah yang lahir bersama isinya tidak
 * pernah berubah.
 */
export function Kabar({ tone, text }: { tone: 'ok' | 'bahaya'; text: string | null }) {
  return (
    <p
      // alert menyela, status menunggu giliran. "Password saat ini salah"
      // pantas menyela; "Tautan verifikasi dikirim" tidak.
      role={tone === 'bahaya' ? 'alert' : 'status'}
      className={`text-[13px] ${tone === 'bahaya' ? 'text-danger' : 'text-ok'} ${text ? 'mt-2' : ''}`}
    >
      {text}
    </p>
  );
}
```

## PanelShell

- File: `web/src/components/ui/PanelShell.tsx`

```tsx
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
```

## Icon

- File: `web/src/components/Icon.tsx`

```tsx
/**
 * Ikon garis, satu berkas.
 *
 * Sebelumnya tempat-tempat ini diisi emoji — 📎, ⚙, 🔔, ✕. Emoji digambar oleh
 * sistem operasi, bukan oleh aplikasi: yang sama persis di layar satu orang
 * tampil berwarna dan gemuk di layar orang lain, ukurannya tidak bisa diatur,
 * dan tidak satu pun bisa mengikuti warna teks di sekitarnya. Untuk tombol —
 * benda yang harus terbaca seragam — itu terlalu banyak yang diserahkan kepada
 * kebetulan.
 *
 * Emoji TETAP dipakai di reaksi dan di pratinjau lampiran, dan itu bukan
 * inkonsistensi: di sana emoji adalah isinya, bukan lambang tombolnya.
 */
export type IconName =
  | 'kembali'
  | 'tulis'
  | 'cari'
  | 'klip'
  | 'kirim'
  | 'anggota'
  | 'tutup'
  | 'lonceng'
  | 'lonceng-mati'
  | 'keluar'
  | 'kunci'
  | 'setelan'
  | 'balas'
  | 'reaksi'
  | 'terbaca'
  | 'kamera'
  | 'matahari'
  | 'bulan'
  | 'perangkat'
  | 'teruskan'
  | 'sematan'
  | 'bawah'
  | 'mikrofon'
  | 'emoji'
  | 'hapus'
  | 'putar'
  | 'jeda'
  | 'tambah'
  | 'peringatan'
  | 'ulang'
  | 'terkirim'
  | 'tulis-ulang'
  | 'buka-menu'
  | 'salin';

/** Jalur `d` tiap ikon. Semuanya digambar di kotak 24×24 yang sama. */
const JALUR: Record<IconName, string> = {
  kembali: 'M19 12H5m7-7-7 7 7 7',
  tulis: 'M4 20h4l10.5-10.5a2.12 2.12 0 0 0-3-3L5 17v3zM14.5 6.5l3 3',
  cari: 'M11 19a8 8 0 1 0 0-16 8 8 0 0 0 0 16zM21 21l-4.35-4.35',
  klip: 'M21.44 11.05l-8.49 8.49a6 6 0 0 1-8.49-8.49l8.49-8.49a4 4 0 0 1 5.66 5.66l-8.5 8.49a2 2 0 0 1-2.82-2.83l7.78-7.78',
  kirim: 'M4.5 12h6m-6.2-7.1 15.2 7.1-15.2 7.1 1.7-7.1-1.7-7.1z',
  anggota:
    'M16 20v-1.5a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4V20M9 10.5a3.5 3.5 0 1 0 0-7 3.5 3.5 0 0 0 0 7zM22 20v-1.5a4 4 0 0 0-3-3.87M16 3.63a4 4 0 0 1 0 7.75',
  tutup: 'M18 6 6 18M6 6l12 12',
  lonceng: 'M18 9a6 6 0 1 0-12 0c0 5-2.5 6-2.5 6h17S18 14 18 9M13.7 20a2 2 0 0 1-3.4 0',
  'lonceng-mati':
    'M13.7 20a2 2 0 0 1-3.4 0M18.6 14A5.5 5.5 0 0 0 18 9a6 6 0 0 0-9.3-5M5.9 6.1A6 6 0 0 0 6 9c0 5-2.5 6-2.5 6h13M2 2l20 20',
  keluar: 'M15 17l5-5-5-5M20 12H9M11 4H6a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h5',
  kunci: 'M9 20a5 5 0 1 0 0-10 5 5 0 0 0 0 10M12.6 11.4 20 4M16.5 7.5l2.5 2.5M19 5l2 2',
  setelan:
    'M3 7h4m4 0h10M3 12h10m4 0h4M3 17h3m4 0h11M11 7a2 2 0 1 0-4 0 2 2 0 0 0 4 0M17 12a2 2 0 1 0-4 0 2 2 0 0 0 4 0M10 17a2 2 0 1 0-4 0 2 2 0 0 0 4 0',
  balas: 'M9 14 4 9l5-5M4 9h9a7 7 0 0 1 7 7v4',
  reaksi:
    'M20.9 13a9 9 0 1 1-7.9-9.9M8.5 14.5s1.3 1.7 3.5 1.7 3.5-1.7 3.5-1.7M9 9.5h.01M15 9.5h.01M19 2v6M22 5h-6',
  terbaca: 'M1.5 12.5 5 16l7.5-9M11 16l1 1 8.5-10',
  kamera:
    'M21 18V8a2 2 0 0 0-2-2h-2.5l-1.3-2h-6.4L7.5 6H5a2 2 0 0 0-2 2v10a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2zM12 16.5a4 4 0 1 0 0-8 4 4 0 0 0 0 8z',
  matahari:
    'M12 8a4 4 0 1 0 0 8 4 4 0 0 0 0-8M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4',
  bulan: 'M20.5 14.3A8.5 8.5 0 0 1 9.7 3.5a8.5 8.5 0 1 0 10.8 10.8z',
  perangkat: 'M20 4H4a1 1 0 0 0-1 1v10a1 1 0 0 0 1 1h16a1 1 0 0 0 1-1V5a1 1 0 0 0-1-1M8 20h8M12 16v4',
  // Cermin dari `balas`: panah yang sama, arah yang berlawanan.
  teruskan: 'M15 14l5-5-5-5M20 9h-9a7 7 0 0 0-7 7v4',
  sematan: 'M9 3.5h6M10 3.5l-.6 6.2L6.5 13v2h11v-2l-2.9-3.3L14 3.5M12 15v5.5',
  bawah: 'M12 5v14m-6-6 6 6 6-6',
  mikrofon: 'M12 3a3 3 0 0 0-3 3v6a3 3 0 0 0 6 0V6a3 3 0 0 0-3-3zM19 11a7 7 0 0 1-14 0M12 18v3M8.5 21h7',
  // `reaksi` tanpa tanda tambah: yang ini menyisipkan, bukan memberi.
  emoji:
    'M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18zM8.5 14.5s1.3 1.7 3.5 1.7 3.5-1.7 3.5-1.7M9 9.5h.01M15 9.5h.01',
  hapus: 'M4 7h16M10 11v6M14 11v6M5.5 7l1 12a2 2 0 0 0 2 1.8h7a2 2 0 0 0 2-1.8l1-12M9 7V4.5h6V7',
  putar: 'M7 4.5v15l12.5-7.5z',
  jeda: 'M8 5v14M16 5v14',
  tambah: 'M12 5v14M5 12h14',
  peringatan: 'M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18zM12 7.5v5.5M12 16.5h.01',
  ulang: 'M3.5 12a8.5 8.5 0 0 1 14.9-5.6L20.5 8.5M20.5 3.5v5h-5M20.5 12a8.5 8.5 0 0 1-14.9 5.6L3.5 15.5M3.5 20.5v-5h5',
  // Satu centang: sampai di server. Dua centang (`terbaca`): sudah dibaca.
  terkirim: 'M5 12.5 9.5 17 19 7',
  // Pensil kecil untuk penanda "diedit" — `tulis` dengan garis dasar.
  'tulis-ulang': 'M12 20h8M4 20h3l10-10a2.12 2.12 0 0 0-3-3L4 17v3z',
  // Panah kecil di pojok gelembung yang membuka menu tindakan pesan.
  'buka-menu': 'M6 9.5l6 6 6-6',
  salin: 'M9 9h10a1 1 0 0 1 1 1v10a1 1 0 0 1-1 1H9a1 1 0 0 1-1-1V10a1 1 0 0 1 1-1zM5 15H4.5A1.5 1.5 0 0 1 3 13.5v-9A1.5 1.5 0 0 1 4.5 3h9A1.5 1.5 0 0 1 15 4.5V5',
};

export default function Icon({
  name,
  size = 20,
  className = '',
}: {
  name: IconName;
  size?: number;
  className?: string;
}) {
  return (
    <svg
      // Ikon di sini tidak pernah berdiri sendiri sebagai makna: tombolnya
      // selalu punya aria-label atau teks di sebelahnya. Jadi bagi pembaca
      // layar dia memang harus tidak ada, bukan dibacakan dua kali.
      aria-hidden="true"
      focusable="false"
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={1.75}
      strokeLinecap="round"
      strokeLinejoin="round"
      className={`shrink-0 ${className}`}
    >
      <path d={JALUR[name]} />
    </svg>
  );
}
```

## Avatar

- File: `web/src/components/Avatar.tsx`

```tsx
import { avatarURL } from '../api';
import type { StatusKind } from '../types';

/**
 * Foto profil, dengan huruf pertama nama sebagai cadangan.
 *
 * Dipakai di sidebar, kepala percakapan, daftar anggota, dan panel akun — dan
 * itu seluruh alasan dia jadi komponen sendiri. Empat tempat yang menggambar
 * lingkaran berisi huruf dengan caranya masing-masing adalah empat tempat yang
 * suatu hari akan berbeda ukurannya.
 */
export default function Avatar({
  name,
  url,
  size = 36,
  grup = false,
  dot,
}: {
  name: string;
  url?: string;
  size?: number;
  /**
   * Grup digambar sebagai persegi membulat, orang sebagai lingkaran.
   *
   * Bentuk, bukan warna atau ikon kecil di pojok: bentuk terbaca dari sudut
   * mata, pada ukuran berapa pun, dan tidak menuntut orang menghafal arti
   * sebuah lambang lebih dulu.
   */
  grup?: boolean;
  /** Titik keadaan di pojok kanan bawah; tidak digambar bila undefined. */
  dot?: DotKind;
}) {
  const rona = ronaDari(name);
  const bentuk = grup ? 'rounded-[30%]' : 'rounded-full';

  return (
    <span
      className={`relative inline-grid shrink-0 place-items-center font-bold ${bentuk}`}
      style={{
        width: size,
        height: size,
        fontSize: Math.round(size * 0.4),
        background: `oklch(var(--avatar-l) var(--avatar-c) ${rona})`,
        color: `oklch(var(--avatar-ink-l) var(--avatar-ink-c) ${rona})`,
      }}
    >
      {url ? (
        <img
          src={avatarURL(url)}
          alt=""
          // Alamatnya memuat id unggahan, jadi dia berubah tiap foto diganti —
          // itu yang membuat `loading="lazy"` dan cache setahun di sisi server
          // aman dipakai bersamaan: tidak ada alamat yang isinya pernah basi.
          loading="lazy"
          className={`size-full object-cover ${bentuk}`}
        />
      ) : (
        <span aria-hidden>{name.charAt(0).toUpperCase()}</span>
      )}

      {dot && (
        <span
          title={dotTitles[dot]}
          className={`absolute right-0 bottom-0 rounded-full border-2 border-surface ${dotColors[dot]}`}
          style={{ width: Math.max(9, size * 0.28), height: Math.max(9, size * 0.28) }}
        />
      )}
    </span>
  );
}

/**
 * Rona tetap untuk sebuah nama.
 *
 * Tujuh rona yang sudah dipilih agar rukun dengan palet, bukan nilai acak dari
 * seluruh lingkaran warna — yang terakhir itu cepat atau lambat menghasilkan
 * lingkaran yang berkelahi dengan teal di sebelahnya.
 *
 * Gunanya bukan hiasan: di grup berisi belasan orang tanpa foto, warna adalah
 * hal pertama yang dikenali mata sebelum hurufnya sempat dibaca. Karena
 * dihitung dari nama, orang yang sama selalu mendapat warna yang sama di setiap
 * perangkat, tanpa sekali pun perlu disimpan.
 */
const RONA = [196, 72, 152, 25, 285, 330, 248];

function ronaDari(name: string): number {
  let jumlah = 0;
  for (let i = 0; i < name.length; i++) jumlah = (jumlah * 31 + name.charCodeAt(i)) % 100003;
  return RONA[jumlah % RONA.length]!;
}

export type DotKind = 'online' | 'busy' | 'away';

const dotColors: Record<DotKind, string> = {
  online: 'bg-ok',
  busy: 'bg-danger',
  away: 'bg-call',
};

const dotTitles: Record<DotKind, string> = {
  online: 'Online',
  busy: 'Sedang sibuk',
  away: 'Sedang tidak di tempat',
};

/**
 * Menggabungkan presence dan status jadi SATU titik.
 *
 * Aturannya: status yang dinyatakan orang dengan sengaja selalu menang, dan
 * presence baru mengisi sisanya. Itu bukan pilihan sembarangan — seseorang yang
 * memasang "sedang rapat" lalu tetap membuka tabnya tidak sedang mengatakan
 * "sapa saya"; kalau presence yang menang, satu-satunya cara statusnya terlihat
 * adalah dengan menutup aplikasinya.
 *
 * Yang offline tanpa status tidak punya titik sama sekali. Titik abu-abu untuk
 * "tidak ada apa-apa" cuma menambah benda di layar yang artinya ketiadaan.
 */
export function dotFor(online: boolean, status?: StatusKind): DotKind | undefined {
  if (status === 'busy') return 'busy';
  if (status === 'away') return 'away';
  return online ? 'online' : undefined;
}

/** Kalimat status untuk baris keterangan, atau kosong bila tidak ada apa-apa. */
export function statusLabel(status?: StatusKind, text?: string, expiresAt?: string): string {
  const dasar = text || (status && status !== 'available' ? dotTitles[status as DotKind] : '');
  if (!dasar) return '';
  const sampai = untilLabel(expiresAt);
  return sampai ? `${dasar} · ${sampai}` : dasar;
}

/**
 * "sampai 13.00", dirender dalam jam LOKAL pembacanya.
 *
 * Server menyimpan instan absolut dan tidak pernah tahu zona waktu siapa pun.
 * Itu keputusan yang harus punya jawaban sebelum baris pertama ditulis:
 * "sampai jam 13.00" di jam siapa — dan jawabannya, jam orang yang membacanya.
 *
 * Yang sudah lewat menghasilkan kosong. Server sudah menyaringnya saat membaca,
 * tapi halaman yang terbuka berjam-jam tidak bertanya lagi — dan status yang
 * batas waktunya sudah lewat sementara halamannya masih terbuka adalah persis
 * kasus yang paling mungkin terjadi.
 */
export function untilLabel(expiresAt?: string): string {
  if (!expiresAt) return '';
  const at = new Date(expiresAt);
  if (Number.isNaN(at.getTime()) || at.getTime() <= Date.now()) return '';
  return (
    'sampai ' +
    at.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' })
  );
}
```

## Tanda

- File: `web/src/components/Tanda.tsx`

```tsx
/**
 * Lambang aplikasi: satu gelembung dengan ekornya.
 *
 * Muncul di dua tempat saja — layar tunggu dan halaman masuk — dan keduanya
 * adalah saat orang belum melihat apa pun lagi.
 */
export default function Tanda({ size = 44 }: { size?: number }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 44 44"
      aria-hidden="true"
      className="text-accent"
    >
      <path
        d="M8 17a9 9 0 0 1 9-9h10a9 9 0 0 1 9 9v6a9 9 0 0 1-9 9h-9l-7.5 5.5a1 1 0 0 1-1.6-.8V30.6A9 9 0 0 1 8 23z"
        fill="currentColor"
      />
      <circle cx="16.5" cy="20" r="2.1" fill="var(--color-accent-ink)" />
      <circle cx="22" cy="20" r="2.1" fill="var(--color-accent-ink)" />
      <circle cx="27.5" cy="20" r="2.1" fill="var(--color-accent-ink)" />
    </svg>
  );
}
```

## EmojiPicker

- File: `web/src/components/EmojiPicker.tsx`

```tsx
import { useEffect, useId, useMemo, useRef, useState, type KeyboardEvent } from 'react';
import { EMOJI_GROUPS, recentEmoji, rememberEmoji } from '../emoji';

/**
 * Papan emoji: satu baris kategori di atas, kisi yang bisa digulir di bawah.
 *
 * Dipakai di dua tempat — kolom tulis dan tombol reaksi — dan keduanya hanya
 * butuh satu hal darinya: emoji yang dipilih. Menutup papannya adalah urusan
 * pemakai, karena keduanya berbeda: kolom tulis membiarkannya terbuka supaya
 * orang bisa memilih beberapa sekaligus, reaksi menutupnya setelah satu.
 *
 * Tombol-tombolnya menahan `mousedown`. Tanpa itu, menekan emoji lebih dulu
 * mencabut fokus dari kolom tulis, dan posisi kursor tempat emoji itu
 * seharusnya disisipkan ikut hilang.
 */
export default function EmojiPicker({
  onPick,
  autoFocus = false,
}: {
  onPick: (emoji: string) => void;
  /** Dibuka lewat papan ketik: fokus langsung masuk ke emoji pertama. */
  autoFocus?: boolean;
}) {
  // Dibaca sekali saat papan dibuka. Memperbaruinya di setiap pilihan akan
  // menggeser isi baris "Terakhir" persis di bawah jari yang sedang menekan.
  const recent = useMemo(() => recentEmoji(), []);
  const groups = useMemo(
    () =>
      recent.length > 0
        ? [{ id: 'terakhir', label: 'Terakhir dipakai', icon: '🕘', items: recent }, ...EMOJI_GROUPS]
        : EMOJI_GROUPS,
    [recent],
  );
  const [active, setActive] = useState(groups[0]!.id);
  // Indeks datar emoji pertama tiap bagian, untuk roving tabindex.
  const offsets = useMemo(() => {
    const out: number[] = [];
    let n = 0;
    for (const g of groups) {
      out.push(n);
      n += g.items.length;
    }
    return out;
  }, [groups]);
  const scroller = useRef<HTMLDivElement>(null);
  const uid = useId();

  // Kisi emoji adalah SATU pemberhentian Tab (roving tabindex). Lima ratus
  // tombol yang masing-masing masuk urutan Tab membuat orang yang hanya
  // memakai papan ketik terjebak di sini sampai menyerah.
  const [cursor, setCursor] = useState(0);

  useEffect(() => {
    if (!autoFocus) return;
    scroller.current?.querySelector<HTMLButtonElement>('[data-emoji]')?.focus();
  }, [autoFocus]);

  const cells = () =>
    Array.from(scroller.current?.querySelectorAll<HTMLButtonElement>('[data-emoji]') ?? []);

  /** Panah kiri/kanan berpindah satu; atas/bawah ke baris tetangga, ke kolom terdekat. */
  function onGridKey(e: KeyboardEvent<HTMLDivElement>) {
    const all = cells();
    const at = all.indexOf(document.activeElement as HTMLButtonElement);
    if (at < 0) return;
    let next = -1;
    const here = all[at]!.getBoundingClientRect();
    const rowStep = (dir: 1 | -1) => {
      const candidates = all
        .map((b, i) => ({ i, r: b.getBoundingClientRect() }))
        .filter(({ r }) => (dir === 1 ? r.top > here.top + 2 : r.top < here.top - 2));
      if (candidates.length === 0) return -1;
      const rowTop =
        dir === 1
          ? Math.min(...candidates.map(c => c.r.top))
          : Math.max(...candidates.map(c => c.r.top));
      const row = candidates.filter(c => Math.abs(c.r.top - rowTop) < 2);
      return row.reduce((best, c) =>
        Math.abs(c.r.left - here.left) < Math.abs(best.r.left - here.left) ? c : best,
      ).i;
    };
    switch (e.key) {
      case 'ArrowRight':
        next = Math.min(all.length - 1, at + 1);
        break;
      case 'ArrowLeft':
        next = Math.max(0, at - 1);
        break;
      case 'ArrowDown':
        next = rowStep(1);
        break;
      case 'ArrowUp':
        next = rowStep(-1);
        break;
      case 'Home':
        next = 0;
        break;
      case 'End':
        next = all.length - 1;
        break;
      default:
        return;
    }
    e.preventDefault();
    if (next < 0) return;
    setCursor(next);
    all[next]!.focus();
    all[next]!.scrollIntoView({ block: 'nearest' });
  }

  const jump = (id: string) => {
    setActive(id);
    const el = scroller.current?.querySelector<HTMLElement>(`[data-group="${id}"]`);
    if (el && scroller.current) scroller.current.scrollTop = el.offsetTop;
    // Lompatan lewat papan ketik ikut memindahkan kursor kisi ke bagian itu.
    const first = el?.querySelector<HTMLButtonElement>('[data-emoji]');
    if (first) setCursor(cells().indexOf(first));
  };

  // Tab yang menyala mengikuti guliran, bukan hanya klik: orang yang menggulir
  // dari wajah ke makanan perlu tahu di mana dia sekarang.
  const onScroll = () => {
    const box = scroller.current;
    if (!box) return;
    let current = groups[0]!.id;
    for (const section of box.querySelectorAll<HTMLElement>('[data-group]')) {
      if (section.offsetTop - box.scrollTop <= 8) current = section.dataset.group!;
    }
    if (current !== active) setActive(current);
  };

  const pick = (emoji: string) => {
    rememberEmoji(emoji);
    onPick(emoji);
  };

  return (
    <div
      role="dialog"
      aria-label="Pilih emoji"
      // Di layar sentuh papannya lebih lebar dan kisinya tujuh kolom: sel
      // ±48 px, bukan ±38 px yang meleset di bawah ibu jari.
      className="w-[min(20rem,calc(100vw-1.5rem))] overflow-hidden rounded-2xl border border-line bg-surface shadow-pop [@media(pointer:coarse)]:w-[min(22rem,calc(100vw-1.5rem))]"
      onMouseDown={e => e.preventDefault()}
    >
      {/* Lompatan ke bagian, bukan tab: semua bagian tetap ada di satu
          daftar yang digulir, dan tombol ini hanya menggulirnya. */}
      <div role="group" aria-label="Kategori emoji" className="flex gap-0.5 border-b border-line px-1.5 py-1">
        {groups.map(g => (
          <button
            key={g.id}
            type="button"
            aria-current={active === g.id ? 'true' : undefined}
            aria-controls={`${uid}-${g.id}`}
            aria-label={`Lompat ke ${g.label}`}
            title={g.label}
            onClick={() => jump(g.id)}
            className={`grid h-9 flex-1 place-items-center rounded-lg text-lg transition ${
              active === g.id ? 'bg-accent-soft' : 'opacity-60 hover:bg-canvas hover:opacity-100'
            }`}
          >
            {g.icon}
          </button>
        ))}
      </div>

      <div
        ref={scroller}
        onScroll={onScroll}
        onKeyDown={onGridKey}
        className="relative h-64 overflow-y-auto px-1.5 pb-2"
      >
        {groups.map((g, gi) => (
          <section key={g.id} data-group={g.id} id={`${uid}-${g.id}`} aria-labelledby={`${uid}-${g.id}-judul`}>
            <h3
              id={`${uid}-${g.id}-judul`}
              className="sticky top-0 bg-surface/95 px-1 pt-2 pb-1 text-[12px] font-bold text-muted"
            >
              {g.label}
            </h3>
            <div className="grid grid-cols-8 [@media(pointer:coarse)]:grid-cols-7">
              {g.items.map((emoji, ii) => (
                <button
                  key={emoji}
                  type="button"
                  data-emoji=""
                  tabIndex={offsets[gi]! + ii === cursor ? 0 : -1}
                  onFocus={() => setCursor(offsets[gi]! + ii)}
                  onClick={() => pick(emoji)}
                  className="grid aspect-square place-items-center rounded-lg text-[22px] leading-none transition hover:bg-accent-soft"
                >
                  {emoji}
                </button>
              ))}
            </div>
          </section>
        ))}
      </div>
    </div>
  );
}
```

## QuickReactions

- File: `web/src/components/QuickReactions.tsx`

```tsx
import Icon from './Icon';
import { QUICK_REACTIONS } from '../emoji';

/**
 * Delapan reaksi cepat dan satu tombol "lainnya" — satu baris yang sama di
 * papan reaksi samping gelembung dan di menu/lembar tindakan pesan.
 *
 * `variant`:
 * - `popover` — papan kecil di samping gelembung dan di menu desktop (40px).
 * - `sheet`   — lembar tindakan di layar sentuh (48px, emoji besar).
 *
 * Tombol biasa, bukan butir menu: di dalam menu, panah atas/bawah melompati
 * baris ini dan Tab yang masuk ke sana.
 */
export default function QuickReactions({
  given,
  onReact,
  onMore,
  variant = 'popover',
}: {
  given: string[];
  onReact: (emoji: string) => void;
  onMore: () => void;
  variant?: 'popover' | 'sheet';
}) {
  const cell =
    variant === 'sheet'
      ? 'h-12 min-w-0 flex-1 text-[24px]'
      : // Di layar sempit sembilan sasaran berbagi lebar (±40 px × 44 px);
        // di layar lebar 40 × 40.
        'h-11 min-w-0 flex-1 text-lg sm:size-10 sm:flex-none';
  return (
    <div role="group" aria-label="Beri reaksi" className="flex items-center gap-0.5">
      {QUICK_REACTIONS.map(emoji => {
        const on = given.includes(emoji);
        return (
          <button
            key={emoji}
            type="button"
            aria-label={`Reaksi ${emoji}`}
            aria-pressed={on}
            onClick={() => onReact(emoji)}
            className={`grid place-items-center rounded-xl transition ${cell} ${
              on ? 'bg-accent-soft' : 'hover:bg-canvas'
            }`}
          >
            {emoji}
          </button>
        );
      })}
      <button
        type="button"
        aria-label="Emoji lainnya"
        title="Emoji lainnya"
        onClick={onMore}
        className={`grid place-items-center rounded-xl text-muted transition hover:bg-canvas hover:text-ink ${cell}`}
      >
        <Icon name="tambah" size={variant === 'sheet' ? 22 : 18} />
      </button>
    </div>
  );
}
```

