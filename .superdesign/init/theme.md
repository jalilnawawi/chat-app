# Theme & Design Tokens

Tailwind CSS v4, CSS-first. **There is no `tailwind.config.*` file** — every token is declared in `@theme` inside `web/src/index.css`. Dark mode is selected by the attribute `:root[data-tema='gelap']` (set by `web/src/tema.ts` before React paints); no `dark:` variants exist anywhere in the app — only the variable values change.

## Part 1 — Compact token summary

### Fonts
- `--font-sans`: `'Plus Jakarta Sans Variable', ui-sans-serif, system-ui, -apple-system, 'Segoe UI', sans-serif`
- Base body size: **15px**, line-height 1.5 (deliberately not 14px).

### Radius
- `--radius-bubble`: `1.125rem` (message bubble corners)
- `--radius-tail`: `7px` (tight corner on the sender side + runs of consecutive messages)

### Colors — light (`@theme`)
| Token | Value |
| --- | --- |
| `--color-canvas` | `oklch(0.966 0.018 192)` |
| `--color-surface` | `oklch(1 0 0)` |
| `--color-line` | `oklch(0.905 0.014 200)` |
| `--color-line-strong` | `oklch(0.62 0.025 205)` |
| `--color-line-bubble` | `oklch(0.905 0.014 200)` |
| `--color-ink` | `oklch(0.28 0.03 225)` |
| `--color-muted` | `oklch(0.525 0.025 222)` |
| `--color-accent` | `oklch(0.545 0.11 196)` (teal lagoon — own messages) |
| `--color-accent-ink` | `oklch(1 0 0)` |
| `--color-accent-soft` | `oklch(0.945 0.03 196)` |
| `--color-accent-deep` | `oklch(0.44 0.09 196)` (surfaces INSIDE a teal bubble) |
| `--color-accent-text` | `oklch(0.505 0.105 196)` (teal as standalone text) |
| `--color-call` | `oklch(0.78 0.15 72)` (mango — mentions/calls ONLY) |
| `--color-call-soft` | `oklch(0.945 0.045 80)` |
| `--color-call-ink` | `oklch(0.38 0.1 62)` |
| `--color-danger` | `oklch(0.52 0.19 25)` |
| `--color-danger-soft` | `oklch(0.955 0.025 25)` |
| `--color-ok` | `oklch(0.5 0.13 155)` |
| `--color-ok-soft` | `oklch(0.95 0.04 155)` |
| `--shadow-pop` | `0 12px 32px -12px oklch(0.28 0.03 225 / 0.28)` |

### Colors — dark (`:root[data-tema='gelap']`)
| Token | Value |
| --- | --- |
| `--color-canvas` | `oklch(0.185 0.018 228)` |
| `--color-surface` | `oklch(0.232 0.02 228)` |
| `--color-line` | `oklch(0.315 0.022 228)` |
| `--color-line-strong` | `oklch(0.52 0.026 228)` |
| `--color-line-bubble` | `oklch(0.4 0.026 228)` |
| `--color-ink` | `oklch(0.95 0.008 210)` |
| `--color-muted` | `oklch(0.72 0.02 220)` |
| `--color-accent` | `oklch(0.545 0.105 196)` |
| `--color-accent-soft` | `oklch(0.3 0.045 200)` |
| `--color-accent-deep` | `oklch(0.44 0.085 196)` |
| `--color-accent-text` | `oklch(0.8 0.09 196)` |
| `--color-call` | `oklch(0.8 0.145 72)` |
| `--color-call-soft` | `oklch(0.33 0.055 70)` |
| `--color-call-ink` | `oklch(0.38 0.1 62)` (stays dark on purpose) |
| `--color-danger` | `oklch(0.7 0.16 25)` |
| `--color-danger-soft` | `oklch(0.3 0.07 25)` |
| `--color-ok` | `oklch(0.78 0.13 160)` |
| `--color-ok-soft` | `oklch(0.3 0.06 160)` |
| `--shadow-pop` | `0 12px 32px -12px oklch(0 0 0 / 0.55)` |

### Avatar fallback tint (hue computed per name in `Avatar.tsx`)
- light: `--avatar-l: 0.9`, `--avatar-c: 0.055`, `--avatar-ink-l: 0.35`, `--avatar-ink-c: 0.09`
- dark: `--avatar-l: 0.42`, `--avatar-c: 0.07`, `--avatar-ink-l: 0.93`, `--avatar-ink-c: 0.03`

