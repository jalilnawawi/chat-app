-- Fase 10: kelola akun & profil.
--
-- Sampai Fase 9 sebuah akun hanya punya username, nama tampilan, dan password.
-- Tidak ada cara mengubah apa pun setelah mendaftar, tidak ada email, dan tidak
-- ada foto. Berkas ini menutup itu.
--
-- Satu keputusan menaungi seluruhnya, dan kalau salah akan terasa di mana-mana:
--
--   **Status BUKAN presence.**
--
-- Presence (online/offline) sudah ada sejak Fase 6 — diturunkan dari koneksi
-- yang hidup, disimpan di Redis dengan TTL, dan sengaja fana: instance yang
-- mati tersapu sendiri. Status ("available", "busy", "sedang rapat sampai
-- 13.00") adalah pernyataan yang dibuat orang dengan SENGAJA, dan harus
-- bertahan melewati tutup laptop, ganti perangkat, dan logout.
--
-- Menyatukan keduanya berarti status "busy sampai jam 1" yang baru saja
-- seseorang pasang lenyap begitu dia menutup tab. Jadi presence tetap di Redis,
-- status tinggal di sini, dan client menampilkan gabungan keduanya.

-- ---------------------------------------------------------------------------
-- 1. Foto profil
-- ---------------------------------------------------------------------------

-- Empat kolom, dan yang paling penting adalah yang pertama: `avatar_id`.
--
-- Fase 8 memasang `Cache-Control: private, max-age=31536000, immutable` pada
-- semua isi lampiran, dan itu benar untuk byte yang memang tidak pernah
-- berubah. Avatar BERUBAH. Alamat tetap seperti /api/users/{id}/avatar berarti
-- browser menyimpan foto lama selama setahun dan tidak pernah lagi bertanya —
-- orang mengganti fotonya, dan tidak seorang pun melihatnya.
--
-- Jadi tiap unggahan menghasilkan id baru, persis seperti lampiran, dan
-- alamatnya ikut berubah. Dengan begitu cache setahun kembali menjadi benar,
-- bukan menjadi jebakan.
--
-- `avatar_key` adalah alamat internal penyimpanan dan tidak pernah dikirim ke
-- client, dengan alasan yang sama dengan `attachments.storage_key` di 0003.
ALTER TABLE users
    ADD COLUMN avatar_id   uuid,
    ADD COLUMN avatar_key  text,
    ADD COLUMN avatar_mime text,
    ADD COLUMN avatar_size bigint;

-- Keempatnya lahir dan mati bersama, pola yang sama dengan kolom turunan gambar
-- di 0004. Sebagian terisi berarti ada jalur kode yang menulis separuh keadaan.
ALTER TABLE users
    ADD CONSTRAINT users_avatar_lengkap CHECK (
        (avatar_id IS NULL     AND avatar_key IS NULL     AND
         avatar_mime IS NULL   AND avatar_size IS NULL) OR
        (avatar_id IS NOT NULL AND avatar_key IS NOT NULL AND
         avatar_mime IS NOT NULL AND avatar_size IS NOT NULL)
    );

-- Alamat baca avatar adalah /api/avatars/{avatar_id}, jadi id inilah yang
-- dicari saat byte-nya diminta — bukan id penggunanya.
CREATE UNIQUE INDEX users_avatar_id_idx ON users (avatar_id) WHERE avatar_id IS NOT NULL;

-- Sampah penyimpanan yang menunggu dibuang.
--
-- Avatar lama TIDAK dihapus langsung saat diganti. Penghapusannya adalah satu
-- permintaan HTTP ke penyimpanan yang bisa gagal sendiri, dan kalau dia gagal
-- di dalam jalur "ganti foto" ada dua pilihan sama buruknya: menggagalkan
-- penggantian foto yang sebenarnya sudah berhasil, atau menelan kegagalannya —
-- dan dengan itu melupakan kunci tersebut selamanya, karena tidak ada satu
-- baris pun lagi yang menyebutnya.
--
-- Barisnya dicatat di sini, di dalam transaksi yang sama dengan penggantian
-- fotonya, lalu penyapu lampiran yatim yang sudah ada sejak Fase 7 yang
-- membuangnya. Yang gagal tetap tercatat dan dicoba lagi putaran berikutnya.
CREATE TABLE blob_garbage (
    key        text        PRIMARY KEY,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX blob_garbage_created_idx ON blob_garbage (created_at);

-- ---------------------------------------------------------------------------
-- 2. Email
-- ---------------------------------------------------------------------------

-- Boleh kosong: akun yang dibuat sebelum fase ini tidak punya email, dan
-- memaksa mereka mengisinya sebelum boleh memakai aplikasi adalah mengubah
-- fitur baru jadi penghalang.
--
-- `email_verified_at` NULL berarti alamatnya diketik tapi belum dibuktikan
-- kepemilikannya, dan alamat seperti itu TIDAK BOLEH dipakai memulihkan akun
-- sama sekali. Kalau boleh, memulihkan akun orang lain cuma butuh mengaku
-- memiliki sebuah alamat.
ALTER TABLE users
    ADD COLUMN email             text,
    ADD COLUMN email_verified_at timestamptz;

-- Case-insensitive tanpa citext, pola yang sama dengan username di 0001.
-- Partial: banyak akun boleh sama-sama tidak punya email.
CREATE UNIQUE INDEX users_email_lower_idx ON users (lower(email)) WHERE email IS NOT NULL;

-- Token verifikasi email dan token reset password tinggal di satu tabel karena
-- bentuknya memang sama: sekali pakai, berumur pendek, dan disimpan sebagai
-- HASH — alasannya sama persis dengan token sesi di 0001. Bocornya tabel ini
-- tidak memberi penyerang satu pun tautan yang bisa dipakai.
CREATE TABLE email_tokens (
    token_hash bytea       PRIMARY KEY,
    user_id    uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind       text        NOT NULL CHECK (kind IN ('verify', 'reset')),

    -- Alamat yang dituju, DISALIN ke dalam barisnya.
    --
    -- Untuk 'verify' dia adalah alamat yang sedang dibuktikan, dan dia harus
    -- dicocokkan lagi saat tautannya diklik: orang yang meminta verifikasi
    -- untuk alamat A lalu menggantinya jadi B tidak boleh memverifikasi B
    -- dengan tautan yang dikirim ke A.
    email      text        NOT NULL,

    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL
);

CREATE INDEX email_tokens_user_idx    ON email_tokens (user_id, kind);
CREATE INDEX email_tokens_expires_idx ON email_tokens (expires_at);

-- ---------------------------------------------------------------------------
-- 3. Sesi yang bisa dilihat dan dicabut satu per satu
-- ---------------------------------------------------------------------------

-- `id` ada supaya sebuah sesi punya nama yang boleh disebut di URL.
--
-- Primary key tabel ini adalah HASH token-nya, dan hash itu memang tidak bisa
-- dikembalikan jadi token — tapi memakainya sebagai handle publik berarti
-- daftar sesi seseorang membocorkan bahan yang persis dipakai untuk mencari
-- sesi di database. Id terpisah tidak berhubungan dengan kredensial apa pun.
ALTER TABLE sessions
    ADD COLUMN id           uuid NOT NULL DEFAULT gen_random_uuid(),

    -- Keterangan perangkat, disalin saat sesinya lahir. Tanpa ini, daftar sesi
    -- aktif adalah daftar tanggal tanpa satu petunjuk pun tentang mana yang
    -- laptop kantor dan mana yang bukan milik kita.
    ADD COLUMN user_agent   text,

    -- Kapan sesi ini terakhir dipakai. Dimajukan paling sering sekali tiap
    -- beberapa menit, bukan tiap permintaan — lihat Store.UserBySession.
    ADD COLUMN last_seen_at timestamptz;

CREATE UNIQUE INDEX sessions_id_idx ON sessions (id);

-- ---------------------------------------------------------------------------
-- 4. Status
-- ---------------------------------------------------------------------------

ALTER TABLE users
    ADD COLUMN status text NOT NULL DEFAULT 'available'
        CHECK (status IN ('available', 'busy', 'away')),

    -- Teks bebas yang ditampilkan ke orang lain. Batasnya dipasang di database
    -- juga, bukan hanya di handler: tanpa batas dia jadi pesan kedua yang
    -- menyamar jadi status, dan handler adalah satu-satunya jalur yang bisa
    -- dilewati oleh skrip perbaikan data yang ditulis buru-buru.
    ADD COLUMN status_text text NOT NULL DEFAULT ''
        CHECK (char_length(status_text) <= 120),

    -- Instan absolut, bukan "tiga jam lagi".
    --
    -- "Sampai jam 13.00" di jam siapa adalah pertanyaan yang harus punya
    -- jawaban sebelum baris pertama ditulis, dan jawabannya: server menyimpan
    -- titik waktu, client yang merendernya ke jam lokal pembacanya.
    --
    -- Tidak ada job yang membersihkan kolom ini. Pembacaan menyaring sendiri
    -- (`status_expires_at > now()`), dengan alasan yang sama dengan typing
    -- indicator di Fase 3 yang sengaja tidak menyentuh database: keadaan yang
    -- kedaluwarsa dengan sendirinya tidak butuh sesuatu yang berjalan. Penyapu
    -- lintas instance justru menambah masalah — butuh penguncian, dan tiap
    -- instance akan menyiarkan kabar kedaluwarsa yang sama.
    ADD COLUMN status_expires_at timestamptz;

-- Status tanpa teks dan tanpa batas waktu adalah 'available' — keadaan bawaan
-- yang tidak perlu disebut. Index parsial ini hanya memuat yang benar-benar
-- punya sesuatu untuk diceritakan, dan dialah yang dipakai snapshot saat client
-- menyambung: sebuah grup dua ratus orang yang semuanya 'available' tidak
-- menghasilkan satu baris pun untuk dibaca.
CREATE INDEX users_status_aktif_idx ON users (id)
    WHERE status <> 'available' OR status_text <> '';

ANALYZE users;
ANALYZE sessions;
