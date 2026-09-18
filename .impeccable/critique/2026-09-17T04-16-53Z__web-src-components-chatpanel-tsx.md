---
target: chatpanel
total_score: 22
max_score: 40
na_heuristics: 
p0_count: 2
p1_count: 3
target_identity: "file:/mnt/data/Coding/go/chat-app/web/src/components/ChatPanel.tsx"
target_fingerprint: "sha256:77fc125510c497738a60e6bb660374583c8b11e0517f805f4f7666e0b66e2fef"
target_path: /mnt/data/Coding/go/chat-app/web/src/components/ChatPanel.tsx
timestamp: 2026-09-17T04-16-53Z
slug: web-src-components-chatpanel-tsx
closed: true
---
Method: dual-agent (A: review desain · B: detector + browser)

Catatan: chrome-devtools MCP gagal (Chrome tidak ada di /opt/google/chrome); kedua agen memakai Chromium Playwright lewat CDP/puppeteer. Server ini `attachments:false`, jadi rekam/unggah/pesan suara ditinjau dari kode saja.

## Design Health Score

| # | Heuristik | Skor | Masalah utama |
|---|---|---|---|
| 1 | Visibilitas status | 2 | Status koneksi hanya di Sidebar (tersembunyi di ponsel saat chat terbuka); tidak ada skeleton riwayat |
| 2 | Cocok dengan dunia nyata | 3 | Copy lugas; "Atur" di PinBar kabur |
| 3 | Kendali pengguna | 2 | "Hapus" langsung tanpa konfirmasi/urungkan; edit tersimpan saat blur |
| 4 | Konsistensi | 3 | Tombol tutup 24–28px vs tombol ikon lain 40px |
| 5 | Pencegahan error | 2 | "@" di grup memilih @semua lebih dulu, Enter membangunkan seluruh grup; Esc membuang rekaman |
| 6 | Mengenali, bukan mengingat | 3 | Enter/Shift+Enter/tempel hanya di `title` |
| 7 | Fleksibilitas | 2 | Tanpa pintasan pesan; baris aksi tak terjangkau keyboard |
| 8 | Estetika minimalis | 2 | Di layar sentuh, tiap pesan sendiri membawa 5 aksi teks + reaksi |
| 9 | Pemulihan error | 2 | "gagal — coba lagi" 11.5px; pesan gagal hilang setelah reload |
| 10 | Bantuan | 1 | Percakapan kosong = kanvas kosong, tanpa petunjuk |
| **Total** | | **22/40** | **Acceptable** |

## Design Specificity Verdict

LLM: sebagian besar memang dibuat untuk produk ini — tulang punggung gelembung dengan ekor 7px, cincin mangga untuk sebutan, pil peristiwa sistem, pemisah hari, slot kirim/rekam bersama, copy Indonesia yang rapi. Tapi saat diam layarnya masih "WhatsApp Web berwarna teal", dan di layar sentuh baris aksi teks di bawah tiap gelembung mengubah kanvas tenang jadi formulir padat.

Detector CLI: 0 temuan pada ChatPanel.tsx dan enam komponen anaknya.

Overlay browser (tema gelap, DM berisi 4 pesan, reaksi, sematan): 17 temuan desktop, 18 mobile.
- 16× `ai-color-palette` pada `accent-text` gelap `oklch(0.8 0.09 196)` — false positive: token Teal Laguna untuk Teks yang disengaja (DESIGN.md), kroma 0.09, dipakai hemat. Hit dihitung per svg/path/span (≈6 titik nyata).
- 1× `layout-transition` di `body` — false positive: tidak ada transition di body.
- 1× `text-overflow` di PinBar.tsx:97 (mobile) — false positive: `truncate` disengaja.
Detector tidak menangkap satu pun masalah prioritas di bawah; semuanya perilaku/aksesibilitas, bukan pola visual.

## Overall Impression

Sistem visualnya disiplin dan tokennya lolos kontras di kedua tema. Yang lemah ada di momen berisiko: hapus, gagal kirim, rekaman, dan akses keyboard. Peluang terbesar: satu model aksi pesan ("⋯" / tekan lama) yang aman, besar, dan terjangkau keyboard — menggantikan hover di desktop dan baris teks permanen di ponsel.

## What's Working

1. Alasan desain ditegakkan di kode: mangga hanya untuk sebutan, logika sudut gelembung, kontras token (putih di atas teal 4.58:1 terang / 4.62:1 gelap; muted di kanvas 4.87:1 / 7.54:1).
2. Kasus tepi nyata ditangani: navigasi keyboard daftar sebutan, tempel/seret lewat satu gerbang, unggahan menahan kirim, satu audio sekaligus, meter level saat merekam, tombol "Ke pesan terbaru".
3. Ketiadaan fitur rapi: tanpa lampiran, tombol klip dan mikrofon hilang, bukan mati.

## Priority Issues

**[P0] Hapus pesan tanpa konfirmasi atau urungkan**
- Why: `MessageBubble.tsx:348` langsung memanggil `deleteMessage`; tombol teks setinggi ~21px tepat di sebelah "Edit", terlihat permanen di layar sentuh, dan berlaku untuk semua orang.
- Fix: toast "Pesan dihapus · Urungkan" yang menahan hapus ±5 detik; pindahkan aksi merusak ke menu, jauh dari Edit, di urutan terakhir dan berwarna bahaya.
- Suggested command: /impeccable harden

