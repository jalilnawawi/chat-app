---
target: chatpanel
total_score: 27
max_score: 40
na_heuristics: 
p0_count: 1
p1_count: 2
target_identity: "file:/mnt/data/Coding/go/chat-app/web/src/components/ChatPanel.tsx"
target_fingerprint: "sha256:8f65e0357794b4d624768a048f4b30317e5c8ad91e4e834b54f5b9b154fdd46d"
target_path: /mnt/data/Coding/go/chat-app/web/src/components/ChatPanel.tsx
timestamp: 2026-09-17T04-52-42Z
slug: web-src-components-chatpanel-tsx
---
Method: dual-agent (A: review desain · B: detector + browser)

Catatan: chrome-devtools MCP tidak bisa jalan; A memakai Chromium headless dengan hover dipaksa aktif lewat blink-settings, B memakai Chromium terlihat lewat CDP. Lampiran mati di server ini — gelembung gambar dan suara dinilai dari kode.

## Design Health Score

| # | Heuristik | Skor | Masalah utama |
|---|---|---|---|
| 1 | Visibilitas status | 3 | Percakapan yang pertama dibuka mendarat di pesan TERLAMA, tanpa tanda ada yang lebih baru di bawah |
| 2 | Cocok dengan dunia nyata | 3 | "Pesan dihapus untuk semua orang." padahal masih menunggu |
| 3 | Kendali pengguna | 3 | Urungkan 5 detik tanpa tanda waktu; hanya hapus terakhir yang terlihat |
| 4 | Konsistensi | 3 | Aksi pesan terbelah: reaksi di luar gelembung, sisanya di panah di dalam |
| 5 | Pencegahan error | 3 | Area tekan 44px panah menumpuk di atas teks di layar sentuh |
| 6 | Mengenali, bukan mengingat | 2 | Di desktop semua aksi tak terlihat saat diam; lembar tidak menyebut pesan mana |
| 7 | Fleksibilitas | 2 | Tanpa pintasan, klik kanan, tekan lama, atau navigasi pesan lewat papan ketik |
| 8 | Estetika minimalis | 3 | Pudar bayangan panah menghapus huruf terakhir dan meluber di atas gelembung |
| 9 | Pemulihan error | 3 | Baris gagal sangat baik; coba ulang otomatis hanya saat WebSocket tersambung ulang |
| 10 | Bantuan | 2 | Petunjuk papan ketik hanya ada di keadaan kosong |
| **Total** | | **27/40** | **Good** |

## Design Specificity Verdict

LLM: spesifik, bukan templat — kanvas mint, teal untuk suara sendiri, cincin mangga hanya untuk sebutan, ekor gelembung, ikon garis sendiri, tema gelap yang disetel ulang. Titik lemahnya justru panah pojok yang baru: pudar bayangannya terbaca seperti kesalahan render, bukan detail yang dirancang.

Detector CLI: 0 temuan di 8 berkas.

Overlay browser (DM gelap, 4 pesan, reaksi, sematan; desktop 1400 dan mobile 390): 21 temuan per viewport, semuanya false positive.
- 19× `ai-color-palette` pada teal teks gelap `oklch(0.8 0.09 196)` — token yang disengaja; dihitung per span/svg/path dan termasuk teks `sr-only`.
- 1× `nested-cards` pada pil kolom tulis — komponen isian khas, bukan kartu bersarang.
- 1× `layout-transition` di `body` — tidak ada transition di sana.
Detector tidak menangkap satu pun masalah prioritas di bawah.

## Overall Impression

Naik dari 22 ke 27. Keadaan keandalan (gagal kirim, antrean tersimpan, bilah koneksi) kini kelas satu, dan mekanik menu solid. Yang menahan skor: percakapan dibuka di tempat yang salah, dan panah pojok yang baru menutupi teks serta tak punya cincin fokus yang terlihat di gelembung teal.

## What's Working

1. Keadaan keandalan: cincin gagal berjarak tanpa opacity, baris `role=alert` dengan sebab dan tombol 44px, antrean tersimpan, bilah koneksi tertunda 2.5 detik dan tidak mangga.
2. Mekanik menu: `role=menu`, fokus pindah ke butir pertama, panah/Home/End, Esc mengembalikan fokus, tindakan merusak terakhir dan terpisah; butir 44px, lembar 52px.
3. Disiplin sinyal: mangga hanya untuk sebutan, skeleton tanpa kilau, keadaan kosong menyebut penerima dan mengajarkan Enter/Shift+Enter, tepi gelembung gelap diperbaiki.

## Priority Issues

**[P0] Percakapan yang pertama dibuka mendarat di pesan terlama**
- Why: efek `activeId` menggulir sebelum riwayat datang; efek pertumbuhan (`ChatPanel.tsx:263-270`) hanya menggulir bila `nearBottom`, dan setelah isian pertama scrollTop masih 0. Terukur 0/691 desktop, 0/1028 mobile. Pesan terbaru di luar layar tanpa penanda — tugas inti "membaca yang baru" gagal.
- Fix: saat `lastCount.current === 0` (isian pertama setelah pindah ruang), selalu gulir ke bawah atau ke pesan belum dibaca pertama.
- Suggested command: /impeccable harden

