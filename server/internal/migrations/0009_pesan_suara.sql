-- Fase 13: pesan suara.
--
-- Lampiran audio sudah bisa diunggah dan diputar sejak Fase 7-8. Yang kurang
-- untuk rekaman dari dalam aplikasi cuma satu angka: panjangnya.
--
-- MediaRecorder menulis berkasnya sambil merekam, jadi dia tidak pernah tahu
-- panjang rekamannya saat menulis header. WebM hasil Chrome dan Firefox tidak
-- membawa durasi sama sekali, dan <audio> melaporkan `Infinity` untuknya —
-- pemutar yang tidak bisa menyebut "0:12" sebelum diputar sampai habis.
--
-- Yang tahu panjangnya adalah perekamnya, dan client mengirimnya bersama
-- unggahan, persis seperti ukuran gambar di 0003. Angka ini murni tampilan:
-- tidak ada yang diputuskan server berdasarkan nilainya, dan pemutar tetap
-- memakai durasi dari berkasnya sendiri begitu itu tersedia.
--
-- NULL untuk semua lampiran lain, termasuk berkas audio yang diunggah dari
-- disk — di sana header berkasnya sudah menyebut durasinya sendiri.

ALTER TABLE attachments
    ADD COLUMN duration_ms integer
        CHECK (duration_ms IS NULL OR duration_ms BETWEEN 1 AND 3600000);
