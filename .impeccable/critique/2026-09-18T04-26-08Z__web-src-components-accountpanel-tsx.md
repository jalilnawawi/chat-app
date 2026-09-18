---
target: akun panel — profil vs privasi akun vs general
total_score: 24
max_score: 40
na_heuristics: 
p0_count: 1
p1_count: 4
target_identity: "file:/mnt/data/Coding/go/chat-app/web/src/components/AccountPanel.tsx"
target_fingerprint: "sha256:5c03563badf37f1b65c0fb27a0e6044e936410e475d3f609031a60cd53cfbbfe"
target_path: /mnt/data/Coding/go/chat-app/web/src/components/AccountPanel.tsx
timestamp: 2026-09-18T04-26-08Z
slug: web-src-components-accountpanel-tsx
---
**Method: dual-agent (A: af8b3e73d9fde5069 · B: acb515ec70b3187f6)**

# Kritik — web/src/components/AccountPanel.tsx (mode Operate)

## Skor Kesehatan Desain

| # | Heuristik | Skor | Masalah utama |
|---|---|---|---|
| 1 | Visibilitas status sistem | 2 | Nama simpan diam-diam saat blur (:154), nol konfirmasi. Progres unggah cuma angka dalam tombol `disabled`. Tak ada live region. |
| 2 | Cocok dunia nyata | 3 | Bahasa lugas dan tepat. Tapi "Tampilan" (setelan per-perangkat) duduk di panel berjudul "Akun kamu". |
| 3 | Kendali dan kebebasan | 2 | Overlay layar penuh (:23) tanpa Escape, tanpa role="dialog", tanpa kembalikan fokus. Cabut perangkat dan ganti password: instan, tanpa undo. |
| 4 | Konsistensi dan standar | 2 | aria-pressed ada di Tampilan, hilang di Status — visual terpilih sama persis. IconButton/PillButton sudah ada di design system, panel ini tulis tombol sendiri. |
| 5 | Pencegahan kesalahan | 2 | Tak ada satu pun <form>: Enter tidak submit, tak ada konfirmasi password baru, tak ada show-password, tak ada konfirmasi sebelum keluarkan semua perangkat. |
| 6 | Kenali bukan ingat | 2 | Enam bilah identik, nol judul grup, nol navigasi. Kamu harus INGAT password ada di bawah pemilih tema. |
| 7 | Fleksibilitas dan efisiensi | 2 | Nol autocomplete — pengelola password tak bisa isi apa pun. Tak ada "keluarkan semua perangkat". |
| 8 | Estetika dan minimalis | 3 | Tenang dan token dipakai benar. Tapi ini minimalisme yang membuang STRUKTUR, bukan kebisingan. |
| 9 | Pemulihan kesalahan | 3 | Pesan spesifik dan manusiawi, menempel di kontrolnya. Tapi <p> biasa tanpa role="alert". |
| 10 | Bantuan dan dokumentasi | 3 | Bagian terkuat panel: akibat dijelaskan di titik kejadian. |
| **Total** | | **24/40** | **Perlu kerja struktural** |

## Verdict Kekhasan Desain

Tulisannya khas produk ini. Komposisinya bisa dipakai produk mana pun.

Yang tak tergantikan: degradasi sadar-kapabilitas (:104-125, :400-421) — fitur yang server tak punya diganti kalimat penjelas, bukan dihilangkan diam-diam, dan email yang sudah tercatat tetap tampil meski SMTP mati. Lalu kalimat-akibat-dulu: "Username tidak bisa diganti", "Sebelum terverifikasi, alamat ini tidak bisa dipakai memulihkan akun".

Yang generik: TATA LETAKNYA. Enam <section> sejajar, semua `border-b border-line px-4 py-4`, semua berkepala `text-[13px] font-semibold text-muted` yang identik. Bentuk baku setiap panel setelan Tailwind sejak 2015. Docstring di :9-16 mengumumkan tesis komposisi ("yang mengubah kredensial diletakkan di bawah, terpisah dari yang cuma mengubah tampilan") yang pikselnya TIDAK kerjakan — TampilanSection dirender persis di antara Status dan Email (:38).

