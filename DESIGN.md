---
name: Chat App
description: Chat realtime untuk tim yang dijalankan sendiri — segar, tenang, jelas.
colors:
  teal-laguna: "oklch(0.545 0.11 196)"
  teal-laguna-teks: "oklch(0.505 0.105 196)"
  teal-laguna-lembut: "oklch(0.945 0.03 196)"
  teal-dalam: "oklch(0.44 0.09 196)"
  putih-di-atas-teal: "oklch(1 0 0)"
  mangga-panggilan: "oklch(0.78 0.15 72)"
  mangga-lembut: "oklch(0.945 0.045 80)"
  mangga-tinta: "oklch(0.38 0.1 62)"
  kanvas-mint: "oklch(0.966 0.018 192)"
  kertas: "oklch(1 0 0)"
  garis-pisah: "oklch(0.905 0.014 200)"
  garis-batas: "oklch(0.62 0.025 205)"
  tepi-gelembung: "oklch(0.905 0.014 200)"
  tinta-laut-dalam: "oklch(0.28 0.03 225)"
  tinta-redup: "oklch(0.525 0.025 222)"
  bahaya: "oklch(0.52 0.19 25)"
  bahaya-lembut: "oklch(0.955 0.025 25)"
  berhasil: "oklch(0.5 0.13 155)"
  berhasil-lembut: "oklch(0.95 0.04 155)"
typography:
  display:
    fontFamily: "'Plus Jakarta Sans Variable', ui-sans-serif, system-ui, -apple-system, 'Segoe UI', sans-serif"
    fontSize: "24px"
    fontWeight: 800
    lineHeight: 1.33
    letterSpacing: "-0.025em"
  headline:
    fontFamily: "'Plus Jakarta Sans Variable', ui-sans-serif, system-ui, sans-serif"
    fontSize: "20px"
    fontWeight: 700
    lineHeight: 1.4
    letterSpacing: "-0.025em"
  title:
    fontFamily: "'Plus Jakarta Sans Variable', ui-sans-serif, system-ui, sans-serif"
    fontSize: "15px"
    fontWeight: 700
    lineHeight: 1.5
  body:
    fontFamily: "'Plus Jakarta Sans Variable', ui-sans-serif, system-ui, sans-serif"
    fontSize: "15px"
    fontWeight: 400
    lineHeight: 1.5
  message:
    fontFamily: "'Plus Jakarta Sans Variable', ui-sans-serif, system-ui, sans-serif"
    fontSize: "15px"
    fontWeight: 400
    lineHeight: 1.45
  label:
    fontFamily: "'Plus Jakarta Sans Variable', ui-sans-serif, system-ui, sans-serif"
    fontSize: "13px"
    fontWeight: 600
    lineHeight: 1.45
  meta:
    fontFamily: "'Plus Jakarta Sans Variable', ui-sans-serif, system-ui, sans-serif"
    fontSize: "11.5px"
    fontWeight: 400
    lineHeight: 1.4
    fontFeature: "'tnum'"
rounded:
  ekor: "7px" # token --radius-tail (rounded-*-tail)
  kecil: "8px"
  kontrol: "12px"
  melayang: "16px"
  gelembung: "18px"
  kartu: "20px"
  kolom-tulis: "22px"
  lembar: "24px"
  penuh: "9999px"
spacing:
  "1": "4px"
  "1.5": "6px"
  "2": "8px"
  "2.5": "10px"
  "3": "12px"
  "3.5": "14px"
  "4": "16px"
  "5": "20px"
