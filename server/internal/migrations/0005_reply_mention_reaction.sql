-- Fase 9: membalas, menyebut, dan bereaksi.
--
-- Ketiganya adalah metadata yang menempel pada SATU pesan tertentu, dan itu
-- yang membuatnya layak dikerjakan bersamaan. Yang membedakan ketiganya adalah
-- apakah metadata itu bisa BERUBAH setelah pesannya terkirim — dan jawabannya
-- menentukan apakah isinya boleh disalin atau harus dibaca ulang setiap kali.
--
--  * Balasan menunjuk pesan lain, dan isi pesan itu berubah: diedit, dihapus.
--    Jadi yang disimpan hanya PENUNJUKNYA. Isinya diambil lewat self-join saat
--    riwayat dibaca.
--
--  * Sebutan ditentukan sekali saat kirim dan tidak pernah berubah sesudahnya —
--    mengedit teks tidak membangunkan orang baru. Jadi dia boleh jadi kolom
--    larik di barisnya sendiri, tanpa tabel dan tanpa query kedua. Ini
--    kebalikan dari keputusan balasan di atas, dan perbedaannya justru
--    menjelaskan aturannya: yang tidak pernah berubah boleh disalin.
--
--  * Reaksi berubah terus-menerus, ditambah dan dicabut oleh banyak orang pada
--    pesan yang sama. Dia butuh tabelnya sendiri, dan butuh cara memberi tahu
--    client yang sedang offline bahwa sesuatu berubah pada pesan LAMA — yang
--    `seq` pesannya sendiri tidak pernah bergerak.

-- ---------------------------------------------------------------------------
-- 1. Balas / kutip
-- ---------------------------------------------------------------------------

-- ON DELETE SET NULL, bukan CASCADE: menghapus pesan yang dikutip tidak boleh
-- ikut menghapus pesan yang mengutipnya. Dalam praktiknya penghapusan pesan di
-- aplikasi ini bersifat soft delete, jadi jalur ini hanya terpakai saat seluruh
-- percakapannya dibuang.
ALTER TABLE messages
    ADD COLUMN reply_to_id uuid REFERENCES messages(id) ON DELETE SET NULL;

-- Foreign key tanpa index pendamping berarti setiap penghapusan pesan memaksa
-- pemindaian penuh tabel ini sambil memegang kunci — persoalan yang sama
-- dengan messages.sender_id di 0002. Parsial, karena sebagian besar pesan tidak
-- membalas apa pun.
CREATE INDEX messages_reply_to_idx ON messages (reply_to_id)
    WHERE reply_to_id IS NOT NULL;

-- ---------------------------------------------------------------------------
-- 2. Sebutan (mention)
-- ---------------------------------------------------------------------------

-- Daftar id yang disebut, dikirim EKSPLISIT oleh client dan divalidasi server
-- (tiap id harus anggota percakapan ini). Server tidak pernah mengurai "@nama"
-- dari teks: nama tampilan boleh mengandung spasi, dan siapa yang dibangunkan
-- tidak boleh ditentukan oleh cara sebuah string kebetulan ditulis.
--
-- Larik, bukan tabel penghubung. Tidak ada foreign key yang bisa dipasang pada
-- elemen larik, dan itu memang harga yang dibayar — tapi isinya sudah melewati
-- pemeriksaan keanggotaan di dalam transaksi pengiriman, dan tidak pernah
-- ditulis lagi setelah itu. Imbalannya: riwayat tetap satu query.
ALTER TABLE messages ADD COLUMN mentions uuid[] NOT NULL DEFAULT '{}';

-- @semua dipisahkan dari daftar di atas, bukan diterjemahkan jadi daftar berisi
-- seluruh anggota. Keanggotaan grup berubah, dan pesan lama harus tetap berarti
-- "semua orang" — bukan "semua orang yang kebetulan ada di sana waktu itu".
ALTER TABLE messages ADD COLUMN mentions_all boolean NOT NULL DEFAULT false;