Pemindaian deterministik: `impeccable detect` jalan normal, exit 0, NOL temuan — baik di file target maupun seluruh web/src/components/. Semua temuan di bawah berasal dari pengukuran statik. Nol kandidat false positive.

Bukti browser: tidak ada. Chrome tidak terpasang di mesin ini (`Could not find Google Chrome executable for channel 'stable' at: /opt/google/chrome/chrome`); hanya Brave yang ada. Injeksi overlay dilewati — tidak ada overlay di browser user.

Koreksi lintas-asesmen: dugaan bahwa `outline-none` membuang cincin fokus TERBUKTI SALAH. Aturan :focus-visible di index.css:150-157 tidak berlapis, jadi menang atas utilitas @layer. Keenam input tetap dapat cincin 2px.

## Kesan Keseluruhan

Panel ini ditulis oleh orang yang berpikir, lalu ditata oleh template. Keluhan user tepat dan bisa diukur: enam batas visual dengan bobot identik berarti tak satu pun batas membawa informasi. Peluang terbesar: tiga grup bernama, bukan enam bilah anonim.

## Yang Sudah Bagus

1. Degradasi yang menjelaskan diri (:104-125). Tombol yang selalu tampil tapi dijawab "tidak aktif di server ini" mengajari orang mengabaikan pesan kesalahan — dan pelajaran itu terbawa ke kesalahan yang penting.
2. Akibat dikatakan sebelum tindakan (:165, :282, :487, :554). Hal paling khas produk di permukaan ini. Pertahankan verbatim dalam restrukturisasi apa pun.
3. Dua mekanika yang benar-benar dipikirkan: tema tiga-pilihan (bukan saklar dua posisi), dan DURASI yang menghitung instan absolut di zona waktu pembaca dengan fallback lewat-tengah-malam (:187-201).

## Masalah Prioritas

### [P1] Enam bagian sejajar, nol grup — keluhan utama user

Apa. :35-42 merender enam section dengan chrome identik byte-per-byte. Urutan: Profil → Status → Tampilan → Email → Password → Perangkat. Satu setelan kosmetik duduk persis di antara setelan sosial dan setelan kredensial.

Kenapa penting. Di ponsel 360x640 ini ±1200px gulir tanpa penanda. "Ganti password" tak bisa ditemukan tanpa menggulir habis. Tema dan password terasa sama pentingnya karena digambar sama persis.

Perbaikan. Tiga grup bernama, pakai bahasa kedalaman yang sudah ada di DESIGN.md (kertas di atas kanvas + garis 1px):
- Pita judul grup: `bg-canvas px-4 py-2 text-[13px] font-bold text-ink border-y border-line`; section tetap bg-surface.
- "Profil" → ProfileSection + StatusSection.
- "Preferensi di perangkat ini" → TampilanSection + tombol notifikasi push yang terdampar sebagai ikon lonceng tanpa label di Sidebar.tsx:159-177.
- "Keamanan akun" → Email + Password + Perangkat, plus "Keluar dari akun" dipindah dari Sidebar.tsx:178-186 ke <footer className="border-t border-line px-4 py-3"> — pola yang sudah dipakai GroupPanel.tsx:225-242.
Pindahkan kalimat akibat ganti password (:554) ke ATAS tombolnya (:543).

Perintah: /impeccable layout

### [P0] Badge "belum terverifikasi" tak terlihat di mode gelap — 1,20:1

Apa. :433 memasangkan bg-call-soft dengan text-call-ink. Di blok :root[data-tema='gelap'], --color-call-soft dibalik jadi coklat gelap (index.css:111) tapi --color-call-ink di-redeclare IDENTIK dengan tema terang (index.css:112). Terhitung: terang 8,73:1, gelap 1,20:1. Pasangan ini satu-satunya di seluruh web/src/.

Kenapa penting. Badge ini satu-satunya penanda keadaan yang menentukan apakah seseorang bisa memulihkan akunnya. Target WCAG 2.2 AA, pengguna lebih tua disebut eksplisit di PRODUCT.md.

Tambahan: mangga dipakai untuk arti ketiga di sini; DESIGN.md menetapkan mangga hanya untuk panggilan/sebutan.