components:
  button-primary:
    backgroundColor: "{colors.teal-laguna}"
    textColor: "{colors.putih-di-atas-teal}"
    typography: "{typography.body}"
    rounded: "{rounded.kontrol}"
    padding: "12px 16px"
  button-send:
    backgroundColor: "{colors.teal-laguna}"
    textColor: "{colors.putih-di-atas-teal}"
    rounded: "{rounded.penuh}"
    height: "44px"
    padding: "0 16px"
  button-soft:
    backgroundColor: "{colors.teal-laguna-lembut}"
    textColor: "{colors.teal-laguna-teks}"
    rounded: "{rounded.kontrol}"
    padding: "10px 12px"
  button-outline:
    backgroundColor: "{colors.kertas}"
    textColor: "{colors.tinta-laut-dalam}"
    typography: "{typography.label}"
    rounded: "{rounded.kecil}"
    padding: "6px 10px"
  button-icon:
    textColor: "{colors.tinta-redup}"
    rounded: "{rounded.kontrol}"
    size: "40px"
  button-icon-hover:
    backgroundColor: "{colors.kanvas-mint}"
    textColor: "{colors.tinta-laut-dalam}"
  input-field:
    backgroundColor: "{colors.kanvas-mint}"
    textColor: "{colors.tinta-laut-dalam}"
    typography: "{typography.body}"
    rounded: "{rounded.kontrol}"
    padding: "10px 14px"
  input-field-focus:
    backgroundColor: "{colors.kertas}"
  composer:
    backgroundColor: "{colors.kanvas-mint}"
    rounded: "{rounded.kolom-tulis}"
    height: "44px"
  bubble-mine:
    backgroundColor: "{colors.teal-laguna}"
    textColor: "{colors.putih-di-atas-teal}"
    typography: "{typography.message}"
    rounded: "{rounded.gelembung}"
    padding: "10px 14px"
  bubble-other:
    backgroundColor: "{colors.kertas}"
    textColor: "{colors.tinta-laut-dalam}"
    typography: "{typography.message}"
    rounded: "{rounded.gelembung}"
    padding: "10px 14px"
  message-menu-button:
    textColor: "{colors.tinta-redup}"
    rounded: "{rounded.penuh}"
    size: "28px"
  message-menu:
    backgroundColor: "{colors.kertas}"
    textColor: "{colors.tinta-laut-dalam}"
    rounded: "{rounded.melayang}"
    padding: "6px"
  message-menu-item-danger:
    textColor: "{colors.bahaya}"
    rounded: "{rounded.kontrol}"
    height: "44px"
  button-retry:
    backgroundColor: "{colors.kertas}"
    textColor: "{colors.bahaya}"
    rounded: "{rounded.penuh}"
    height: "44px"
    padding: "0 16px"
  conversation-item:
    textColor: "{colors.tinta-laut-dalam}"
    rounded: "{rounded.kontrol}"
    padding: "10px"
  conversation-item-active:
    backgroundColor: "{colors.teal-laguna-lembut}"
  chip-reaction:
    backgroundColor: "{colors.kertas}"
    textColor: "{colors.tinta-redup}"
    rounded: "{rounded.penuh}"
    padding: "2px 8px"
  chip-reaction-mine:
    backgroundColor: "{colors.teal-laguna-lembut}"
    textColor: "{colors.teal-laguna-teks}"
  badge-unread:
    backgroundColor: "{colors.teal-laguna}"
    textColor: "{colors.putih-di-atas-teal}"
    rounded: "{rounded.penuh}"
    padding: "2px 6px"
  badge-mention:
    backgroundColor: "{colors.mangga-panggilan}"
    textColor: "{colors.mangga-tinta}"
    rounded: "{rounded.penuh}"
    size: "22px"
  dialog:
    backgroundColor: "{colors.kertas}"
    rounded: "{rounded.lembar}"
    padding: "20px"
---

# Design System: Chat App

## Overview

**Creative North Star: "Pagi di Tepi Air"**

Layar ini seperti pagi yang cerah di tepi laguna. Kanvasnya kertas putih yang disemburati mint, suara sendiri berwarna teal laguna, dan mangga hanya muncul saat ada yang memanggil. Hasilnya segar, tenang, dan jelas. Aplikasinya dipakai seharian untuk kerja tim, jadi dia tidak boleh terasa seperti dokumen kantor. Dia juga tidak boleh menuntut perhatian yang tidak perlu.

Kepadatannya sedang: huruf dasar 15px, target sentuh 44px di kolom tulis, dan ruang napas yang cukup antara baris percakapan. Semua warna dipilih setelah kontrasnya dihitung, karena penggunanya lintas umur dan targetnya WCAG 2.2 AA. Teks yang harus dikira-kira adalah teks yang gagal.

Disiplin utamanya adalah sinyal. Setiap nada punya satu tugas, dan gerak hanya ada kalau dia satu-satunya cara menyampaikan kabar. Tema gelap bukan kebalikan tema terang: perannya sama, hanya sumber cahayanya yang berubah.

**Key Characteristics:**
- Kanvas mint pucat, permukaan kertas putih, dan tinta biru laut dalam, bukan hitam.
- Satu aksen teal untuk "milikku dan yang bisa ditekan"; mangga khusus untuk panggilan.
- Plus Jakarta Sans di seluruh aplikasi, dengan dasar 15px.
- Datar saat diam; bayangan hanya untuk benda yang melayang.
- Sudut lembut di mana-mana, dengan satu sudut rapat di gelembung sebagai ekor.
- Tiga keadaan tema (sistem, terang, gelap) yang dipilih lewat atribut `data-tema`.

## Colors

Palet sejuk dan terang: satu aksen teal, satu nada panggilan hangat, dan netral yang semuanya condong ke biru-hijau.

### Primary
- **Teal Laguna** (`teal-laguna`): bidang untuk "suaraku" dan aksi utama. Dipakai di gelembung pesan sendiri, tombol kirim, tombol utama, lencana belum dibaca, dan cincin fokus. Teks di atasnya selalu **Putih di Atas Teal**.
- **Teal Laguna untuk Teks** (`teal-laguna-teks`): versi sedikit lebih gelap untuk huruf yang berdiri sendiri di atas kertas, misalnya nama penulis di grup, tanda terbaca, dan tautan. Teal untuk bidang terlalu terang untuk huruf kecil.
- **Teal Laguna Lembut** (`teal-laguna-lembut`): latar untuk keadaan terpilih, misalnya percakapan aktif, reaksi milik sendiri, tombol lunak, dan warna seleksi teks.
- **Teal Dalam** (`teal-dalam`, `--color-accent-deep`): bidang di DALAM gelembung sendiri — kutipan balasan, kartu berkas, kolom sunting, chip kecepatan, sorotan sebutan. Putih penuh di atasnya ±7:1.

