-- Fase 12: menemukan pesan — cari, teruskan, sematkan.
--
-- Tiga fitur yang saling menguatkan: pencarian menemukan pesan lama, pin
-- membuat yang penting tidak perlu dicari lagi, dan meneruskan membawa pesan
-- yang sudah ditemukan ke tempat lain. Ketiganya juga berbagi satu kebutuhan
-- yang belum ada sebelumnya — MELOMPAT ke pesan yang jauh di belakang riwayat —
-- tapi yang itu tidak butuh skema: `seq` yang tanpa lompatan sejak 0001 sudah
-- cukup untuk menghitung jendela di sekitar pesan mana pun.

-- ---------------------------------------------------------------------------
-- 1. Pencarian: tsvector, bukan trigram
-- ---------------------------------------------------------------------------
--
-- pg_trgm sudah terpasang sejak 0002 untuk mencari nama pengguna, dan untuk
-- nama itu pilihan yang tepat: teksnya pendek, dan orang mengetik potongan dari
-- tengah ("santo" untuk "Budi Santoso"). Isi pesan berbeda di tiga hal:
--
--  * Trigram mencocokkan POTONGAN huruf, bukan kata. "api" menemukan
--    "kapital" dan "sapi"; pada tabel berisi jutaan kalimat itu berarti hasil
--    yang sebagian besar tidak dicari siapa pun.
--  * Index trigram memuat setiap tiga huruf dari setiap pesan. Untuk teks
--    sepanjang pesan chat itu jauh lebih besar daripada index kata — lihat
--    angka di docs/menemukan-pesan.md.
--  * Kueri di bawah tiga huruf tidak bisa memakai index trigram sama sekali.
--
-- Konfigurasi 'simple', BUKAN 'indonesian'. Stemmer Indonesia di Postgres
-- diuji langsung sebelum baris ini ditulis, dan hasilnya:
--
--    mengirimkan -> irim      meeting  -> eting     berita -> ita
--    perdana     -> dana      bertemu  -> berte     diana  -> ana
--    kirimin     -> kirimin   ngirim   -> ngirim
--
-- Artinya mencari "kirim" TIDAK menemukan "mengirimkan" (akarnya jadi "irim"),
-- mencari "dana" menemukan "perdana", dan bahasa percakapan sehari-hari —
-- yang justru bahasa aplikasi ini — tidak disentuh sama sekali. Stemmer yang
-- salah lebih buruk daripada tidak ada stemmer: hasil yang hilang tanpa
-- penjelasan membuat orang berhenti percaya pada kotak pencarian.
--
-- 'simple' cuma memecah kata dan mengecilkan huruf. Kekurangannya ditutup
-- dengan pencocokan AWALAN di sisi kueri ("kirim" menemukan "kirimkan"), dan
-- yang tidak tertutup — "dikirim" — adalah kekurangan yang bisa dijelaskan
-- dalam satu kalimat.
--
-- Index ekspresi, bukan kolom tsvector tersimpan. Hasil pencarian diurutkan
-- dari yang TERBARU, bukan menurut ts_rank — di chat, yang dicari hampir selalu
-- "yang kemarin itu", bukan "yang paling mirip" — jadi tidak ada yang perlu
-- membaca tsvector-nya kembali dari tabel. Kolom tersimpan hanya menambah
-- ukuran setiap baris pesan untuk nilai yang tidak pernah dibaca.
--
-- Parsial: pesan sistem dan pesan yang dihapus tidak pernah boleh ditemukan,
-- dan keduanya memang tidak punya teks. Kueri pencarian menulis kedua syarat
-- ini persis sama supaya planner mau memakai index-nya.
--
-- Satu konsekuensi yang sudah diperiksa: UPDATE yang tidak mengubah `body` —
-- memasang lampiran ke pesan yang baru dikirim — tetap boleh jadi HOT update.
-- Postgres membandingkan nilai kolom yang dirujuk index, bukan sekadar daftar
-- kolom yang disebut UPDATE. (Jam reaksi memang sudah bukan HOT sejak 0005,
-- karena index parsialnya sendiri; index ini tidak mengubah apa pun di sana.)
--
-- Bukan CONCURRENTLY: migrator menjalankan tiap berkas di dalam transaksi, dan
-- selama index ini dibangun penulisan pesan tertahan — 8,6 detik pada sejuta
-- pesan. IF NOT EXISTS supaya index yang sudah dibuat lebih dulu dengan
-- CONCURRENTLY dilewati; langkahnya ada di docs/menemukan-pesan.md.
CREATE INDEX IF NOT EXISTS messages_search_idx ON messages
    USING gin (to_tsvector('simple', body))
    WHERE kind = 'user' AND deleted_at IS NULL;