Perbaikan. Pakai bg-danger-soft/text-danger (lolos AA di kedua tema). Perbaiki --color-call-ink di blok gelap, atau hapus pasangan call-soft+call-ink.

Perintah: /impeccable audit

### [P1] Tombol "Cabut" tak terlihat bagi pemakai keyboard

Apa. :613 `opacity-0 … group-hover:opacity-100 [@media(hover:none)]:opacity-100`. Fallback sentuh ADA. Fallback keyboard TIDAK ADA.

Kenapa penting. Di perangkat ber-hover, pemakai keyboard yang Tab mendarat di elemen opacity: 0 — tetap fokusabel, tetap bisa ditekan Enter, tapi tak terlihat sama sekali, termasuk cincin fokusnya. Kontrol ini menghapus sesi login.

Penyimpangan panel-spesifik: MessageMenu.tsx:265 dan ReactionRow.tsx:135 sudah punya focus-visible:opacity-100. GroupPanel.tsx:193 punya bug yang sama.

Perbaikan. Tambah focus-visible:opacity-100 di kedua tempat, atau tampilkan selalu.

Perintah: /impeccable audit

### [P1] Lima input berlabel placeholder, nol autocomplete, nol live region

- 1.3.1 — baris 243, 439, 451, 526, 534 hanya berbekal placeholder. Judul section di atasnya <p>, bukan <label>, tanpa aria-labelledby. Pola benar sudah ada di :144-160.
- Kontras placeholder di tema terang: 2,97:1 — label satu-satunya pun gagal dibaca.
- 1.3.5 — grep -c autocomplete = 0. Tiga field password dan satu email. AuthPage.tsx:117-147 melakukannya dengan benar.
- 4.1.3 — tujuh paragraf note/error (:167, :286, :491, :492, :558, :559, :622) tanpa role="status"/role="alert". ChatPanel, RecordingBar, DeleteNotice semuanya patuh.
- Semantik dialog — di <md panel overlay layar penuh (:23) tanpa role="dialog", aria-modal, Escape, pemindahan fokus, focus trap. ForwardDialog dan NewChatDialog sudah punya semuanya.
- Status (:228) dan pill Durasi (:259) menandai pilihan HANYA lewat warna, tanpa aria-pressed, padahal tombol Tampilan di file yang sama punya.

Perintah: /impeccable harden

### [P1] Dua keadaan yang ditampilkan panel ini salah

- :208 useState(0) — tak pernah diturunkan dari me.statusExpiresAt. Buka lagi panel saat status berakhir 13.00: pill menyorot "Tanpa batas", kalimat di bawahnya (:276) berbunyi "Berlaku sampai 13.00". Menekan tombol status setelah buka ulang MENGHAPUS DIAM-DIAM batas waktu yang orang itu pasang.
- :576-578 — SessionSection memuat sekali saat mount. Setelah ganti password berhasil dan mengumumkan "Semua perangkat lain sudah dikeluarkan" (:514), daftar perangkat di atasnya masih menampilkan perangkat itu sebagai aktif.

Perbaikan. Turunkan indeks durasi awal dari me.statusExpiresAt. Angkat pemuatan sesi ke store dan picu ulang setelah ganti password. Tambah "Keluarkan semua perangkat lain".

Perintah: /impeccable harden

## Beban Kognitif — 6 gagal dari 8

Gagal: fokus tunggal, pengelompokan, hierarki visual, satu-hal-sekaligus, ≤4 pilihan per titik keputusan, memori kerja. Sebagian gagal: chunking. Lolos: pengungkapan bertahap.

Titik keputusan dengan >4 pilihan:
- StatusSection (:222-288) — 8 kontrol untuk satu keputusan: 3 tombol status + 1 kolom teks + 4 pill durasi, tanpa label yang menjelaskan bahwa pill mengubah tombol.
- Tingkat panel — 6 section sejajar.
- SessionSection — tak terbatas, tiap baris membawa tombol destruktif.
- Panel diam: ±20 kontrol interaktif serentak di kolom 320px.

## Perjalanan Emosi