### Secondary
- **Mangga Panggilan** (`mangga-panggilan`): hanya untuk "ada yang memanggil namamu". Dipakai sebagai cincin 2px di gelembung yang menyebut kita, lencana `@` di daftar percakapan, dan titik status "sedang tidak di tempat". **Mangga Tinta** adalah huruf di atasnya; **Mangga Lembut** latarnya yang pucat.

### Neutral
- **Kanvas Mint** (`kanvas-mint`): latar halaman, area riwayat pesan, isi kolom isian saat diam, dan hover tombol ikon.
- **Kertas** (`kertas`): permukaan panel, header, kolom tulis, dialog, dan gelembung orang lain.
- **Garis Pisah** (`garis-pisah`): garis yang hanya memisahkan, misalnya tepi panel, tepi header, dan batas gelembung orang lain.
- **Garis Batas** (`garis-batas`): garis di sekeliling sesuatu yang bisa ditekan atau diketik. Cukup gelap untuk terlihat tanpa dicari.
- **Tepi Gelembung** (`tepi-gelembung`, `--color-line-bubble`): garis gelembung orang lain. Di tema terang sama dengan Garis Pisah; di tema gelap naik ke `oklch(0.4 0.026 228)` supaya gelembung tidak mengambang tanpa bentuk.
- **Tinta Laut Dalam** (`tinta-laut-dalam`): teks utama. Juga dipakai untuk tirai dialog (40%).
- **Tinta Redup** (`tinta-redup`): teks kedua, jam, pratinjau pesan, dan ikon yang sedang diam.

### Status
- **Bahaya** / **Bahaya Lembut**: pesan gagal, rekaman aktif, aksi hapus, dan status "sibuk".
- **Berhasil** / **Berhasil Lembut**: pemberitahuan sukses dan status "online".

### Tema gelap
Tema gelap mengganti nilai variabel yang sama di `:root[data-tema='gelap']`. Utilitas tidak pernah memakai varian `dark:`. Kanvas turun ke `oklch(0.185 0.018 228)` dan kertas ke `oklch(0.232 0.02 228)`. Teal untuk teks naik ke `oklch(0.8 0.09 196)`. Avatar cadangan ikut meredup lewat `--avatar-l` dan `--avatar-c`.

### Named Rules
**The One Signal, One Meaning Rule.** Setiap nada punya satu tugas. Mangga tidak pernah muncul sebagai hiasan. Kalau mangga ada di layar, artinya ada yang memanggil namamu.

**The Ring, Not Fill Rule.** Latar gelembung sudah membedakan pesan sendiri dari pesan orang. Karena itu, sebutan ditandai dengan cincin, bukan dengan bidang.

**The Contrast First Rule.** Rasio kontras dihitung sebelum warna dipilih, bukan sebaliknya. Warna baru wajib lolos WCAG 2.2 AA di kedua tema.

**The Solid White On Teal Rule.** Putih di atas Teal Laguna hanya 4,6:1. Teks di dalam gelembung sendiri selalu putih PENUH — tanpa opacity, tanpa putih transparan. Pembeda dibuat lewat bobot, ukuran, atau bidang Teal Dalam.

## Typography

**Body Font:** Plus Jakarta Sans Variable (fallback: ui-sans-serif, system-ui, -apple-system, Segoe UI). Font dipasang lokal lewat `@fontsource-variable`, hanya varian tegak.

**Character:** Satu keluarga huruf geometris yang hangat untuk semua peran. Hierarki dibangun dari bobot, bukan dari keluarga huruf kedua.

### Hierarchy
- **Display** (800, 24px, tracking rapat): hanya untuk wordmark "Chat" di halaman masuk.
- **Headline** (700, 20px; dialog memakai 18px, tracking rapat): judul kartu masuk, judul dialog, dan judul panel.
- **Title** (700, 15px): nama di daftar percakapan, nama pengguna di header, dan judul percakapan.
- **Body** (400, 15px, line-height 1.5): teks umum dan isian formulir. Dasar 15px adalah keputusan keterbacaan, bukan selera.
- **Message** (400, 15px, line-height 1.45): isi gelembung.
- **Label** (600, 13px): label kolom isian, nama penulis di grup (700, teal), chip, dan pratinjau pesan (400).
- **Meta** (400, 11.5–12px, angka tabular): jam, penanda hari, dan jumlah reaksi.

### Named Rules
**The Fifteen Pixel Floor Rule.** Teks yang dibaca, bukan dipindai, tidak pernah di bawah 15px. Ukuran 13px ke bawah hanya untuk label dan meta.

**The One Family Rule.** Jangan menambah keluarga huruf kedua. Pakai bobot 400–800 untuk membangun hierarki.

