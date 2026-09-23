-- Fase 15: bentuk gelombang pada pemutar pesan suara.
--
-- Fase 13 memberi pemutar satu angka — panjang rekaman. Yang masih hilang
-- adalah bentuknya: di mana orangnya bicara, di mana dia diam, dan apakah
-- rekaman lima belas detik itu berisi kalimat atau berisi sunyi karena
-- mikrofonnya dibisukan dari perangkat. Angka "0:15" tidak menjawab satu pun
-- dari itu, dan menggeser penunjuk ke tengah rekaman tanpa bentuk adalah
-- menebak.
--
-- Asal datanya sama persis dengan duration_ms, dan karena alasan yang sama:
-- berkasnya sendiri tidak menyebutnya. Membaca puncak-puncak dari sebuah
-- WebM/Opus menuntut mendekode seluruh Opus — dekoder audio di dalam proses
-- server, untuk sesuatu yang murni hiasan navigasi. Yang sudah memegang
-- angkanya tanpa biaya tambahan adalah perekamnya: AnalyserNode di client
-- sudah menghitung tingkat suara tiap 100 ms sejak Fase 13, cuma dibuang
-- setelah digambar di bilah perekam.
--
-- Bentuknya: 40 batang, tiap batang satu karakter base64url yang bernilai
-- 0..63. Empat puluh karena itu yang muat: batang 3 px dengan celah 1 px, sama
-- dengan bilah perekam, di lebar pemutar yang tersisa setelah tombol putar dan
-- tombol kecepatan. Satu kolom text 40 karakter, bukan array smallint — yang
-- dibaca client selalu seluruhnya, tidak pernah satu elemen, dan 40 byte
-- melewati salinan jsonb pesan tanpa perlu diterjemahkan di kedua ujungnya.
--
-- NULL untuk semua lampiran lain, termasuk berkas audio yang diunggah dari
-- disk: pemutarnya kembali ke penggeser polos, persis seperti sebelum fase
-- ini. peaks yang terisi berarti hal yang sama dengan duration_ms yang
-- terisi — direkam di dalam aplikasi.

ALTER TABLE attachments
    ADD COLUMN peaks text
        CHECK (peaks IS NULL OR peaks ~ '^[A-Za-z0-9_-]{40}$');