**[P1] Panah pojok dan pudarnya menutupi teks**
- Why: saat hover di desktop, gelembung satu kata tertutup penuh ("Ok" tak terbaca, "Siap" jadi "Si"). Di layar sentuh, walau ada `pr-9`, bayangan `-6px 0 8px 2px` memudarkan huruf terakhir baris pertama dan meluber di atas tepi gelembung di kedua tema. Melanggar prinsip keterbacaan untuk pengguna lebih tua.
- Fix: buang bayangan; pesan sudutnya dengan spacer mengambang 28×20 sebagai anak pertama gelembung supaya hanya baris pertama yang membungkus — sama di semua perangkat, tanpa reflow saat hover.
- Suggested command: /impeccable polish

**[P1] Cincin fokus tak terlihat pada kontrol di dalam gelembung sendiri**
- Why: cincin global `var(--color-accent)` dengan jarak 2px (`index.css:150`) — di atas gelembung teal, cincin dan celahnya sama-sama teal (~1:1). Kena panah, tombol kutipan, pemutar suara. Gagal WCAG 2.4.7/1.4.11; panah yang opacity-0 hanya punya cincin itu sebagai tanda.
- Fix: aturan terbatas untuk gelembung sendiri, mis. `outline-color: var(--color-accent-ink)` di dalam gelembung teal.
- Suggested command: /impeccable harden, lalu /impeccable audit

**[P2] Urungkan hapus terlalu optimistis**
- Why: kalimatnya menyatakan sudah dihapus padahal masih menunggu; tanpa tanda waktu; 5 detik pendek untuk pengguna lebih tua; hapus kedua menyembunyikan urungkan pertama (`.at(-1)`); menutup tab dalam 5 detik diam-diam membatalkan hapus.
- Fix: "Menghapus untuk semua orang…" dengan bilah menyusut dan cuplikan pesan; gabungkan beberapa hapus ("2 pesan · Urungkan"); kirim hapus tertunda saat `pagehide` (keepalive); pertimbangkan 8–10 detik.
- Suggested command: /impeccable clarify + /impeccable harden

**[P2] Papan ketik dan pembaca layar mahal**
- Why: 27 tab stop untuk 12 pesan (reaksi + panah per pesan, plus kutipan); semua panah dan tombol reaksi berlabel sama (`MessageBubble.tsx:272`, `ReactionRow.tsx`); panah ada SEBELUM isi di DOM sehingga aksi dibacakan sebelum pesannya.
- Fix: label dengan penulis/cuplikan atau `aria-describedby`; pindahkan panah setelah isi di DOM; roving tabindex pada log; ↑ di kolom kosong untuk edit pesan terakhir.
- Suggested command: /impeccable audit

## Persona Red Flags

**Alex (power user):** tanpa pintasan (edit terakhir, balas, salin); tidak ada "Salin teks" di menu; dua kontrol per pesan; hover butuh bidikan tepat; tanpa menu klik kanan.

**Sam (keyboard / pembaca layar):** cincin panah pesan sendiri tak terlihat; label identik berulang; aksi dibacakan sebelum isi; ~2 tab stop per pesan tanpa roving focus. Positif: `role=log` + pengumuman sopan dan model keyboard menu.

**Casey (ponsel, satu tangan):** tanpa tekan lama atau geser untuk balas; panah di atas pesan panjang jauh dari ibu jari; area tekan 44px menumpuk di teks (memilih teks bisa membuka menu); header + bilah sematan ≈28% tinggi layar. Positif: lembar dalam jangkauan ibu jari.

**Pekerja kantor yang lebih tua:** di desktop tak ada tanda cara membalas sebelum hover; huruf terakhir yang pudar terlihat rusak; urungkan 5 detik tanpa penghitung padahal tulisannya sudah "dihapus"; ikon reaksi tanpa label; "Kelola sematan" terasa istilah teknis.

## Minor Observations

- Opacity untuk tombol nonaktif, melawan DESIGN.md: kirim pesan suara (`disabled:opacity-40`), VoicePlayer, tombol Simpan edit (`disabled:opacity-60`).
- Di bawah lantai 40px: "coba lagi" UploadStrip dan VoicePlayer (`min-h-8`), Batal/Simpan edit (`h-9`), reaksi cepat di ponsel (`size-9`).
- Baris gagal bisa muncul di bawah lipatan: gulir dipicu jumlah, bukan status.
- Bilah galat sematan sempit (`py-1`, 13px).
- Lembar tanpa judul/cuplikan pesan sasaran; "Batal" dibacakan sebagai butir menu.
- `title` panah ("Tindakan pesan") berbeda dari `aria-label`.
- Tema gelap mobile: lingkar teal di atas kanan gelembung sendiri (dari bayangan panah).

## Questions to Consider

- Balas adalah aksi paling sering, tapi tersembunyi di panah hover sementara reaksi dapat tombol permanen. Apakah Balas yang seharusnya jadi tombol samping?
- Apakah trik pudar bayangan sepadan, atau setiap gelembung cukup memesan sudutnya dan merelakan 28px?
- Lima detik urungkan disetel untuk pengguna tercepat atau terlambat — dan apakah "dihapus" benar sebelum server melakukannya?