## Layout

Susunannya tiga kolom di dalam bingkai maksimal 1480px, dengan garis tepi mulai `lg`. Sidebar percakapan lebarnya 320px, riwayat pesan mengisi sisa ruang, dan panel samping opsional lebarnya 320px (panel cari 384px).

Di bawah `md`, daftar dan percakapan bergantian mengisi satu kolom. Yang menentukan tampilan adalah `activeId`, bukan state tampilan tersendiri. Panel samping berubah menjadi lapisan layar penuh. Dialog naik dari bawah sebagai lembar, lalu mulai `sm` mengapung di tengah dengan lebar maksimal `28rem`.

Ritme ruang memakai kelipatan 4px, dengan langkah setengah yang sering dipakai (6, 10, 14). Rentetan pesan dari satu orang berjarak 2px, sedangkan pembuka rentetan baru berjarak 12px. Lebar gelembung maksimal `min(80%, 34rem)`. Tinggi akar memakai `100dvh`, supaya bilah alamat ponsel tidak memotong kolom tulis.

## Elevation & Depth

Sistem ini datar. Kedalaman di permukaan yang diam dibangun dari lapisan nada, yaitu kertas di atas kanvas mint, plus garis 1px. Bayangan hanya dipakai untuk benda yang benar-benar melayang di atas konten: dialog, kartu masuk, pemilih reaksi, pemilih emoji, dan tombol "ke bawah".

### Shadow Vocabulary
- **Pop** (`box-shadow: 0 12px 32px -12px oklch(0.28 0.03 225 / 0.28)`; gelap: `oklch(0 0 0 / 0.55)`): satu-satunya bayangan di sistem ini, khusus untuk lapisan yang melayang: dialog, menu ⋯, pemilih, dan tombol ⋯ yang sedang terbuka.
- **Tirai** (`background: tinta-laut-dalam / 40%` + `backdrop-filter: blur(2px)`): latar di belakang dialog modal.

### Named Rules
**The Flat At Rest Rule.** Panel, header, gelembung, dan daftar tidak pernah memakai bayangan. Kalau sebuah benda tidak melayang, beri dia garis, bukan bayangan.

**The Undo Over Confirm Rule.** Tindakan merusak yang berlaku untuk semua orang ditahan sebentar dan menawarkan "Urungkan", bukan dialog "Yakin?". Tindakan yang tidak bisa ditahan (membuang rekaman panjang) memakai ketukan kedua.

## Shapes

Bentuknya lembut dan konsisten. Kontrol memakai sudut 12px, lapisan melayang 16px, gelembung 18px, kartu 20px, kolom tulis 22px, dan lembar dialog 24px. Tombol kirim, avatar, chip, dan lencana bulat penuh.

Gelembung adalah bentuk khasnya. Tiga sudut bulat penuh (18px), dan satu sudut di sisi pengirim dirapatkan menjadi 7px sebagai ekor. Kalau pesan bukan pembuka rentetan, sudut atas di sisi yang sama ikut dirapatkan. Hasilnya, satu rentetan terbaca sebagai satu blok dengan tulang punggung lurus.

Ikon adalah ikon garis buatan sendiri di kotak 24×24 (lihat `Icon.tsx`). Emoji hanya dipakai sebagai isi (reaksi, pratinjau), tidak pernah sebagai lambang tombol. Lambang aplikasi adalah satu gelembung teal dengan ekor dan tiga titik.

## Components

### Buttons
Lembut tapi tegas: bentuknya membulat, warnanya pasti, dan tombol bereaksi saat ditekan.
- **Shape:** sudut kontrol (12px) untuk tombol blok; bulat penuh untuk tombol kirim dan rekam.
- **Primary:** bidang Teal Laguna dengan teks putih, 15px semibold, padding 12px × 16px, lebar penuh di formulir.
- **Send:** pil Teal Laguna setinggi 44px dengan ikon, plus label mulai `sm`.
- **Hover / Active:** `brightness(1.1)` saat hover; `scale(0.99)` untuk tombol blok, `scale(0.95)` untuk tombol kirim. Tombol nonaktif memakai opacity 40–50%.
- **Soft:** latar Teal Laguna Lembut dengan teks teal, misalnya "Percakapan baru".
- **Outline:** garis batas, 13px medium. Saat hover, garis dan teks berubah menjadi teal.
- **Icon:** kotak 40px (44px di kolom tulis), ikon Tinta Redup. Saat hover, latar berubah ke Kanvas Mint dan ikon ke Tinta Laut Dalam. Tombol yang sedang aktif memakai garis teal dan latar Teal Laguna Lembut.

### Chips
- **Reaction:** pil dengan garis pisah, emoji, dan jumlah 13px. Reaksi milik sendiri memakai garis teal, latar Teal Laguna Lembut, dan teks teal tebal.
- **Suggestion / Jump:** pil kertas dengan garis pisah, 13px medium, dan teks redup. Saat hover, garis dan teks berubah menjadi teal.
- **Day divider:** pil `tinta / 8%`, 12px tebal, teks redup.

