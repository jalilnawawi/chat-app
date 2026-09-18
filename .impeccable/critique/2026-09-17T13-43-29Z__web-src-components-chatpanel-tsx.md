---
target: chatpanel
total_score: 30
max_score: 40
na_heuristics: 
p0_count: 0
p1_count: 2
target_identity: "file:/mnt/data/Coding/go/chat-app/web/src/components/ChatPanel.tsx"
target_fingerprint: "sha256:371a67eee372203f0309144235a2c56bf8d4aa6db7bb3463d6463491b0551a19"
target_path: /mnt/data/Coding/go/chat-app/web/src/components/ChatPanel.tsx
timestamp: 2026-09-17T13-43-29Z
slug: web-src-components-chatpanel-tsx
---
Method: dual-agent (A: review desain · B: detector + browser)

Catatan: chrome-devtools MCP tidak bisa jalan; A memakai Chromium headless dengan hover dipaksa aktif dan mikrofon palsu, B memakai Chromium terlihat lewat CDP. Lampiran aktif (suara, berkas, gambar diuji langsung).

## Design Health Score

| # | Heuristik | Skor | Masalah utama |
|---|---|---|---|
| 1 | Visibilitas status | 3 | Mengirim saat menggulir ke atas tidak memberi tanda terlihat; tanpa pil "pesan baru ↓" di luar mode jendela lama |
| 2 | Cocok dengan dunia nyata | 3 | Keadaan kosong ponsel mengajarkan "tekan lama" yang hanya bekerja di Android |
| 3 | Kendali pengguna | 3 | Tidak ada jalan kembali ke pesan terbaru setelah menggulir biasa |
| 4 | Konsistensi | 3 | Tombol di dalam pil kolom tulis memakai cincin fokus bawaan browser |
| 5 | Pencegahan error | 4 | Kuat: @semua terakhir, buang rekaman dua ketukan, sunting hanya atas perintah, kirim menunggu unggahan |
| 6 | Mengenali, bukan mengingat | 3 | Di desktop panah dan reaksi hanya muncul saat hover; petunjuk pintasan hanya saat kolom difokus |
| 7 | Fleksibilitas | 3 | Tanpa tombol/pintasan ke pesan terbaru |
| 8 | Estetika minimalis | 3 | Di layar sentuh setiap gelembung selalu membawa panah dan tombol reaksi |
| 9 | Pemulihan error | 3 | "Tidak bisa diputar · Coba lagi" 11.5px ±3.5:1 |
| 10 | Bantuan | 2 | Pintasan hanya tertulis di baris 13px yang sementara |
| **Total** | | **30/40** | **Good** |

## Design Specificity Verdict

LLM: spesifik — tiap warna satu tugas, ekor gelembung dan tulang punggung rentetan, ikon sendiri, keputusan yang tercatat di kode. Kelemahan ada di tindak lanjut: kontras teks di dalam gelembung sendiri, gulir yang bisa meninggalkan pesan sendiri di luar layar, cincin fokus bawaan di pil kolom tulis, dan dua kontrol permanen per gelembung di ponsel. Kepribadian tenang cenderung "aplikasi chat teal yang cakap"; mangga satu-satunya momen berkarakter.

Batas refactor: sebagian besar baik (ComposerHandle sempit, aturan gulir dan papan ketik di hook sendiri). Gesekan: `excerpt` tinggal di PinBar tapi dipakai modul non-UI; `formatDuration` di VoicePlayer dipakai empat berkas; bilah-bilah kabar masih inline di ChatPanel; MessageBubble 670 baris; kelas tombol pil disalin-tempel; teks UI tersebar (bertentangan dengan rencana i18n).

Detector CLI: 0 temuan di 15 berkas.

Overlay browser (DM terang/gelap dengan suara, berkas, reaksi, sematan, lipatan terhapus): 50 temuan desktop, 51 mobile.
- `ai-color-palette` (48–49): false positive — token teal teks gelap yang disengaja.
- `clipped-overflow-container` (1): false positive — kontrol pesan yang tergulir keluar area baca.
- `text-occlusion` (2, mobile, "Ok"/"Siap"): false positive setelah diukur ulang — kotak paragraf ikut memuat ruang sudut panah, tapi glyph teksnya (27–48, 304–336) tidak bersinggungan dengan panah (58–86, 346–374).

## Overall Impression

Naik dari 29 ke 30. Pencegahan error kini penuh, dan fondasi papan ketik/pembaca layar sangat baik. Yang tersisa: kontras di dalam gelembung teal, gulir pesan sendiri, dan kerapatan kontrol di layar sentuh.

## What's Working

1. Disiplin sinyal di kedua tema: mangga hanya untuk panggilan, cincin gagal berjarak, terhapus putus-putus miring; meta gelap 7.6:1, teks aksen 10.3:1.
2. Fondasi papan ketik dan pembaca layar: riwayat satu tab stop, cincin pada gelembung, Enter membuka menu, Esc kembali, wilayah status selalu terpasang, fokus pindah ke Urungkan.
3. Kasus tepi disengaja: lipatan terhapus, rekaman ke laci asalnya, @semua tak pernah terpilih, tombol kirim bergaris saat nonaktif.

## Priority Issues