-- ---------------------------------------------------------------------------
-- 2. Meneruskan
-- ---------------------------------------------------------------------------
--
-- Yang diteruskan adalah ISINYA, bukan asal-usulnya. Kolom ini cuma penanda
-- "diteruskan", tanpa penunjuk ke pesan sumber, tanpa nama penulis aslinya, dan
-- tanpa nama percakapan asalnya.
--
-- Alasannya: pesan sumber berada di percakapan yang belum tentu diikuti
-- penerima. Menyebut siapa yang menulisnya dan di grup mana adalah membuka
-- sesuatu yang tidak pernah dibagikan oleh penulisnya — yang memutuskan untuk
-- membagikan cuma orang yang meneruskan, dan dia hanya bisa memutuskan untuk
-- isinya. Penunjuk ke pesan sumber, walau tidak pernah dikirim ke client,
-- adalah jalur kebocoran yang persis sama dengan yang ditutup pemeriksaan
-- balasan di 0005 — lebih aman tidak menyimpannya daripada menjaganya.
--
-- Konstanta default: menambah kolom ini tidak menulis ulang tabel.
ALTER TABLE messages ADD COLUMN forwarded boolean NOT NULL DEFAULT false;

-- Lampiran yang diteruskan TIDAK disalin byte-nya. Barisnya yang disalin —
-- pemilik baru, pesan baru, izin baca baru — dan kunci penyimpanannya dipakai
-- bersama. Menyalin byte berarti foto sepuluh megabyte yang diteruskan ke lima
-- grup memakan enam kali ruangnya.
--
-- Harga dari berbagi kunci: penyapu tidak boleh lagi membuang byte hanya
-- karena SATU baris yang menunjuknya sudah yatim. Dia harus bertanya apakah
-- masih ada baris lain, dan dua index ini yang membuat pertanyaan itu murah.
CREATE INDEX attachments_storage_key_idx ON attachments (storage_key);
CREATE INDEX attachments_thumb_key_idx ON attachments (thumb_key)
    WHERE thumb_key IS NOT NULL;

-- ---------------------------------------------------------------------------
-- 3. Menyematkan
-- ---------------------------------------------------------------------------
--
-- Sematan adalah KEADAAN, dan tabel ini memegangnya. KEJADIANNYA — siapa
-- menyematkan apa, kapan — dicatat sebagai pesan sistem, persis seperti
-- pengelolaan grup di 0006. Itu yang membuat sematan tidak butuh jam ketiga:
-- client yang menyusul setelah reconnect menerima catatan sistemnya lewat
-- cursor `seq` biasa, dan catatan itulah tanda untuk membaca ulang daftar ini.
--
-- Bandingkan dengan reaksi di 0005, yang terpaksa punya jam sendiri karena
-- tidak ada yang layak dicatat di riwayat untuk setiap penekanan emoji.
--
-- Satu pesan hanya bisa disematkan sekali, jadi message_id cukup jadi kunci.
CREATE TABLE pinned_messages (
    message_id      uuid        PRIMARY KEY REFERENCES messages(id) ON DELETE CASCADE,
    -- Disalin dari pesannya supaya daftar sematan sebuah percakapan tidak
    -- menuntut join untuk sekadar menemukan barisnya. Tidak pernah berubah:
    -- pesan tidak pernah pindah percakapan.
    conversation_id uuid        NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    -- SET NULL, bukan CASCADE: sematan yang penting tidak boleh ikut hilang
    -- hanya karena akun yang memasangnya dihapus.
    pinned_by       uuid        REFERENCES users(id) ON DELETE SET NULL,
    pinned_at       timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX pinned_messages_conversation_idx
    ON pinned_messages (conversation_id, pinned_at DESC);

-- Pemeriksaan foreign key saat sebuah user dihapus — alasan yang sama dengan
-- messages_sender_idx di 0002.
CREATE INDEX pinned_messages_pinned_by_idx ON pinned_messages (pinned_by)
    WHERE pinned_by IS NOT NULL;

ANALYZE messages;
ANALYZE attachments;