### Cards / Containers
- **Corner Style:** 20px untuk kartu masuk, 24px untuk dialog (hanya sudut atas di ponsel), 16px untuk lapisan kecil.
- **Background:** Kertas.
- **Shadow Strategy:** Pop, hanya untuk yang melayang (lihat Elevation & Depth).
- **Border:** Garis Pisah 1px.
- **Internal Padding:** 20px untuk dialog, 28px untuk kartu masuk.

### Inputs / Fields
- **Style:** latar Kanvas Mint, garis 1px Garis Batas, sudut 12px, padding 10px × 14px, teks 15px. Label 13px semibold redup ada di atas kolom.
- **Focus:** latar berubah ke Kertas. Cincin fokus global muncul: garis 2px Teal Laguna dengan jarak 2px, hanya untuk `:focus-visible`.
- **Error:** kotak Bahaya Lembut dengan teks Bahaya, sudut 12px, di bawah formulir.

### Navigation
- **Conversation list:** baris dengan sudut 12px, padding 10px, avatar 44px, nama 15px, dan pratinjau 13px redup. Percakapan aktif memakai latar Teal Laguna Lembut dan `aria-current`; hover memakai Kanvas Mint. Percakapan belum dibaca memakai nama tebal, dengan lencana jumlah teal atau lencana `@` mangga di kanan.
- **Header:** latar Kertas dengan garis bawah, tombol ikon 40px, dan tombol kembali yang hanya muncul di bawah `md`.

### Composer (Signature)
Kolom tulis berbentuk pil setinggi minimal 44px, dengan sudut 22px, latar Kanvas Mint, dan garis Garis Batas. Saat ada fokus di dalamnya, garis berubah menjadi teal dan latar menjadi Kertas. Tombol lampiran dan emoji bulat 44px ada di dalam pil. Tombol kirim, atau tombol mikrofon saat kolom kosong, ada di luar pil. Saat merekam, pil berisi titik merah berdenyut dan gelombang batang teal 3px.

### Message Bubble (Signature)
Pesan sendiri memakai bidang Teal Laguna dengan teks putih dan rata kanan. Pesan orang lain memakai Kertas dengan Tepi Gelembung dan rata kiri, plus avatar 32px di pembuka rentetan. Pesan yang dihapus memakai garis putus-putus, teks redup, dan huruf miring. Kutipan balasan memakai garis kiri 3px dan sudut 8px, dengan warna yang menyesuaikan gelembungnya. Baris meta ada di bawah gelembung (11.5px): jam, ikon sematan, "diedit" dengan ikon pensil, lalu satu centang (terkirim) atau dua centang teal (dibaca, khusus DM). Setiap ikon meta punya teks `sr-only`.

- **Gagal kirim:** cincin Bahaya 2px dengan jarak 2px dari gelembung. Tanpa opacity; teks tetap kontras penuh. Di bawahnya ada baris `role="alert"` berisi ikon peringatan, sebab 13px, tombol "Tulis ulang" (teksnya kembali ke kolom tulis) dan pil "Kirim ulang" 44px bergaris Bahaya. Riwayat menggulir ke baris itu saat statusnya berubah.
- **Fokus di gelembung sendiri:** cincin fokus memakai warna huruf gelembung (`.gelembung-sendiri`), karena cincin teal lenyap di atas teal.
- **Sedang diedit:** kolom sunting di dalam gelembung, petunjuk "Enter simpan · Esc batal", serta tombol Batal dan Simpan 36px. Menyimpan hanya lewat perintah, tidak pernah saat kolom kehilangan fokus.

### Message Menu (Signature)
Panah kecil ke bawah (lingkaran 28px, 4px dari tepi atas dan kanan) di pojok kanan atas DI DALAM gelembung. Warnanya mengikuti gelembung: putih di atas teal untuk pesan sendiri, redup di atas Kertas untuk pesan orang. Tanpa bayangan dan tanpa efek pudar.
- **Ruang sudut:** gelembung memesan sudutnya dengan `MenuCornerSpacer` (28×28, mengambang kanan) sehingga hanya baris pertama yang membungkus. Spacer diletakkan di DALAM paragraf teks bila teks adalah baris pertama, supaya ikut dihitung dalam lebar gelembung. Kutipan dibungkus `flow-root` agar menyempit di samping panah. Gelembung berisi lampiran saja (pesan suara, berkas) memesan `padding-right` 36px; hanya gelembung berisi foto saja yang membiarkan panah berdiri di atas isinya.
- **Area tekan:** dengan tetikus diperluas ke atas, kanan, dan kiri (±40px), hampir tidak ke bawah, supaya memilih teks tidak membuka menu. Di pointer kasar diperluas 8px ke semua arah (44×44).
- **Tetikus:** selalu terlihat dengan opacity 80% (kontras ikon tetap ≥3:1), menyala penuh saat BARIS pesannya di-hover — cakupan yang sama dengan tombol reaksi — atau saat difokus. Klik kanan pada gelembung membuka menu yang sama, kecuali ada teks yang sedang dipilih.
- **Layar sentuh (`hover: none`):** selalu tampil; tekan lama (Android) membuka menu. Tombol reaksi di samping gelembung TIDAK ada di layar sentuh.
- Saat menu terbuka, panahnya berputar 180°. Label aksesibelnya menyebut pengirim dan cuplikan pesan; tombolnya berada setelah isi pesan di DOM.
- Dibuka dengan papan ketik, fokus masuk ke butir pertama; dibuka dengan tetikus atau jari, fokus masuk ke wadah menu supaya tidak ada butir yang tampak terpilih.

