---
target: chatpanel
total_score: 29
max_score: 40
na_heuristics: 
p0_count: 0
p1_count: 1
target_identity: "file:/mnt/data/Coding/go/chat-app/web/src/components/ChatPanel.tsx"
target_fingerprint: "sha256:767f3937647c49f539566d364817523d561236bb43a8d8926ec48a590ecd4a71"
target_path: /mnt/data/Coding/go/chat-app/web/src/components/ChatPanel.tsx
timestamp: 2026-09-17T06-28-39Z
slug: web-src-components-chatpanel-tsx
---
Method: dual-agent (A: review desain · B: detector + browser)

Catatan: chrome-devtools MCP tidak bisa jalan; A memakai Chromium headless dengan hover dipaksa aktif, B memakai Chromium terlihat lewat CDP. Server uji berjalan tanpa lampiran, jadi rekam suara tidak terlihat langsung.

## Design Health Score

| # | Heuristik | Skor | Masalah utama |
|---|---|---|---|
| 1 | Visibilitas status | 3 | Kabar singkat dan kabar hapus baru dipasang bersama teksnya — pembaca layar bisa melewatkannya |
| 2 | Cocok dengan dunia nyata | 3 | Keadaan kosong di ponsel masih mengajarkan Shift+Enter |
| 3 | Kendali pengguna | 3 | Pindah ruang selama jeda hapus menyembunyikan Urungkan, hapusnya tetap jalan |
| 4 | Konsistensi | 3 | Kabar singkat selalu bercentang hijau, termasuk untuk kegagalan |
| 5 | Pencegahan error | 3 | Kolom sunting hanya tumbuh pada baris baru; pesan panjang disunting dalam satu baris terpotong |
| 6 | Mengenali, bukan mengingat | 3 | Di desktop tindakan pesan tak terlihat sebelum hover |
| 7 | Fleksibilitas | 3 | Tanpa tombol tunggal (balas/reaksi) dari pesan terfokus; pemilih emoji tanpa pencarian |
| 8 | Estetika minimalis | 3 | Deretan pesan terhapus masing-masing jadi gelembung putus-putus penuh |
| 9 | Pemulihan error | 3 | Hapus yang gagal hanya menawarkan "Tutup", tanpa coba lagi |
| 10 | Bantuan | 2 | Pintasan hanya di placeholder (hilang saat mengetik), title, dan teks sr-only |
| **Total** | | **29/40** | **Good** |

## Design Specificity Verdict

LLM: spesifik dan disengaja — ekor gelembung yang merapat dalam rentetan, cincin mangga untuk sebutan, urungkan alih-alih konfirmasi, tema gelap yang menjaga peran warna, lantai 15px. Tenang dan setia pada "Pagi di Tepi Air". Kelemahan tersisa ada di detail fokus papan ketik, asumsi desktop yang terbawa ke layar sentuh, dan satu kegagalan kontras.

Detector CLI: 0 temuan di 8 berkas.

Overlay browser (DM gelap, 4 pesan, reaksi, sematan): 21 temuan desktop, 20 mobile.
- `ai-color-palette` (17–18): false positive — token teal teks gelap yang disengaja; ikon dihitung per span/svg/path dan termasuk teks sr-only.
- `nested-cards` di pil kolom tulis: kemungkinan false positive — induknya bilah datar bergaris atas, bukan kartu.
- `layout-transition` di `body`: kemungkinan berasal dari kelas `transition-[width]` bilah kemajuan unggahan (UploadStrip.tsx:107); nyata tapi pendek dan hanya saat mengunggah.
Detector tidak menangkap satu pun masalah prioritas di bawah.

## Overall Impression

Naik dari 27 ke 29. Model papan ketik riwayat, urungkan hapus, dan jalur gagal kini matang. Yang tersisa bukan masalah bentuk, melainkan tepi: fokus setelah hapus, kebocoran roving focus, Enter di layar sentuh, dan kontras penanda hari.

## What's Working

1. Disiplin sinyal: cincin sebutan, cincin gagal berjarak, tombol kirim nonaktif bergaris. Kontras terukur: gelembung orang 14.5, nama penulis grup 4.96/10.33, chip sebutan 5.01/5.37.
2. Jalur gagal dirancang: baris gagal dengan "Tulis ulang"/"Kirim ulang" 44px, urungkan hapus, dua ketukan untuk membuang rekaman panjang, bilah koneksi tertunda 2.5 detik.
3. Model papan ketik: satu tab stop untuk riwayat; ↑/↓/Home/End; Enter membuka menu dan fokus ke "Balas"; Esc kembali ke pemicu; ↑ di kolom kosong menyunting; lembar mobile 52px dengan cuplikan dan fokus terkunci.

## Priority Issues

