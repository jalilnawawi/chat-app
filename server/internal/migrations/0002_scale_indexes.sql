-- Penyetelan untuk beban ratusan user aktif bersamaan.
--
-- Tiga hal yang diperbaiki di sini, semuanya berasal dari query yang benar-
-- benar dijalankan aplikasi, bukan dari tebakan:
--
--  1. Pencarian user memakai ILIKE '%kata%'. Pola dengan wildcard di DEPAN
--     tidak bisa memakai index btree sama sekali — setiap kali seseorang
--     membuka dialog "chat baru", Postgres membaca seluruh tabel users.
--     Dengan 50 user itu tak terasa; dengan 50.000 itu satu detik penuh yang
--     dibayar berulang-ulang.
--  2. `conversations.last_seq` dan `conversation_members.last_read_seq`
--     di-UPDATE pada hampir setiap pesan. Menyisakan ruang kosong di tiap
--     halaman membuat Postgres bisa menulis versi baris yang baru di halaman
--     yang sama (HOT update) tanpa menyentuh index — jalur tulis terpanas di
--     aplikasi ini.
--  3. Daftar percakapan adalah query yang paling sering dijalankan. Menyertakan
--     kolom yang dibaca ke dalam index membuatnya bisa dijawab tanpa membuka
--     tabel sama sekali.

-- ---------------------------------------------------------------------------
-- 1. Pencarian user
-- ---------------------------------------------------------------------------

-- pg_trgm memecah teks jadi trigram sehingga GIN bisa melayani LIKE/ILIKE
-- berpola bebas. Pada Postgres terkelola, extension kadang harus diaktifkan
-- lebih dulu oleh administrator; kalau migrasi ini gagal di sana, itu sebabnya.
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX users_username_trgm_idx
    ON users USING gin (username gin_trgm_ops);

CREATE INDEX users_display_name_trgm_idx
    ON users USING gin (display_name gin_trgm_ops);

-- ---------------------------------------------------------------------------
-- 2. Ruang untuk HOT update di jalur tulis terpanas
-- ---------------------------------------------------------------------------

-- fillfactor hanya berlaku untuk halaman yang ditulis SETELAH ini, jadi
-- tabel yang sudah berisi perlu ditulis ulang sekali. Di sini masih murah;
-- pada tabel besar, VACUUM FULL mengunci tabel dan harus dijadwalkan.
ALTER TABLE conversations        SET (fillfactor = 85);
ALTER TABLE conversation_members SET (fillfactor = 85);

-- Baris percakapan yang ramai menghasilkan versi mati jauh lebih cepat
-- daripada rata-rata tabel. Ambang autovacuum bawaan (20% dari tabel) terlalu
-- longgar untuk tabel yang barisnya sedikit tapi ditulis terus-menerus.
ALTER TABLE conversations        SET (autovacuum_vacuum_scale_factor = 0.02);
ALTER TABLE conversation_members SET (autovacuum_vacuum_scale_factor = 0.05);

-- ---------------------------------------------------------------------------
-- 3. Daftar percakapan tanpa menyentuh tabel
-- ---------------------------------------------------------------------------

-- INCLUDE menempelkan kolom yang dibaca tapi tidak dicari. Query daftar
-- percakapan jadi index-only scan: satu pembacaan index, nol pembacaan heap.
CREATE INDEX conversation_members_user_covering_idx
    ON conversation_members (user_id) INCLUDE (conversation_id, last_read_seq);

DROP INDEX conversation_members_user_idx;

-- ---------------------------------------------------------------------------
-- 4. Pemeriksaan foreign key
-- ---------------------------------------------------------------------------

-- messages.sender_id menunjuk users tanpa index pendamping. Selama tidak ada
-- user yang dihapus ini tak terasa, tapi satu DELETE user memaksa pemindaian
-- penuh tabel messages sambil memegang kunci.
CREATE INDEX messages_sender_idx ON messages (sender_id);

-- Statistik lama membuat planner memilih rencana untuk tabel kosong sekalipun
-- datanya sudah banyak.
ANALYZE users;
ANALYZE conversations;
ANALYZE conversation_members;
ANALYZE messages;