Di atas daftar ada baris reaksi cepat (`QuickReactions`: 8 emoji + "tambah" yang membuka papan emoji lengkap di wadah yang sama; komponen yang sama dipakai papan reaksi samping gelembung) — di layar sentuh satu-satunya jalan memberi reaksi. Tombol biasa, bukan butir menu. Isi menunya: Balas, Salin teks, Teruskan, Sematkan, Edit, lalu garis pemisah dan "Hapus untuk semua" berwarna Bahaya, selalu paling akhir.
- **Layar ≥ 40rem:** popover Kertas selebar 320px, sudut 16px, bayangan Pop; reaksi 36px. Posisinya di bawah tombol, atau di atas bila tidak muat. Butir menu setinggi 44px.
- **Layar sempit:** lembar dialog dari bawah dengan tirai, sudut atas 24px, cuplikan pesan sasaran di kepala (garis kiri teal 3px), baris reaksi 48px 24px, butir 52px 16px, dan "Batal" di dasar sebagai tombol biasa (bukan butir menu). Fokus berputar di dalam lembar.
- **Keyboard:** `role="menu"`, panah/Home/End berpindah butir, Esc menutup dan mengembalikan fokus ke tombol ⋯.

### Riwayat dan papan ketik
Riwayat adalah `role="log"` dengan satu pemberhentian Tab (roving focus, `useRovingLog`). Setiap baris adalah `role="article"` berlabel "siapa, jam: cuplikan", ditambah jumlah lampiran, "menyebut kamu", dan jumlah reaksi; catatan sistem dibacakan kalimat lengkapnya. Tombol di dalam baris baru masuk urutan Tab setelah fokus berada di baris itu. Tombol emoji dan lampiran berada di KANAN kolom tulis, jadi Shift+Tab dari kolom tulis mendarat langsung di baris pesan; Esc di kolom tulis yang kosong juga melompat ke pesan terbaru. Saat pesan baru datang dan fokus tidak di riwayat, baris yang masuk urutan Tab ikut pindah ke pesan terbaru. Panah atas/bawah dan Home/End berpindah baris; Enter, tombol menu, atau Shift+F10 membuka tindakan pesan. Hanya baris terpilih yang tombol dan tautannya ada di urutan Tab — aturan ini ditegakkan pada DOM, jadi chip reaksi, lampiran, pemutar suara, dan catatan sistem ikut tanpa prop. Cincin fokus baris digambar pada gelembungnya (`.sasaran-fokus`, jarak 3px), bukan selebar baris. Di kolom tulis yang kosong, ↑ menyunting pesan terakhir sendiri. Aturan gulir (`useHistoryScroll`): "pembaca pergi dari bawah" hanya dicatat untuk gulir yang dilakukan orangnya (roda, sentuh, papan ketik, penggeser) — gulir yang dilakukan browser sendiri (scroll anchoring saat bilah di atas riwayat muncul) tidak memutus tempelan. Pindah ruang, isian pertama, dan pesan SENDIRI yang baru dikirim selalu ke bawah; pesan orang lain hanya bila pembaca di dekat bawah; tetap menempel di bawah saat tinggi area baca atau isinya berubah (gambar yang dimuat belakangan); pesan yang baru gagal ikut digulir ke layar.
- **Pil "N pesan baru" / "Ke pesan terbaru":** pil teal 44px berbayangan Pop, menempel di tepi bawah riwayat setiap kali pembaca lebih dari 200px dari bawah (dan selama jendela lama terbuka). Jumlahnya hanya menghitung pesan yang menambah UJUNG riwayat, bukan pesan lama yang dimuat ke atas.

### Keadaan riwayat
- **Kolom baca** dibatasi 56rem dan diletakkan di tengah; di layar lebar, giliran bicara tidak berjarak selebar layar.
- **Penanda hari — garis cakrawala (tanda khas):** pil Kertas bergaris, 12px tebal redup (5,3:1), duduk di atas garis Garis Pisah yang memudar ke kedua tepi — matahari di atas batas air. Tidak ada warna baru.
- **Salam menurut jam** ("Selamat pagi/siang/sore/malam") membuka keadaan kosong: "pagi" hidup di waktu, bukan di warna.
- **Pesan terhapus** tidak membawa baris jam; jamnya dibacakan lewat label baris. Selama jeda urungkan gelembungnya berbunyi "Menghapus…", bukan "Pesan ini dihapus".
- **Catatan "lepas sematan"** ditulis tanpa pil (tidak menunjuk ke mana pun).
- **Pesan terhapus berurutan** dari orang yang sama di hari yang sama dilipat jadi satu pil putus-putus 13px miring: "N pesan dihapus".