-- Penanda "ada yang menyebut kamu", DIPISAHKAN dari last_read_seq.
--
-- Keduanya menjawab pertanyaan berbeda. last_read_seq bergerak begitu ruangnya
-- dibuka sekilas; sebutan harus bertahan sampai orangnya benar-benar sampai ke
-- pesan yang menyebut namanya. Menyatukan keduanya berarti membuka percakapan
-- sebentar untuk melihat apa yang terjadi sudah cukup untuk melupakan bahwa ada
-- yang memanggil.
--
--   mention_seq     = seq pesan TERAKHIR yang menyebut anggota ini
--   mention_ack_seq = sejauh mana sebutan itu sudah benar-benar dilihat
--
-- Badge menyala selama mention_seq > mention_ack_seq.
ALTER TABLE conversation_members ADD COLUMN mention_seq     bigint NOT NULL DEFAULT 0;
ALTER TABLE conversation_members ADD COLUMN mention_ack_seq bigint NOT NULL DEFAULT 0;

-- ---------------------------------------------------------------------------
-- 3. Reaksi
-- ---------------------------------------------------------------------------

-- Bentuk primary key-nya yang memaksakan aturannya, bukan kode aplikasi: satu
-- orang boleh memberi beberapa emoji berbeda pada satu pesan, tapi tidak bisa
-- memberi emoji yang sama dua kali. Menekan tombol yang sama dua kali karena
-- jaringan lambat menghasilkan keadaan yang sama, bukan hitungan ganda.
CREATE TABLE message_reactions (
    message_id uuid        NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    user_id    uuid        NOT NULL REFERENCES users(id)    ON DELETE CASCADE,

    -- Panjangnya dibatasi di sini juga, bukan hanya di aplikasi. Satu grafem
    -- emoji bisa panjang — bendera, keluarga dengan ZWJ, warna kulit — tapi
    -- tidak sepanjang kalimat. Kolom teks bebas di sini berarti pesan kedua
    -- yang menyamar jadi reaksi.
    emoji      text        NOT NULL CHECK (char_length(emoji) BETWEEN 1 AND 24),

    created_at timestamptz NOT NULL DEFAULT now(),

    PRIMARY KEY (message_id, user_id, emoji)
);

-- Pemeriksaan foreign key saat sebuah user dihapus, alasan yang sama dengan
-- messages_sender_idx di 0002.
CREATE INDEX message_reactions_user_idx ON message_reactions (user_id);

-- ---------------------------------------------------------------------------
-- 4. Jam kedua: supaya reaksi ikut menyusul lewat jalur resume
-- ---------------------------------------------------------------------------
--
-- Resume di Fase 3 bekerja dengan satu pertanyaan: "pesan apa yang seq-nya
-- lebih besar dari punyaku?". Reaksi tidak muat di pertanyaan itu — dia
-- mengubah pesan LAMA, yang seq-nya sudah lama berhenti bergerak. Orang yang
-- menutup laptop lalu membukanya lagi akan melihat pesannya lengkap dan
-- reaksinya hilang semua.
--
-- Jadi percakapan punya penghitung kedua, dan tiap pesan menyimpan nilai
-- penghitung itu pada saat terakhir reaksinya berubah. Menambah DAN mencabut
-- sama-sama menaikkannya, sehingga pencabutan pun punya jejak — tanpa perlu
-- menyimpan baris nisan untuk reaksi yang sudah tidak ada.
--
-- Yang dikirim saat menyusul adalah KEADAAN reaksi pesan itu sekarang, bukan
-- daftar kejadian yang terlewat. Dua puluh orang yang menekan dan melepas emoji
-- yang sama tetap menghasilkan satu kiriman, dan hasilnya benar tanpa client
-- harus memutar ulang urutannya.
ALTER TABLE conversations ADD COLUMN reaction_seq bigint NOT NULL DEFAULT 0;
ALTER TABLE messages      ADD COLUMN reaction_seq bigint NOT NULL DEFAULT 0;

-- Parsial: sebagian besar pesan tidak pernah menerima satu reaksi pun, dan
-- index yang hanya memuat yang pernah berubah tetap kecil walau tabelnya
-- tumbuh — pola yang sama dengan attachments_orphan_idx di 0003.
--
-- Konsekuensi yang disengaja: UPDATE pada reaction_seq menyentuh index ini,
-- jadi dia bukan HOT update. Itu dibayar hanya oleh pesan yang benar-benar
-- direaksikan, dan reaksi jauh lebih jarang daripada pesan.
CREATE INDEX messages_reaction_seq_idx ON messages (conversation_id, reaction_seq)
    WHERE reaction_seq > 0;

ANALYZE messages;
ANALYZE conversations;
ANALYZE conversation_members;
ANALYZE message_reactions;
