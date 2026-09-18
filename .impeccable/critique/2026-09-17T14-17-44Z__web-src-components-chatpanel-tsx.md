---
target: chatpanel
total_score: 28
max_score: 40
na_heuristics: 
p0_count: 0
p1_count: 2
target_identity: "file:/mnt/data/Coding/go/chat-app/web/src/components/ChatPanel.tsx"
target_fingerprint: "sha256:184fbbe1833e8b6cb7fd16538062511a3e9c8d34486100d33d1ca94a918f6c39"
target_path: /mnt/data/Coding/go/chat-app/web/src/components/ChatPanel.tsx
timestamp: 2026-09-17T14-17-44Z
slug: web-src-components-chatpanel-tsx
---
Method: dual-agent (A: review desain · B: detector + browser)

Catatan: chrome-devtools MCP tidak bisa jalan; A memakai Chromium headless dengan hover dipaksa aktif dan mikrofon palsu, B memakai Chromium terlihat lewat CDP. A tanpa sengaja menyematkan lalu melepas satu pesan saat uji keyboard — data DM bertambah dua catatan sistem, dua pesan dari Budi, dan dua pesan terhapus.

## Design Health Score

| # | Heuristik | Skor | Masalah utama |
|---|---|---|---|
| 1 | Visibilitas status | 3 | Selama jeda urungkan gelembung sudah bertulisan "Pesan ini dihapus" sementara kabar berbunyi "Menghapus…" |
| 2 | Cocok dengan dunia nyata | 3 | "7 dtk", "bangunkan semua anggota", centang tanpa kata |
| 3 | Kendali pengguna | 3 | Sematkan/lepas langsung berlaku, membuat catatan sistem untuk semua, tanpa urungkan |
| 4 | Konsistensi | 3 | Petunjuk "Shift+Tab ke riwayat" — Shift+Tab pertama mendarat di tombol emoji |
| 5 | Pencegahan error | 3 | @semua dan buang rekaman terjaga; sematkan tidak |
| 6 | Mengenali, bukan mengingat | 2 | Di desktop panah menu dan tombol reaksi tak terlihat sebelum hover |
| 7 | Fleksibilitas | 3 | Tanpa pintasan balas, lompat ke belum dibaca, atau cari emoji |
| 8 | Estetika minimalis | 3 | Pesan terhapus dan catatan sematan masing-masing sebaris penuh bercap waktu |
| 9 | Pemulihan error | 3 | Pesan suara gagal hanya "Tidak bisa diputar", tanpa sebab dan tanpa pengumuman |
| 10 | Bantuan | 2 | Petunjuk pintasan hanya saat kolom tulis difokus di layar ≥64rem |
| **Total** | | **28/40** | **Good** |

## Design Specificity Verdict

LLM: produk yang teliti dan sadar aksesibilitas, bukan templat — pemindai kontras tidak menemukan kegagalan AA di terang/gelap, desktop/mobile; label baris kaya; hapus dengan urungkan bekerja sesuai rancangan. Tapi identitas "Pagi di Tepi Air" baru setengah terlihat: tanpa sebutan mangga, layar terbaca "WhatsApp Web di atas mint". Kepala, bilah sematan, dan kolom tulis adalah pita putih polos; ekor gelembung satu-satunya bentuk khas. Disiplin, tapi belum berkesan.

Batas kode: pembagian baik (ChatPanel mengatur; MessageParts, NoticeStack, PillButton, teks.ts, format.ts). Utang: belum ada IconButton bersama (kelas yang sama berulang di ChatHeader, ChatPanel, Composer); pil "Muat pesan lama/berikutnya" belum memakai PillButton; dua baris reaksi cepat (ReactionRow, MessageMenu); teks JSX yang masih tertanam; sudut `[7px]` belum jadi token; ChatPanel 706 baris dengan ±30 selector.

Detector CLI: 0 temuan di 18 berkas.

Overlay browser (DM gelap): 70 temuan desktop, 77 mobile — semuanya false positive.
- `ai-color-palette` (68/76): token teal teks gelap yang disengaja; termasuk span sr-only dan path svg.
- `text-overflow` di cuplikan PinBar: `truncate` disengaja.
- `clipped-overflow-container` di cangkang App.tsx: tidak ada yang benar-benar terpotong.
- Tidak ada `text-occlusion`.

## Overall Impression

Turun dari 30 ke 28. Penyebabnya bukan kemunduran, melainkan penilaian yang lebih tajam: dua janji yang tidak ditepati (petunjuk Shift+Tab, kolom tulis yang tidak tumbuh) dan tindakan desktop yang masih tersembunyi. Fondasi aksesibilitas dan jalur urungkan kini jadi kekuatan utama.

## What's Working

1. Hapus dengan urungkan (terverifikasi): fokus ke Urungkan, pengumuman "Urungkan tersedia selama 8 detik", beberapa hapus digabung, garis waktu memendek.
2. Struktur pembaca layar dan papan ketik (terverifikasi): label baris dengan jumlah reaksi/lampiran, pengumuman pesan baru, pil "1 pesan baru", kirim sendiri kembali ke bawah.
3. Layar sentuh: lembar tindakan dengan cuplikan dan baris reaksi 48px, Enter jadi baris baru, chip 40px. Kontras lolos di mana-mana — teks gelembung sendiri tepat di lantai (4.58/4.64:1).