**[P0] Pesan gagal lemah secara visual dan hilang setelah reload**
- Why: melanggar prinsip produk #1 ("pesan tidak boleh hilang"). Cincin bahaya 1px nyaris tak terlihat di gelembung teal (1.33:1 terang), opacity 70% menurunkan kontras teks ke 2.81:1, alasan hanya kata kecil bergaris bawah. Antrean pending di `store.ts` tidak disimpan.
- Fix: simpan antrean pending/gagal (IndexedDB/localStorage, kunci UUID klien — server sudah idempoten); buang `opacity-70`, pakai garis bahaya + ikon peringatan di samping gelembung, baris 13px "Tidak terkirim — tidak ada koneksi" dan tombol "Kirim ulang" 44px.
- Suggested command: /impeccable harden

**[P1] Aksi pesan tak terjangkau keyboard di desktop**
- Why: `MessageBubble.tsx:317` memakai `hidden group-hover:flex`; `display:none` mengeluarkan tombol dari urutan tab. Balas/Teruskan/Sematkan/Edit/Hapus hanya bisa lewat tetikus — bertentangan dengan target WCAG 2.2 AA.
- Fix: minimal `group-focus-within:flex`; idealnya pesan bisa difokus (roving tabindex dalam `role="log"`) dengan menu aksi yang bisa dibuka keyboard.
- Suggested command: /impeccable audit, lalu /impeccable harden

**[P1] Target sentuh terlalu kecil dan padat untuk penggunanya**
- Why: tombol aksi ~21px (`MessageBubble.tsx:362`), chip reaksi 26px, reaksi cepat 32px, tombol tutup 24–28px (`ChatPanel.tsx:648,799,852`, `UploadStrip.tsx:69`), toggle kecepatan 24px — di bawah aturan 40/44px sistem sendiri.
- Fix: di layar sentuh ganti baris permanen dengan tekan lama atau tombol "⋯" 44px yang membuka bottom sheet berlabel besar; chip minimal 32px dengan area tekan tambahan.
- Suggested command: /impeccable adapt + /impeccable distill

**[P1] Status dan keadaan kosong tidak ada di dalam percakapan**
- Why: indikator menyambung ulang hanya di Sidebar (tak terlihat di ponsel saat chat terbuka); tanpa skeleton riwayat; percakapan baru tampil sebagai kanvas mint kosong.
- Fix: bilah tipis `role="status"` di bawah header ("Menyambungkan ulang… pesan dikirim setelah tersambung"); keadaan kosong dengan avatar dan nama lawan bicara, "Belum ada pesan. Sapa Citra", plus petunjuk Enter/Shift+Enter/tempel gambar.
- Suggested command: /impeccable onboard + /impeccable clarify

## Persona Red Flags

**Alex (power user):** tanpa ↑ untuk edit terakhir, tanpa balas/reaksi lewat keyboard; "@" memilih @semua lebih dulu (`ChatPanel.tsx:334-339,409`) sehingga Enter cepat membangunkan seluruh grup; edit tersimpan saat blur.

**Sam (keyboard / pembaca layar):** baris aksi tak terjangkau; tidak ada `role="log"`/`aria-live` — pesan baru dan "sedang mengetik" tidak diumumkan; popup sebutan tanpa `listbox`/`option`/`aria-activedescendant`; textarea edit tanpa label; tanda terbaca hanya `title`; mulai merekam tidak diumumkan; aturan global `:focus-visible` (index.css) menimpa `focus-visible:outline-none` textarea sehingga muncul cincin kotak di dalam pil bulat.

**Casey (ponsel, satu tangan):** lima aksi teks kecil per gelembung, "Hapus" tepat di sebelah "Edit"; tombol reaksi 32px; pemilih emoji penuh bisa menutupi kolom tulis (`ReactionRow.tsx:136`); tanpa indikator koneksi saat chat terbuka.

**Pekerja kantor yang lebih tua (dari PRODUCT.md):** meta dan aksi 11.5px padahal harus dibaca untuk bertindak; tombol kirim saat kosong opacity 40% (1.76:1) terlihat rusak — tanpa mikrofon, kolom tulis selalu berakhir dengan tombol pudar; label "Diteruskan" 12px putih/80 di atas teal 3.5:1 (gagal AA); "Atur" tidak menjelaskan apa yang diatur; tidak ada petunjuk bahwa Enter mengirim.

## Minor Observations

- Baris "@semua membangunkan seluruh anggota" terlipat dua baris di popup w-72.
- Banner balas menulis "Membalas Bu Sri Wahyuni" saat membalas diri sendiri; lebih jelas "Membalas pesanmu".
- Pesan sistem "X menyematkan…" memakai nama pembaca sendiri, bukan "Kamu".
- PinBar memotong teks jadi satu baris bahkan di layar 1400px.
- Tema gelap: garis gelembung orang vs kanvas ≈1.04:1, surface vs canvas ≈1.01:1 — gelembung melayang tanpa tepi.
- "diedit" dan "mengirim…" 11.5px muted tanpa ikon.
- EmojiPicker `role="dialog"` tanpa memindahkan fokus; tab tanpa `aria-controls`; tombol emoji dilabeli glyph saja.
- Pemutar suara: "Tidak bisa diputar" tanpa tombol coba lagi.
- Grup tidak punya konfirmasi terkirim selain hilangnya status "mengirim…".

## Questions to Consider

- Kalau "Hapus" adalah aksi paling berbahaya, kenapa bobot dan jaraknya sama dengan "Balas", aksi paling sering?
- Pesan yang sudah ditulis tapi tak pernah terkirim — apakah itu termasuk "pesan hilang"? Sekarang reload menghapusnya.
- Apakah satu tombol "⋯" per pesan lebih jelas bagi semua umur daripada dua model sekaligus (hover di desktop, baris permanen di ponsel)?
