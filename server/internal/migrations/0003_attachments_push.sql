-- Fase 7: lampiran dan push notification.
--
-- Dua tabel, dua alasan berbeda:
--
--  1. `attachments` adalah OTORITAS sebuah lampiran — siapa pemiliknya, di
--     kunci mana isinya tersimpan, dan pesan mana yang memakainya. Kolom
--     `messages.attachments` (jsonb, sudah disiapkan sejak 0001) tetap dipakai,
--     tapi perannya hanya salinan-baca: menampilkan riwayat tidak boleh menuntut
--     satu JOIN tambahan per pesan.
--
--     Kenapa perlu tabel sendiri kalau jsonb-nya sudah ada? Karena file
--     diunggah SEBELUM pesannya terkirim. Di antara dua momen itu — bisa
--     beberapa detik, bisa selamanya kalau orangnya berubah pikiran — file itu
--     sudah ada isinya tapi belum punya pesan. Dia tetap butuh pemilik (untuk
--     izin baca) dan tetap butuh tercatat (untuk bisa dibuang).
--
--  2. `push_subscriptions` menyimpan izin yang diberikan browser. Satu baris
--     per pemasangan browser, bukan per user: satu orang bisa memasang di
--     ponsel dan laptop, dan mencabut izin di salah satunya tidak boleh
--     mematikan yang lain.

CREATE TABLE attachments (
    id          uuid        PRIMARY KEY,
    owner_id    uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- NULL selama lampiran belum dipasang ke pesan mana pun. Baris seperti ini
    -- hanya boleh dibaca pemiliknya sendiri, dan akan disapu kalau tidak
    -- kunjung dipakai.
    message_id  uuid        REFERENCES messages(id) ON DELETE CASCADE,

    -- Path di filer SeaweedFS. Sengaja disimpan, bukan disusun ulang dari id:
    -- tata letak penyimpanan boleh berubah tanpa membuat file lama hilang.
    storage_key text        NOT NULL,

    -- Nama asli dari komputer pengunggah, dipakai saat file diunduh kembali.
    -- Tidak pernah ikut menentukan tempat penyimpanan.
    name        text        NOT NULL,
    mime        text        NOT NULL,
    size        bigint      NOT NULL CHECK (size >= 0),

    -- Hanya untuk gambar. Dikirim client dan dipakai memesan ruang di layar
    -- sebelum gambarnya selesai dimuat — tanpa ini daftar pesan melompat-lompat
    -- saat gambar berdatangan.
    width       integer,
    height      integer,

    created_at  timestamptz NOT NULL DEFAULT now()
);

-- "Lampiran milik pesan ini" — dijalankan saat memasang lampiran ke pesan.
CREATE INDEX attachments_message_idx ON attachments (message_id)
    WHERE message_id IS NOT NULL;

-- Pembersih lampiran yatim: yang belum terpasang dan sudah tua. Index parsial
-- ini hanya memuat baris yang sedang menganggur, jadi ukurannya tetap kecil
-- walau tabelnya tumbuh — dan kembali mengecil begitu lampirannya terpakai.
CREATE INDEX attachments_orphan_idx ON attachments (created_at)
    WHERE message_id IS NULL;

CREATE TABLE push_subscriptions (
    -- Endpoint adalah URL yang diberikan browser dan sudah unik per pemasangan.
    -- Memakainya sebagai primary key membuat pendaftaran ulang — yang terjadi
    -- tiap kali service worker memperbarui langganannya — menimpa baris yang
    -- sama alih-alih menumpuk duplikat.
    endpoint   text        PRIMARY KEY,
    user_id    uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- Kunci enkripsi payload. Web Push mengenkripsi di sisi kita dan hanya
    -- browser tujuan yang bisa membukanya; layanan push di tengah meneruskan
    -- byte yang tidak bisa dia baca.
    p256dh     text        NOT NULL,
    auth       text        NOT NULL,

    user_agent text        NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    last_ok_at timestamptz
);

CREATE INDEX push_subscriptions_user_idx ON push_subscriptions (user_id);

ANALYZE attachments;
ANALYZE push_subscriptions;