### Kolom tulis dan layar sentuh
- Di pointer kasar, Enter membuat baris baru dan yang mengirim adalah tombolnya (juga saat menyunting); petunjuk di keadaan kosong memakai bahasa jari.
- Di layar lebar bertetikus, petunjuk pintasan tampil di bawah kolom tulis selama kolomnya difokus (13px redup), satu-satunya tempat pintasan tertulis.
- Tombol rekam saat diam bergaris (Garis Batas, teks teal), bukan berbidang, supaya tidak mengundang rekaman tak sengaja. Tombol di dalam pil (emoji, lampiran) memakai cincin fokus di dalam (`ring-inset`).
- Kolom tulis dan kolom sunting tumbuh mengikuti isinya (termasuk baris yang membungkus) sampai ±6 baris, lalu menggulir.
- Label "Kirim" dan "Rekam" tetap tampil di ponsel.
- Papan emoji di pointer kasar: tujuh kolom, sel ±48px.
- Bilah rekam menulis "Mendengarkan…" di 1,5 detik pertama, sebelum batang suara muncul.
- Tombol di dalam pil kolom tulis memakai `cincin-sendiri` DAN `outline-none`, dengan cincin inset.
- Papan emoji: kisinya satu pemberhentian Tab; panah berpindah emoji (atas/bawah ke kolom terdekat), Home/End ke ujung. Belum ada pencarian — daftar emoji tidak membawa nama.

### Delete Notice
Hapus ditahan 8 detik, dan tetap dikirim bila tab ditutup dalam jeda itu (`pagehide`, `keepalive`). Selama itu gelembung tampil "dihapus", dan bilah Kertas di atas kolom tulis berbunyi "Menghapus untuk semua orang… “cuplikan”" dengan hitung mundur detik, garis Bahaya 2px yang memendek di tepi atas, dan tombol teks teal "Urungkan" setinggi 40px.
- Beberapa hapus digabung ("Menghapus 2 pesan…") dengan satu Urungkan, lintas percakapan — pindah ruang tidak menyembunyikannya.
- Hitung mundur dijeda ("dijeda") selama kabar di-hover atau difokus lewat papan ketik, dan dilanjutkan dengan paling sedikit 2 detik.
- Fokus yang hilang bersama menu pesan dipindah ke Urungkan; setelah Urungkan, fokus kembali ke baris pesannya.
- Pengumuman ke pembaca layar lewat satu wilayah status yang selalu terpasang.
- Bila server menolak: bilah Bahaya Lembut dengan sebab, tombol "Biarkan" dan "Coba lagi".

### Tombol ikon (`ui/IconButton`)
Tombol ikon 40px (44px di pointer kasar) untuk kepala percakapan, penutup kabar, dan pembatal balasan. Label wajib; `pressed` memberi `aria-pressed` dan nada Teal Laguna Lembut.

### Tombol pil (`ui/PillButton`)
Satu komponen untuk semua tombol berbentuk pil: `solid` (tindakan utama; nonaktif = bergaris Kertas, teks redup), `outline` (Garis Batas, teks teal — tombol rekam, Urungkan), `danger` (Kirim ulang, Coba lagi), `quiet` (Muat pesan lama/berikutnya), `ghost` (Tulis ulang, Biarkan), dan `onTealSolid`/`onTealGhost` untuk form sunting di dalam gelembung sendiri. Ukuran `md` 44px, `sm` 40px; `compact` menyembunyikan label di layar sempit.

### Tumpukan kabar (`NoticeStack`)
Di atas kolom tulis paling banyak SATU kabar, berurutan: hapus yang ditolak server → hapus yang menunggu → kabar singkat. Kabar singkat boleh membawa satu tombol dan bertahan (`sticky`) — dipakai sematan: "Menyematkan pesan… · Urungkan" selama 4 detik sebelum sematan dikirim dan menjadi catatan untuk semua orang. Pengumuman yang sama dua kali tetap dibacakan ulang. Bilah koneksi dan galat sematan menempel di bawah kepala percakapan; "sedang mengetik" di wilayah statusnya sendiri. Yang tidak tampil tetap diumumkan.

### Bilah sematan
Satu baris di semua layar: di layar lebar judul "Disematkan" teal tebal duduk di depan kalimat; di ponsel judulnya hanya untuk pembaca layar dan tombolnya "Lihat".

### Salinan dan kabar singkat
"Teks disalin." dan kabar sejenis tampil 2.5 detik sebagai bilah Kertas dengan centang Berhasil; kabar peringatan memakai ikon peringatan Bahaya. Keduanya diumumkan lewat wilayah status yang selalu terpasang.