**[P1] Fokus hilang setelah "Hapus untuk semua"**
- Why: `select()` memanggil `close(true)` untuk memfokus pemicu (MessageMenu.tsx:166-168), tapi pesan kini terhapus, `menuItems` kosong, dan pemicunya lenyap — fokus jatuh ke `body`. Pengguna pembaca layar kehilangan posisi, dan Urungkan hanya tercapai kebetulan.
- Fix: setelah `scheduleDelete`, pindahkan fokus ke tombol Urungkan (atau ke baris pesannya); pasang wilayah status kabar hapus secara permanen; pertimbangkan menjeda hitung mundur selama kabar di-hover/difokus (WCAG 2.2.1).
- Suggested command: /impeccable harden

**[P2] Roving focus bocor dan pemilih emoji jadi perangkap Tab**
- Why: 7 tab stop di dalam riwayat DM pendek — chip reaksi (ReactionRow.tsx:36), tombol catatan sematan (MessageBubble.tsx:126), tautan lampiran, dan kontrol pemutar suara mengabaikan `tabbable`; jumlahnya tumbuh dengan panjang riwayat. Pemilih emoji berisi ~520 tombol tanpa navigasi panah.
- Fix: teruskan `tabbable` ke semua kontrol di dalam pesan; kisi emoji dengan roving tabindex dan panah, plus kolom cari.
- Suggested command: /impeccable harden

**[P2] Perilaku Enter desktop di layar sentuh**
- Why: Enter selalu mengirim (ChatPanel.tsx:691) sehingga pesan multi-baris mustahil di ponsel; keadaan kosong di ponsel mengajarkan Shift+Enter.
- Fix: pada `(pointer: coarse)`, Enter membuat baris baru dan kirim hanya lewat tombol; ganti petunjuknya dengan teks sentuh ("Tekan lama pesan untuk tindakan").
- Suggested command: /impeccable adapt

**[P2] Penanda hari gagal kontras AA**
- Why: "Hari ini" 12px tebal `text-muted` di atas `bg-ink/8` di kanvas terukur 4.21:1 di tema terang (ChatPanel.tsx:984) — melanggar Contrast First Rule proyek sendiri.
- Fix: `bg-surface border border-line` (muted di surface 5.34) atau teks lebih gelap.
- Suggested command: /impeccable colorize

**[P2] Kolom sunting tidak tumbuh mengikuti teks yang membungkus**
- Why: `rows` hanya dihitung dari baris baru (MessageBubble.tsx:354); pesan panjang disunting dalam satu baris terpotong. Petunjuk "Enter simpan · Esc batal" 12px.
- Fix: ukur otomatis dari `scrollHeight` (atau `field-sizing: content`), batasi ±6 baris; petunjuk 13px.
- Suggested command: /impeccable polish

## Persona Red Flags

**Alex (power user):** tanpa tombol tunggal balas/reaksi dari pesan terfokus; pemilih emoji tanpa pencarian; reaksi cepat tetap delapan.

**Sam (keyboard / pembaca layar):** fokus jatuh ke body setelah hapus; wilayah status kabar singkat dan hapus dipasang bersama teksnya; kebocoran roving focus dan 520 tab stop di pemilih emoji; Enter pada baris pesan terhapus diam saja.

**Casey (ponsel, satu tangan):** panah menu 28px (area tekan ±40×34) di pojok kanan atas, di bawah janji 44px; tombol kembali 40px; Enter mengirim; papan reaksi cepat membungkus dua baris dan menutup gelembungnya; setiap gelembung selalu membawa panah dan ikon reaksi.

**Pekerja kantor yang lebih tua:** di desktop tidak ada tanda terlihat bahwa pesan bisa dibalas; petunjuk pintasan hilang begitu mengetik; urungkan 8 detik dengan label kecil; jam 11.5px.

## Minor Observations

- Kebocoran mangga: "@semua" disorot sebagai `isMe` (ChatPanel.tsx:383) sehingga tampil mangga bahkan di gelembung pengirimnya sendiri.
- Cincin fokus baris pesan membentang selebar 1030px — terbaca seperti pilihan baris tabel.
- Cincin fokus tombol emoji bulat bertabrakan dengan garis pil kolom tulis; perlakukan seperti textarea (`cincin-sendiri`).
- Deretan pesan terhapus sebaiknya dilipat jadi satu baris ("5 pesan dihapus").
- Petunjuk rangkap tiga: placeholder, keadaan kosong, dan teks sr-only.
- Prop mati: `DeleteNotice onDismiss={() => {}}`.
- Hapus gagal hanya "Tutup", tanpa coba lagi.
- Tombol "Rekam" teal adalah elemen paling keras di kolom tulis saat diam — mengundang rekaman tak sengaja (belum terlihat langsung).
- Butir menu pertama tampil bercincin saat dibuka lewat klik sintetis — periksa dengan tetikus/sentuh sungguhan.
- ChatPanel.tsx kini 1.689 baris berisi 7 komponen.

## Questions to Consider

- Di desktop tanpa hover, apa yang memberi tahu orang berusia 58 tahun bahwa sebuah pesan bisa dijawab? Haruskah "Balas" terlihat tanpa hover?
- Kenapa pesan yang dihapus mendapat bobot visual yang sama dengan pesan nyata?
- Apakah Enter-untuk-kirim kebiasaan papan ketik yang dipaksakan ke ibu jari?