## Priority Issues

**[P1] Kolom tulis tidak tumbuh mengikuti teks**
- Why: textarea utama tetap 44px (`rows={1}`, `max-h-36` tanpa penyesuaian tinggi; Composer.tsx:476, 501). Hanya form sunting yang tumbuh. Di ponsel baris pertama pesan dua baris tergulir keluar.
- Fix: pakai efek ukur yang sama dengan EditForm (atau `field-sizing: content` dengan cadangan), batas ±6 baris.
- Suggested command: /impeccable harden

**[P1] Shift+Tab dari kolom tulis tidak sampai ke riwayat, padahal petunjuknya berkata begitu**
- Why: tombol emoji berada sebelum textarea di dalam pil, jadi Shift+Tab pertama mendarat di "Pilih emoji" (terverifikasi). Petunjuk (Composer.tsx:584) dan DESIGN.md menjanjikan sebaliknya.
- Fix: pindahkan tombol emoji ke setelah textarea (di samping lampiran), atau ubah petunjuk dan DESIGN.md, atau tambah Esc/Alt+↑ untuk lompat ke riwayat.
- Suggested command: /impeccable harden

**[P2] Tindakan pesan tersembunyi sampai hover di desktop, dan dua pemicunya tidak sepakat**
- Why: panah 28px muncul saat gelembung di-hover (`group-hover/bubble`), tombol reaksi saat baris di-hover (`group-hover`) — kursor di selokan menampilkan satu tapi tidak yang lain. Klik kanan hanya di tooltip. Pekerja kantor yang lebih tua tidak akan menemukan Balas atau Hapus.
- Fix: tampilkan panah saat diam dengan nada redup (setidaknya di baris terpilih/terakhir), satukan cakupan hover kedua kontrol.
- Suggested command: /impeccable clarify, lalu /impeccable polish

**[P2] Hapus yang tertunda sudah terbaca selesai; sematkan tanpa urungkan**
- Why: selama 8 detik gelembung berbunyi "Pesan ini dihapus" sementara kabar berbunyi "Menghapus…" — melanggar aturan DESIGN sendiri tentang bentuk lampau. Sematkan/lepas langsung disiarkan sebagai catatan sistem tanpa Urungkan (terpicu tak sengaja oleh Enter di menu).
- Fix: tampilkan keadaan tertunda sebagai "Menghapus…" dengan garis putus-putus; lewatkan sematkan/lepas lewat kabar singkat dengan Urungkan, atau tunda catatan sistemnya.
- Suggested command: /impeccable clarify + /impeccable harden

**[P3] Identitas lemah dan lebar terbuang di layar lebar**
- Why: di 1400px gelembung sendiri dan orang lain terpisah ±1000px; "pagi di tepi air" hanya hidup di token warna.
- Fix: batasi kolom baca (mis. `max-w-[56rem] mx-auto`); beri satu tanda khas yang tertahan — penanda hari, keadaan kosong, atau bilah sematan — tanpa menambah warna.
- Suggested command: /impeccable layout, lalu /impeccable bolder (ringan)

## Persona Red Flags

**Alex (power user):** tanpa pintasan balas/reaksi; tanpa lompat ke belum dibaca; pemilih emoji tanpa cari; menu ±15 pilihan.

**Sam (keyboard / pembaca layar):** jalan memutar Shift+Tab; setelah pesan baru datang, pemberhentian roving tetap di baris terakhir yang disentuh, bukan yang terbaru; pesan suara gagal tidak diumumkan; daftar sematan tidak memindahkan fokus dan Esc tidak mengembalikannya; menyalin dua kali memberi teks pengumuman yang sama sehingga tidak dibacakan ulang.

**Casey (ponsel, satu tangan):** kolom tulis tidak tumbuh; panah permanen di setiap gelembung; sel emoji kolom tulis ±38px; Rekam dan Kirim hanya ikon di ponsel.

**Pekerja kantor yang lebih tua:** tindakan tersembunyi di hover; centang tanpa kata; "dtk"; jam 11.5px; "Pesan ini dihapus" saat urungkan masih mungkin; teks gelembung sendiri tepat di lantai AA.

## Minor Observations

- Lipatan terhapus dan pesan terhapus tunggal masing-masing membawa baris jam; catatan "lepas sematan" tak bisa ditekan dan mengotori DM.
- Chip reaksi 32px dengan tetikus, di bawah aturan 40px DESIGN.
- Teks alt gambar adalah nama berkas.
- Belum pasti, terlihat sekali: Tab dari baris "2 pesan dihapus" pindah ke baris berikutnya, lalu Shift+Tab keluar ke bilah sematan (useRovingLog.ts:33-64).
- Elemen `<audio>` di riwayat melaporkan tabIndex 0.
- Baris petunjuk 13px bisa membungkus di 1024–1100px.

## Questions to Consider

- Kalau mangga hanya untuk sebutan, di mana "pagi" tinggal? Di waktu (penanda hari, keadaan kosong) alih-alih di warna?
- Apakah hal yang bukan kejadian (pesan terhapus, catatan sematan) pantas mendapat ruang permanen di DM dua orang?
- Kenapa satu-satunya jalan ke Balas disembunyikan di balik hover, padahal banyak pengguna inti belum pernah mendengar klik kanan?