### Pemutar pesan suara (Signature)
Bukan `<audio controls>`: lingkaran putar 40px, bentuk gelombang di tengah, dan tombol kecepatan `1×`/`1,5×`/`2×` di kanan, dengan durasi (atau posisi selama diputar) di bawah gelombang bersama ikon mikrofon. Lebarnya tetap (`w-68`) tapi tidak pernah melebihi gelembungnya.
- **Bentuk gelombang:** 40 batang selebar 3px dengan celah 1px, tinggi maksimum 24px dan minimum 2px — sama dengan batang bilah perekam, karena keduanya menggambar hal yang sama. Batang yang sudah lewat berwarna penuh (Putih di Atas Teal / Teal Laguna), sisanya diredupkan (putih 45% / Garis Batas 55%).
- Gelombang adalah lapisan gambar di BELAKANG `<input type="range">` yang menutupi kotak yang sama; penunjuk posisinya garis tegak 3px setinggi gelombang, bukan kelereng. Seret, panah papan ketik, dan pembacaan posisi tetap milik penggeser itu.
- Rekaman tanpa gelombang — berkas audio yang diunggah dari disk — memakai penggeser polos dengan jalur dan bulatan bawaan.

### Conversation States
- **Memuat:** kerangka lima gelembung diam (tanpa kilau), teal lembut untuk sisi sendiri dan garis untuk sisi orang.
- **Kosong:** avatar 64px, nama 17px tebal, kalimat sapaan 15px, dan petunjuk tombol `kbd` (Enter kirim, Shift + Enter baris baru, tempel gambar bila lampiran aktif).
- **Menyambungkan ulang:** bilah Kertas di bawah header, muncul setelah 2.5 detik terputus, dengan tiga titik redup berdenyut dan janji bahwa pesan akan terkirim begitu tersambung. Tidak memakai mangga.

## Do's and Don'ts

### Do:
- **Do** memakai token warna semantik (`bg-accent`, `text-muted`, `border-line-strong`) supaya tema gelap ikut berubah tanpa varian `dark:`.
- **Do** memakai `accent-text`, bukan `accent`, untuk teks teal kecil di atas kertas.
- **Do** memberi Garis Batas (`line-strong`) pada setiap kolom isian dan kontrol bergaris, dan Garis Pisah (`line`) pada pemisah biasa.
- **Do** membuat target sentuh minimal 40px, dan 44px di kolom tulis.
- **Do** memakai ikon garis dari `Icon.tsx` untuk tombol.
- **Do** menghormati `prefers-reduced-motion`. Kabar yang dibawa gerak tetap harus terlihat saat animasinya berhenti.
- **Do** mengumpulkan tindakan atas sebuah pesan di menu panah pojok gelembung, dengan tindakan merusak paling akhir dan berwarna Bahaya.
- **Do** memberi elemen yang menggambar fokusnya sendiri kelas `cincin-sendiri`, lalu memindahkan cincinnya ke wadah (`focus-within`).
- **Do** memesan ruang untuk kontrol yang menumpang di dalam gelembung; kontrol tidak boleh menutupi teks, di perangkat mana pun.
- **Do** memasang wilayah status sebelum teksnya datang; wilayah yang lahir bersama teksnya sering dilewatkan pembaca layar.
- **Do** membedakan papan ketik dan jari untuk Enter: kirim di papan ketik, baris baru di layar sentuh.
- **Do** memakai `PillButton` untuk tombol pil, dan `format.ts` / `teks.ts` untuk waktu, cuplikan, dan kalimat yang dipakai di lebih dari satu tempat.

### Don't:
- **Don't** memakai mangga untuk apa pun selain panggilan atau sebutan, termasuk hiasan, tombol, atau sorotan.
- **Don't** memberi bayangan pada panel, header, gelembung, atau baris daftar.
- **Don't** memakai emoji sebagai lambang tombol.
- **Don't** memudarkan tombol utama dengan opacity saat nonaktif. Pakai bentuk bergaris Kertas dengan teks redup.
- **Don't** menampilkan kabar jaringan atau status sistem dengan mangga.
- **Don't** memakai bayangan berwarna sebagai pudar untuk menutupi teks di bawah kontrol.
- **Don't** menulis tindakan yang masih bisa diurungkan dalam bentuk lampau ("dihapus"); tulis yang sedang terjadi ("menghapus…").
- **Don't** memakai mangga untuk "@semua" di gelembung pengirimnya sendiri.
- **Don't** memakai opacity atau putih transparan untuk teks di dalam gelembung sendiri.
- **Don't** menumpuk lebih dari satu kabar di atas kolom tulis.
- **Don't** menyembunyikan satu-satunya jalan ke sebuah tindakan di balik hover.
- **Don't** menulis teks bacaan di bawah 15px, atau memakai keluarga huruf kedua.
- **Don't** menambah gerak yang tidak dipicu orang, kecuali gerak itu satu-satunya cara menyampaikan kabar (seperti tiga titik mengetik dan titik rekam).
- **Don't** menyalin palet gelap ke media query. Tema dipilih lewat `data-tema`, dan palet ditulis sekali saja.