Puncak: kalimat-kalimat terus terang. Produk hampir tak pernah memberi tahu batasannya sebelum kamu mencari tombolnya.

Lembah 1 — pusing di tengah gulir. Pasang "Sibuk" (sosial). Satu sentil: terang/gelap (kosmetik). Satu sentil lagi: ketik password (keamanan). Tiga mode mental dalam tiga sentil, tanpa judul "Keamanan".

Lembah 2 — email belum terverifikasi. Badge 11px yang di mode gelap 1,20:1. Kalimat penjelasnya dicetak dengan 12px muted yang sama persis dengan "Berlaku di perangkat ini saja".

Akhir — bahu terangkat. Hal terakhir di layar: daftar perangkat yang aksinya tak terlihat, lalu tak ada apa-apa. Tak ada "Keluar".

## Bendera Merah Persona

Pemakai keyboard / pembaca layar. Tab ke "Cabut" dan tidak melihat apa pun. Mendengar satu heading di seluruh panel. Salah ketik password saat ini, tekan tombol, tidak diberi tahu apa pun. Di ponsel, fokus tertinggal di belakang overlay dan Escape tak berfungsi.

Ibu Sri, 54, admin kantor, Android, pakai tiap hari (PRODUCT.md). Buka "Akun kamu" untuk ganti foto; hal pertama di bawah namanya dua tombol setinggi 33px. Gulir, ketemu "Tampilan" berikon bulan, berhenti. Pasang "Sibuk sampai akhir hari" jam 09.00; buka lagi jam 14.00: pill bilang "Tanpa batas", kalimat di bawah bilang "Berlaku sampai 23.59". Dia tekan "Sibuk" lagi dan menghapus batas waktunya sendiri.

Pekerja yang bergantung pengelola password. Nol field bisa diisi. Enter di kolom password baru tidak berbuat apa-apa. Ingin keluar dari semua perangkat: satu-satunya jalan mengganti password, dan daftar perangkat tidak menyegar.

## Observasi Kecil

- Sembilan kelompok kontrol di bawah minimum 40px DESIGN.md: tutup 36px, "Ganti foto" 33,5px, "Hapus" 31,5px, status 37,5px, durasi 32px, "Simpan email" 35,5px, "Kirim ulang" 37,5px, "Ganti password" 35,5px, "Cabut" 26px. Lolos: input teks 44,5px, tombol Tampilan 64px. (Semua lolos WCAG 2.5.8 24px; yang dilanggar patokan proyek sendiri.)
- IconButton dan PillButton sudah ada dan tepat untuk tombol tutup serta dua tombol solid. disabled:opacity-50 di :466/:546 juga Don't menurut DESIGN.md.
- Tooltip sidebar "Kelola akun kamu" vs header panel "Akun kamu".
- Tiga <aside> tanpa aria-label.
- Di tepat 768px: Sidebar 320 + panel 320 menyisakan ±128px untuk percakapan.
- Tak ada kolom konfirmasi password baru dan tak ada show-password.
- SessionSection tanpa empty state.
- Baris perangkat menyebut peramban tapi tak pernah lokasi atau IP.

## Pertanyaan untuk Dipikirkan

1. Docstring :9-16 bilang kredensial dipisah dari yang cuma mengubah tampilan. Baris :38 menaruh pemilih tema persis di antaranya. Mana yang bohong?
2. Tema dan notifikasi push sama-sama per-perangkat. Kenapa satu di "Akun kamu" dan satu ikon lonceng tanpa label di sidebar?
3. "Keluar" ikon 20px tanpa label di pojok sidebar; "Ganti foto" tombol berteks. Mana yang akibatnya lebih besar?
4. Kalau StatusSection dipindah ke avatar sidebar, apa yang tersisa di "Akun kamu"?

## Jawaban user (arah yang dipilih)

1. Komentar DAN tata letak dua-duanya belum benar; UI-nya memang belum dibahas detail.
2. Notifikasi push dan Tema: pisahkan dari panel akun.
3. "Keluar": pakai ikon, jangan di atas; taruh di panel baru yang memuat privasi akun.
4. Pecah jadi ProfilePanel dan AkunPanel.