**[P1] Teks di dalam gelembung sendiri (teal) gagal AA**
- Why: isi kutipan 13px 2.85:1 (`opacity-80`), penulis kutipan 3.2:1, ukuran berkas 12px 2.69:1 (`text-accent-ink/75` di atas `accent-ink/15`), jam pesan suara 11.5px 3.5:1. Putih di teal hanya 4.58:1, jadi transparansi apa pun jatuh di bawah 4.5.
- Fix: buang opacity pada teks di gelembung sendiri; latar kutipan teal lebih gelap (±L 0.45) dengan teks putih penuh; meta di dalam gelembung putih penuh, dibedakan lewat bobot/ukuran; tambahkan aturan kontras "teks di gelembung sendiri".
- Suggested command: /impeccable colorize

**[P1] Gulir bisa meninggalkan pesan terbaru di luar layar**
- Why: mengirim pesan sendiri saat menggulir ke atas tidak menggulir ke bawah (1333px dari bawah, pesan tak terlihat). Dua kali di desktop percakapan terbuka dengan gelembung gambar terakhir terpotong — kemungkinan konten yang tumbuh belakangan, karena ResizeObserver hanya mengamati kotak gulir, bukan isinya (dugaan).
- Fix: selalu turun saat pesan tertunda baru adalah milik sendiri; amati juga pembungkus isi; tampilkan pil "Pesan baru ↓" setiap kali tidak di bawah.
- Suggested command: /impeccable harden

**[P2] Tombol di pil kolom tulis memakai cincin fokus bawaan browser**
- Why: `inBoxButton` punya `cincin-sendiri` (keluar dari cincin global) tapi tanpa `outline-none`; terhitung `outline: auto 1px` — lingkaran gelap memotong lengkung pil, bertumpuk dengan cincin inset.
- Fix: tambahkan `outline-none`; periksa semua elemen `cincin-sendiri` selain textarea.
- Suggested command: /impeccable polish

**[P2] Layar sentuh: dua kontrol permanen per gelembung, reaksi cepat di atas layar**
- Why: panah dan tombol reaksi selalu tampil di ponsel — kolom ikon di selokan yang "terbaca seperti tabel". Papan cepat `fixed top-20`: titik terjauh dari ibu jari, menutup bilah sematan, tanpa kaitan visual ke pesannya.
- Fix: di layar sentuh, buang tombol reaksi samping dan taruh 8 reaksi cepat sebagai baris teratas lembar tindakan (sudah di bawah ibu jari dan sudah menyebut pesannya).
- Suggested command: /impeccable adapt, lalu /impeccable distill

**[P2] Label baris untuk pembaca layar menyembunyikan isi; petunjuk Shift+Tab tidak akurat**
- Why: `rowLabel` hanya "Catatan sistem" untuk semua catatan sistem, dan `aria-label` pada article menggantikan isinya; label juga tanpa jenis lampiran, status gagal, sebutan. "Shift+Tab ke riwayat" butuh 3 tekanan.
- Fix: `rowLabel` memakai `systemText` dan menambahkan "menyebut kamu", "gagal terkirim", "N reaksi"; ubah petunjuknya.
- Suggested command: /impeccable audit

## Persona Red Flags

**Alex (power user):** tanpa lompat ke terbaru; reaksi cepat butuh Enter → menu atau tetikus; pintasan hanya tampil saat difokus dan hanya ≥64rem.

**Sam (keyboard / pembaca layar):** catatan sistem buram; Shift+Tab butuh 3 tekanan; setelah hapus lewat papan ketik fokus di Urungkan bercincin sehingga hitung mundur terjeda sampai fokus pergi — "8 detik" menyesatkan.

**Casey (ponsel, satu tangan):** reaksi cepat di atas; kontrol permanen di setiap gelembung; chip reaksi 32px (di bawah lantai 40px); penggeser pesan suara 20px; petunjuk tekan lama tidak berlaku di iOS.

**Pekerja kantor yang lebih tua:** teks kutipan dan berkas kontras rendah di gelembung sendiri; panah tak terlihat sebelum hover di desktop (petunjuk klik kanan hanya di tooltip); "1×" tanpa arti terlihat; meta 11.5px.

## Minor Observations

- Keadaan kosong ponsel memberi petunjuk tindakan pesan padahal belum ada pesan.
- Istilah lampiran tidak seragam: "🎵 Rekaman suara" (kindLabel) vs "🎤 Pesan suara 0:03" (excerpt); emoji di teks akan menyulitkan i18n.
- `time` di MessageBubble menduplikasi `jam` di chatHistory.
- Hingga enam bilah bisa bertumpuk di atas kolom tulis tanpa urutan/batas yang dirancang.
- Bilah sematan desktop dua baris; bisa satu baris plus "Lihat semua".
- Panah di gelembung foto menutupi foto (disengaja, tapi menyembunyikan isi).
- Bilah rekam dengan titik rata kanan terbaca "tidak ada yang tertangkap" di detik-detik awal.

## Questions to Consider

- Lembar tindakan sudah di bawah ibu jari dan menyebut pesannya — kenapa reaksi dibuka di atas layar? Bisakah lembar jadi satu-satunya tempat tindakan pesan di layar sentuh?
- Haruskah mengirim pesan sendiri pernah meninggalkanmu di tempat pesan itu tak terlihat?
- Mangga satu-satunya momen berkepribadian. Apakah "tenang" bergeser ke "chat teal anonim" — apa yang membuat "Pagi di Tepi Air" dikenali tanpa sidebar?
