# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

Anggota tim di sebuah perusahaan yang memakai aplikasi ini sebagai chat kerja
internal: DM 1-on-1 dan grup per proyek atau per bagian. Rentang umurnya lebar —
termasuk pengguna yang lebih tua — sehingga keterbacaan bukan hiasan, melainkan
syarat. Dipakai setiap hari, di desktop maupun ponsel.

## Product Purpose

Chat realtime untuk tim yang dijalankan sendiri oleh perusahaannya. Tujuannya
penerapan sungguhan, bukan demo: berhasil berarti orang memakainya setiap hari
tanpa kehilangan pesan, tanpa pesan ganda, dan tanpa merasa aplikasinya
merepotkan.

## Positioning

Data milik perusahaan sendiri, dengan tumpukan yang ringan. Aplikasi utuh
berjalan hanya dengan satu binary Go dan PostgreSQL; Redis (multi-instance),
SeaweedFS (lampiran dan foto profil), Web Push, dan SMTP masing-masing bisa
dimatikan lewat satu variabel lingkungan, dan fitur yang bergantung padanya
hilang dengan rapi dari UI alih-alih rusak. Slack atau WhatsApp tidak bisa
menawarkan kepemilikan data dan kesederhanaan operasi ini sekaligus.

## Operating Context

- Di-host sendiri (self-hosted) di infrastruktur perusahaan; tim IT perusahaan
  yang menjalankannya. Catatan operasional di `docs/scaling.md`.
- Sudah diuji untuk 1000 koneksi WebSocket bersamaan dan rolling deploy
  multi-instance.
- Dibuka di browser desktop dan ponsel; layar sempit menampilkan daftar dan
  percakapan bergantian dalam satu kolom.
- Notifikasi lewat Web Push hanya untuk yang sedang offline.

## Capabilities and Constraints

- DM dan grup memakai satu abstraksi `conversation`.
- Presence, typing indicator, read receipt, edit dan hapus (soft delete), balas,
  sebut (@), reaksi emoji, lampiran berkas dengan thumbnail, pesan suara,
  pencarian isi pesan, teruskan, dan sematkan.
- Kelola grup: judul, anggota, keluar, pindah pemilik.
- Kelola akun: foto profil, email terverifikasi, ganti dan pulihkan password,
  daftar perangkat, status.
- Sesi memakai cookie httpOnly; frontend dan API dilayani dari satu origin.
- Fitur opsional (lampiran, foto profil, notifikasi, email) harus bisa absen
  tanpa meninggalkan tombol mati di UI.
- Bahasa UI: Indonesia. Terjemahan belum ada, tetapi teks harus disusun supaya
  bisa diterjemahkan nanti (i18n direncanakan).
- Belum ada integrasi pihak ketiga: tidak ada aplikasi, bot, token mesin, atau
  mini-app. Bentuk sistemnya sudah diputuskan di `docs/integrasi.md` — sebuah
  aplikasi adalah baris `users`, permukaannya halaman penuh di kolom utama dan
  bukan percakapan dengan bot, dan bawaannya dia hanya menerima yang ditujukan
  kepadanya — tetapi pekerjaannya berada di belakang daftar rilis di `TASKLIST.md`.
- Terbuka: model pendaftaran akun untuk perusahaan (undangan, SSO, atau daftar
  bebas) belum diputuskan. Jawabannya juga yang menentukan siapa berwenang
  memasang aplikasi pihak ketiga.

## Brand Commitments

- Nama kerja: "Chat App". Belum ada nama produk atau logo final selain
  `web/public/icon.png`.
- Istilah dan dokumentasi ditulis dalam Bahasa Indonesia yang lugas.

## Evidence on Hand

- Hasil uji beban 1000 koneksi (`server/cmd/loadtest`, `docs/scaling.md`).
- Angka terukur di `docs/` (mis. thumbnail 6,2 MB jadi 13 KB; perbandingan
  index pencarian pada sejuta pesan).
- Belum ada pelanggan, testimoni, atau studi kasus. Jangan dikarang.

## Product Principles

1. Pesan tidak boleh hilang atau ganda; keandalan mendahului fitur.
2. Setiap bagian infrastruktur tambahan bersifat opsional, dan ketiadaannya
   terlihat rapi, bukan rusak.
3. Mudah dipakai semua umur: teks yang harus dikira-kira adalah teks yang gagal.
4. Satu isyarat, satu arti — warna atau tanda yang dipakai untuk dua hal
   berhenti menjadi isyarat.
5. Keputusan yang mahal ditambahkan belakangan dikerjakan di awal.

## Accessibility & Inclusion

- Target WCAG 2.2 AA: kontras, operasi keyboard penuh, fokus yang terlihat.
- Pengguna lebih tua: ukuran huruf yang nyaman dibaca, kontras dihitung sebelum
  warna dipilih, target sentuh yang cukup besar.