### Color discipline (hard rules in this codebase)
- One tone, one job. Mango (`call`) is **never** decoration — it only means "someone is calling your name" (mention / away status dot).
- Contrast ratios were computed before the palette was chosen; the app targets WCAG AA across light and dark.
- Teal on teal: white text on `--color-accent` is only 4.6:1, so nested surfaces inside an own-message bubble use `--color-accent-deep` (≈6.8:1 with white).

### Focus / motion
- Single app-wide focus ring: `:focus-visible:not(.cincin-sendiri) { outline: 2px solid var(--color-accent); outline-offset: 2px }`.
- `.cincin-sendiri` opts an element out (pill-shaped composer draws its own ring on the wrapper).
- Focused history row draws the ring on `.sasaran-fokus` (the bubble), not the full-width row.
- `[data-msg]:focus-visible .aksi-pesan { opacity: 1 }` — hover-only message actions must appear for keyboard users.
- Only two non-user-triggered animations exist: `denyut` (typing dots) and `rekam` (recording dot). `prefers-reduced-motion` kills durations but keeps the dots visible.
- `#root { height: 100dvh }` (dvh, not %, so the mobile URL bar can't clip the composer).

## Part 2 — Raw sources

### `web/src/index.css`

```css
@import 'tailwindcss';
/* Hanya varian tegak. Miringnya tidak dipakai di mana pun, dan berkas yang
   tidak pernah dipakai tetap ikut terunduh kalau diimpor. */
@import '@fontsource-variable/plus-jakarta-sans/wght.css';

/**
 * Palet: pagi di tepi air.
 *
 * Teal lagoon untuk suara sendiri, mangga untuk panggilan, kertas putih yang
 * disemburati mint supaya layar tidak terasa seperti dokumen kantor. Warnanya
 * dipilih setelah rasio kontrasnya dihitung, bukan sebaliknya — aplikasi ini
 * dipakai lintas umur, dan teks yang harus dikira-kira adalah teks yang gagal.
 *
 * Setiap nada punya SATU pekerjaan. Mangga tidak pernah muncul sebagai hiasan;
 * kalau dia ada di layar, artinya ada yang memanggil namamu. Isyarat yang
 * dipakai untuk dua hal berhenti jadi isyarat.
 */
@theme {
  --font-sans:
    'Plus Jakarta Sans Variable', ui-sans-serif, system-ui, -apple-system, 'Segoe UI', sans-serif;

  /* Lengkung gelembung. Satu angka, dipakai bersama oleh empat sudut yang
     berbeda-beda nasibnya — lihat MessageBubble. */
  --radius-bubble: 1.125rem;
  /* Ekor gelembung: sudut rapat di sisi pengirim, dan sambungan antarpesan
     dalam satu rentetan. */
  --radius-tail: 7px;

  --color-canvas: oklch(0.966 0.018 192);
  --color-surface: oklch(1 0 0);
  --color-line: oklch(0.905 0.014 200);
  /* Garis yang MEMBATASI sesuatu yang bisa ditekan atau diketik, bukan yang
     sekadar memisahkan. Cukup gelap untuk terlihat tanpa harus dicari. */
  --color-line-strong: oklch(0.62 0.025 205);
  /* Tepi gelembung orang lain. Di terang sama dengan garis pisah; di gelap
     dinaikkan sendiri, karena garis pisah gelap nyaris tidak terbedakan dari
     kanvas dan gelembungnya mengambang tanpa bentuk. */
  --color-line-bubble: oklch(0.905 0.014 200);
  --color-ink: oklch(0.28 0.03 225);
  --color-muted: oklch(0.525 0.025 222);

  --color-accent: oklch(0.545 0.11 196);
  --color-accent-ink: oklch(1 0 0);
  --color-accent-soft: oklch(0.945 0.03 196);
  /* Bidang di DALAM gelembung teal (kutipan, kartu berkas, kolom sunting).
     Lebih gelap, bukan putih transparan: putih di atas teal hanya 4,6:1, dan
     lapisan putih tipis menurunkannya lagi di bawah AA. Di atas bidang ini
     putih penuh mencapai ±6,8:1. */
  --color-accent-deep: oklch(0.44 0.09 196);
  /* Teal untuk TEKS berdiri sendiri: yang dipakai mengisi bidang terlalu terang
     untuk dibaca sebagai huruf kecil di atas kertas. */
  --color-accent-text: oklch(0.505 0.105 196);

  --color-call: oklch(0.78 0.15 72);
  --color-call-soft: oklch(0.945 0.045 80);
  --color-call-ink: oklch(0.38 0.1 62);

  --color-danger: oklch(0.52 0.19 25);
  --color-danger-soft: oklch(0.955 0.025 25);
  --color-ok: oklch(0.5 0.13 155);
  --color-ok-soft: oklch(0.95 0.04 155);

  --shadow-pop: 0 12px 32px -12px oklch(0.28 0.03 225 / 0.28);
}

/**
 * Warna avatar cadangan.
 *
 * Bukan tujuh warna tetap, melainkan satu terang dan satu gelap untuk RONA yang
 * dihitung dari namanya — lihat Avatar.tsx. Nilainya duduk di sini, bukan di
 * komponen, supaya lingkaran yang sama ikut redup saat temanya gelap alih-alih
 * menyala seperti lampu kecil di tengah layar malam.
 */
:root {
  --avatar-l: 0.9;
  --avatar-c: 0.055;
  --avatar-ink-l: 0.35;
  --avatar-ink-c: 0.09;
}

/**
 * Gelap bukan versi terang yang dibalik.
 *
 * Yang diganti hanya nilai variabelnya, bukan aturannya — jadi tidak ada satu
 * pun utilitas di seluruh aplikasi yang perlu menulis varian `dark:`. Peran tiap
 * nada tetap sama; yang berubah cuma dari mana cahayanya datang.
 *
 * Dipilih oleh ATRIBUT, bukan oleh media query, walau sebagian besar orang tidak
 * akan pernah memilih apa pun. Alasannya: dengan media query, palet gelap harus
 * ditulis DUA KALI — sekali untuk yang mengikuti sistem, sekali lagi untuk yang
 * memaksanya — dan dua salinan satu palet adalah dua salinan yang suatu hari
 * berbeda. Yang menerjemahkan "ikut sistem" jadi salah satu dari dua nilai di
 * sini adalah tema.ts, sebelum React sempat menggambar apa pun.
 */
:root[data-tema='gelap'] {
  --color-canvas: oklch(0.185 0.018 228);
  --color-surface: oklch(0.232 0.02 228);
  --color-line: oklch(0.315 0.022 228);
  --color-line-strong: oklch(0.52 0.026 228);
  --color-line-bubble: oklch(0.4 0.026 228);
  --color-ink: oklch(0.95 0.008 210);
  --color-muted: oklch(0.72 0.02 220);

  --color-accent: oklch(0.545 0.105 196);
  --color-accent-ink: oklch(1 0 0);
  --color-accent-soft: oklch(0.3 0.045 200);
  --color-accent-deep: oklch(0.44 0.085 196);
  --color-accent-text: oklch(0.8 0.09 196);

  --color-call: oklch(0.8 0.145 72);
  --color-call-soft: oklch(0.33 0.055 70);
  /* Tinta mangga tetap GELAP di tema gelap, dan itu bukan kelalaian: dia hanya
     pernah dipakai di atas `bg-call` yang terang — sebutan (@) dan titik status
     "tidak di tempat". Di atas `bg-call-soft` yang ikut gelap, pasangan ini
     turun ke 1,2:1; itu bukan pasangan yang sah, dan tidak ada lagi yang
     memakainya. */
  --color-call-ink: oklch(0.38 0.1 62);

  --color-danger: oklch(0.7 0.16 25);
  --color-danger-soft: oklch(0.3 0.07 25);
  --color-ok: oklch(0.78 0.13 160);
  --color-ok-soft: oklch(0.3 0.06 160);

  --shadow-pop: 0 12px 32px -12px oklch(0 0 0 / 0.55);

  --avatar-l: 0.42;
  --avatar-c: 0.07;
  --avatar-ink-l: 0.93;
  --avatar-ink-c: 0.03;
}

html,
body {
  height: 100%;
}

#root {
  /* dvh, bukan %: bilah alamat ponsel yang menyusut saat digulir akan memotong
     kolom tulis kalau tingginya diukur dari viewport yang tidak ikut berubah. */
  height: 100dvh;
}

body {
  background: var(--color-canvas);
  color: var(--color-ink);
  font-family: var(--font-sans);
  /* Dasarnya 15px, bukan 14px. Satu piksel ini yang membedakan "bisa dibaca"
     dari "bisa dibaca sambil menyipit" bagi sebagian besar pemakainya. */
  font-size: 15px;
  line-height: 1.5;
  -webkit-text-size-adjust: 100%;
  -webkit-font-smoothing: antialiased;
}

/* Teks contoh di kolom kosong. Bawaan Tailwind memudarkan warna huruf sampai
   setengah, dan di tema terang itu jatuh ke 2,97:1 — di bawah ambang yang
   dijanjikan halaman ini. Warna redup yang sudah dihitung dipakai sebagai
   gantinya. */
::placeholder {
  color: var(--color-muted);
  opacity: 1;
}

/* Satu cincin fokus untuk seluruh aplikasi, dan hanya untuk yang berpindah
   lewat papan ketik. Tanpa ini, orang yang tidak memakai tetikus kehilangan
   jejak posisinya begitu meninggalkan kolom tulis.

   Aturan ini tidak berlapis, jadi dia menang atas utilitas apa pun. Elemen
   yang menggambar fokusnya sendiri — kolom tulis berbentuk pil, yang cincinnya
   ada di wadahnya — menandai dirinya dengan `cincin-sendiri`. Tanpa jalan
   keluar ini, pil bulat itu mendapat kotak persegi di dalam lengkungnya. */
:focus-visible:not(.cincin-sendiri) {
  outline: 2px solid var(--color-accent);
  outline-offset: 2px;
}

/* Baris riwayat yang difokus (roving focus) menggambar cincinnya pada
   gelembung, bukan selebar baris: cincin selebar layar terbaca seperti
   pilihan baris tabel, bukan seperti pesan yang sedang dipilih. */
[data-msg]:focus-visible .sasaran-fokus {
  outline: 2px solid var(--color-accent);
  outline-offset: 3px;
}

/* Panah menu dan tombol reaksi bersembunyi sampai gelembungnya disentuh
   kursor. Orang yang berpindah dengan papan ketik tidak punya kursor untuk
   memunculkannya, jadi baris yang sedang difokus menampilkan keduanya —
   tindakan yang hanya bisa ditemukan dengan tetikus sama saja dengan tidak
   ada. */
[data-msg]:focus-visible .aksi-pesan {
  opacity: 1;
}

/* Di dalam gelembung sendiri latarnya teal: cincin teal dan celahnya lenyap
   di sana. Cincinnya ikut warna huruf gelembung itu. */
.gelembung-sendiri :focus-visible:not(.cincin-sendiri) {
  outline-color: var(--color-accent-ink);
}

::selection {
  background: var(--color-accent-soft);
  color: var(--color-ink);
}

/* Bilah gulir yang tidak ikut menuntut perhatian. */
* {
  scrollbar-width: thin;
  scrollbar-color: var(--color-line) transparent;
}

/**
 * Tiga titik "sedang mengetik".
 *
 * Satu-satunya gerak di aplikasi ini yang TIDAK dipicu orang, dan dia boleh ada
 * karena dia bukan hiasan: dia satu-satunya cara mengatakan bahwa ada yang
 * sedang menulis di seberang sana.
 */
@keyframes denyut {
  0%,
  60%,
  100% {
    opacity: 0.25;
    transform: translateY(0);
  }
  30% {
    opacity: 1;
    transform: translateY(-2px);
  }
}

.denyut > span {
  animation: denyut 1.2s infinite;
}
.denyut > span:nth-child(2) {
  animation-delay: 0.15s;
}
.denyut > span:nth-child(3) {
  animation-delay: 0.3s;
}

/**
 * Titik merah selama merekam — gerak kedua yang tidak dipicu orang, dengan
 * alasan yang sama: dia satu-satunya kabar bahwa mikrofonnya sedang mendengar.
 */
@keyframes rekam {
  0%,
  100% {
    opacity: 1;
  }
  50% {
    opacity: 0.3;
  }
}

.rekam {
  animation: rekam 1.2s ease-in-out infinite;
}

@media (prefers-reduced-motion: reduce) {
  *,
  *::before,
  *::after {
    animation-duration: 0.01ms !important;
    animation-iteration-count: 1 !important;
    transition-duration: 0.01ms !important;
  }

  /* Titiknya tetap harus terlihat walau tidak bergerak — yang diminta orang
     adalah gerakannya berhenti, bukan kabarnya hilang. */
  .denyut > span {
    opacity: 0.7;
  }
}

/**
 * Penggeser posisi yang memakai bentuk gelombang sebagai jalurnya.
 *
 * Yang digambar di belakang adalah 40 batang; yang menerima jari, papan ketik,
 * dan pembaca layar tetap `<input type="range">` yang sama seperti sebelum
 * Fase 15 — hanya jalur dan bulatannya yang dilepas. Menggambar gelombang
 * sebagai tombol atau div berarti menulis ulang seret, panah kiri-kanan,
 * Home/End, dan pengumuman posisi dengan tangan, dan empat-empatnya sudah ada
 * di sini tanpa satu baris pun.
 *
 * Bulatannya jadi garis tegak setinggi gelombang: penunjuk posisi pada bentuk
 * gelombang adalah garis, bukan kelereng yang menutupi tiga batang di bawahnya.
 */
.gelombang {
  appearance: none;
  -webkit-appearance: none;
  background: transparent;
}

.gelombang::-webkit-slider-runnable-track {
  height: 26px;
  background: transparent;
}

.gelombang::-moz-range-track {
  height: 26px;
  background: transparent;
}

.gelombang::-webkit-slider-thumb {
  -webkit-appearance: none;
  width: 3px;
  height: 26px;
  border-radius: 2px;
  background: currentColor;
}

.gelombang::-moz-range-thumb {
  width: 3px;
  height: 26px;
  border: 0;
  border-radius: 2px;
  background: currentColor;
}
```

### `web/src/tema.ts`

```ts
/**
 * Pilihan tampilan: ikut sistem, terang, atau gelap.
 *
 * Tiga keadaan, bukan dua. "Ikut sistem" bukan sekadar nilai awal yang kebetulan
 * dipakai sebelum orang memilih — dia pilihan tersendiri, dan satu-satunya yang
 * ikut berubah saat ponsel berpindah ke mode malam pada jam enam sore. Saklar
 * dua posisi memaksa orang membekukan salah satunya selamanya.
 *
 * Disimpan di perangkat, bukan di akun. Layar yang dipakai siang hari di kantor
 * dan layar yang dipakai di kamar sebelum tidur adalah dua layar yang berbeda,
 * walau akunnya satu.
 */
export type Tema = 'sistem' | 'terang' | 'gelap';

const KUNCI = 'tema';

/**
 * Warna bilah alamat ponsel, satu untuk tiap tema.
 *
 * Nilainya dieja di sini karena `<meta>` tidak bisa membaca variabel CSS, dan
 * karena oklch() belum aman dipakai di sana. Keduanya adalah --color-canvas
 * pada index.css; kalau yang di sana berubah, yang di sini ikut.
 */
const BILAH: Record<'terang' | 'gelap', string> = {
  terang: '#e7f8f7',
  gelap: '#0a1419',
};

const gelapDiSistem = () =>
  typeof window !== 'undefined' && window.matchMedia('(prefers-color-scheme: dark)').matches;

/** Pilihan yang tersimpan, atau "ikut sistem" bila belum pernah ada yang dipilih. */
export function temaTersimpan(): Tema {
  try {
    const nilai = localStorage.getItem(KUNCI);
    return nilai === 'terang' || nilai === 'gelap' ? nilai : 'sistem';
  } catch {
    // Mode penyamaran dan penyimpanan yang diblokir melempar di sini. Yang
    // hilang cuma ingatannya; aplikasinya tetap harus jalan.
    return 'sistem';
  }
}

/**
 * Menerjemahkan pilihan jadi keadaan yang benar-benar terpasang di halaman.
 *
 * Di sinilah "ikut sistem" berhenti jadi pilihan dan jadi salah satu dari dua
 * nilai — index.css hanya mengenal `terang` dan `gelap`.
 */
function terapkanTema(pilihan: Tema): void {
  const gelap = pilihan === 'gelap' || (pilihan === 'sistem' && gelapDiSistem());
  const akar = document.documentElement;

  akar.dataset.tema = gelap ? 'gelap' : 'terang';
  // Memberi tahu browser warna dasar halaman: yang ikut berubah karenanya
  // adalah bilah gulir, kolom isian bawaan, dan latar di balik halaman saat
  // digulir melewati ujungnya.
  akar.style.colorScheme = gelap ? 'dark' : 'light';

  document
    .querySelector('meta[name="theme-color"]')
    ?.setAttribute('content', BILAH[gelap ? 'gelap' : 'terang']);
}

/** Menyimpan pilihan sekaligus memasangnya. */
export function pilihTema(pilihan: Tema): void {
  try {
    if (pilihan === 'sistem') localStorage.removeItem(KUNCI);
    else localStorage.setItem(KUNCI, pilihan);
  } catch {
    // Tidak bisa diingat untuk kunjungan berikutnya, tapi masih bisa dipakai
    // sekarang. Menolak mengganti tema karena tidak bisa menyimpannya adalah
    // menolak mengerjakan bagian yang justru diminta orang.
  }
  terapkanTema(pilihan);
}

/**
 * Mengikuti sistem yang berganti selagi halaman terbuka.
 *
 * Bukan kasus langka: ponsel yang dijadwalkan masuk mode malam berganti sendiri
 * pada jam tertentu, dan halaman yang sedang terbuka saat itu akan berdiri
 * dengan warna kemarin sampai dimuat ulang. Hanya berlaku untuk yang memilih
 * "ikut sistem" — pilihan yang tegas tidak boleh ditimpa oleh jam berapa pun.
 */
export function pantauSistem(): () => void {
  const media = window.matchMedia('(prefers-color-scheme: dark)');
  const onChange = () => {
    if (temaTersimpan() === 'sistem') terapkanTema('sistem');
  };
  media.addEventListener('change', onChange);
  return () => media.removeEventListener('change', onChange);
}
```

### `web/vite.config.ts`

```ts
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';

const BACKEND = process.env.BACKEND_URL ?? 'http://127.0.0.1:8090';

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    // 5174 karena 5173 sering sudah dipakai project lain.
    port: 5174,
    strictPort: true,

    // '::' membuat Node mendengarkan dual-stack: localhost yang di-resolve ke
    // ::1 MAUPUN ke 127.0.0.1 sama-sama tersambung. Default Vite hanya mengikat
    // IPv6 loopback, sehingga browser yang memilih IPv4 lebih dulu gagal konek.
    host: '::',

    // Backend diteruskan lewat origin yang sama dengan halaman.
    //
    // Ini bukan sekadar kenyamanan. Kalau frontend di 127.0.0.1:5174 memanggil
    // API di localhost:8090, browser menganggapnya LINTAS SITE — "localhost"
    // dan "127.0.0.1" adalah site berbeda walau menunjuk mesin yang sama —
    // sehingga cookie sesi SameSite=Lax tidak ikut terkirim dan semua
    // permintaan setelah login jadi 401.
    //
    // Dengan proxy, apa pun yang diketik di address bar (localhost, 127.0.0.1,
    // atau IP LAN) selalu satu origin: cookie jalan, CORS tidak terlibat, dan
    // pemeriksaan origin WebSocket otomatis lolos.
    proxy: {
      '/api': { target: BACKEND },
      '/ws': { target: BACKEND, ws: true },
    },
  },
});
```

### `web/package.json`

```json
{
  "name": "chat-app-web",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "tsc --noEmit && vite build",
    "test": "bun test",
    "preview": "vite preview",
    "typecheck": "tsc --noEmit"
  },
  "dependencies": {
    "@fontsource-variable/plus-jakarta-sans": "^5.3.0",
    "react": "^19.2.8",
    "react-dom": "^19.2.8",
    "zustand": "^5.0.15"
  },
  "devDependencies": {
    "@tailwindcss/vite": "^4.3.3",
    "@types/bun": "^1.4.2",
    "@types/react": "^19.2.18",
    "@types/react-dom": "^19.2.7",
    "@vitejs/plugin-react": "^6.1.1",
    "tailwindcss": "^4.3.3",
    "typescript": "^7.0.2",
    "vite": "^8.2.2"
  }
}
```

