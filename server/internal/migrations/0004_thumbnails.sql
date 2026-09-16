-- Fase 8: turunan kecil untuk gambar.
--
-- Foto dua belas megapiksel dari ponsel sampai di sini sebagai berkas beberapa
-- megabyte, lalu ditampilkan selebar tiga ratus piksel di dalam gelembung
-- pesan. Sebelum kolom-kolom ini ada, seluruh berkas itu tetap harus diunduh
-- utuh untuk menghasilkan gambar sebesar kuku ibu jari — dan diunduh ulang oleh
-- setiap anggota percakapan.
--
-- Turunannya disimpan sebagai OBJEK TERSENDIRI di penyimpanan, bukan sebagai
-- bytea di sini. Alasannya sama persis dengan alasan berkas aslinya tidak
-- disimpan di Postgres (lihat 0003): tabel ini ikut dibaca di jalur panas, dan
-- setiap byte gambar yang menumpang di dalamnya masuk ke WAL, ikut direplikasi,
-- dan ikut tersalin tiap kali basis datanya dicadangkan.
--
-- Yang disimpan di sini hanya penunjuknya. Kolomnya NULL berarti lampiran ini
-- tidak punya turunan — bukan gambar, terlalu kecil untuk diperkecil, atau
-- pembuatannya memang gagal. Ketiganya sah, dan ketiganya berakhir sama:
-- client memakai berkas aslinya.

ALTER TABLE attachments
    -- Path turunan di filer. Bentuknya sama dengan storage_key dan hidup di
    -- direktori yang sama, hanya dengan akhiran yang membedakan.
    ADD COLUMN thumb_key  text,

    -- Tipe turunan TIDAK sama dengan tipe aslinya: yang kita hasilkan sendiri
    -- selalu JPEG atau PNG, apa pun tipe masukannya. Disimpan, bukan ditebak
    -- dari ekstensi, karena tipe yang disebut saat menyajikan berkas tidak
    -- pernah boleh berasal dari tebakan.
    ADD COLUMN thumb_mime text,

    -- Dipakai mengisi Content-Length tanpa menanyai penyimpanan lebih dulu,
    -- dan untuk menjawab "seberapa besar penghematannya" tanpa memindai filer.
    ADD COLUMN thumb_size bigint CHECK (thumb_size IS NULL OR thumb_size >= 0);

-- Ketiganya lahir dan mati bersama. Satu kolom terisi sementara dua lainnya
-- kosong berarti ada jalur kode yang menulis separuh, dan akibatnya adalah
-- turunan yang disajikan tanpa tipe — persis keadaan yang membuat browser
-- kembali menebak sendiri isi berkas.
ALTER TABLE attachments
    ADD CONSTRAINT attachments_thumb_lengkap CHECK (
        (thumb_key IS NULL AND thumb_mime IS NULL AND thumb_size IS NULL)
     OR (thumb_key IS NOT NULL AND thumb_mime IS NOT NULL AND thumb_size IS NOT NULL)
    );

-- Penyapu lampiran yatim harus ikut membuang turunannya, jadi thumb_key ikut
-- terbaca di sana. Tidak ada index baru: turunan selalu dicari lewat id
-- lampirannya, yang sudah jadi primary key.

ANALYZE attachments;
