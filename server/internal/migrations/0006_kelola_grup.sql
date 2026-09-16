-- Kelola grup: ganti judul, tambah/keluarkan anggota, keluar, pindah pemilik.
--
-- Sampai sini sebuah grup hanya bisa DIBUAT. Setelah itu dia beku: tidak ada
-- cara menambah orang, mengeluarkan orang, mengganti judulnya, atau keluar
-- darinya. Fase ini menutup itu — dan satu keputusan menaungi seluruhnya:
--
--   **Perubahan keanggotaan adalah PESAN, bukan sekadar perubahan baris.**
--
-- Godaannya adalah menyiarkan "daftar anggota berubah" lalu selesai. Itu
-- membuat perubahannya tidak punya jejak: orang yang membuka aplikasi besok
-- pagi hanya melihat bahwa jumlah anggotanya berbeda, tanpa tahu siapa yang
-- mengeluarkan siapa, atau kapan.
--
-- Sebuah percakapan SUDAH punya catatan berurutan yang tahan putus koneksi:
-- tabel messages, dengan `seq` yang jadi dasar pagination dan resume sejak Fase
-- 1. Menumpang di sana berarti pesan sistem ikut terbawa oleh SELURUH mesin
-- yang sudah ada — riwayat, cursor, susulan setelah reconnect, urutan yang
-- deterministik — tanpa satu baris pun jalur sinkronisasi baru.
--
-- Bandingkan dengan reaksi di 0005, yang TIDAK bisa menumpang dan karena itu
-- butuh jam kedua: reaksi mengubah pesan lama yang seq-nya sudah berhenti
-- bergerak. Pesan sistem tidak begitu — dia memang kejadian baru, pada saat
-- ini, dan `seq` berikutnya adalah tempat yang tepat untuknya.

-- `kind` memisahkan apa yang ditulis ORANG dari apa yang dicatat SISTEM.
--
-- Pemisahan ini bukan kerapian tampilan. Tanpa dia, pesan sistem ikut memenuhi
-- syarat `sender_id = $aku` pada jalur edit dan hapus — dan seseorang bisa
-- menyunting catatan "Budi mengeluarkan Ani" menjadi kalimat apa pun yang dia
-- mau, lalu catatan itu tetap tampil sebagai keterangan resmi dari sistem.
ALTER TABLE messages
    ADD COLUMN kind text NOT NULL DEFAULT 'user'
        CHECK (kind IN ('user', 'system'));

-- Isi kejadiannya, bukan kalimatnya.
--
-- Yang disimpan adalah {type, actor, targets, title} — bukan "Budi menambahkan
-- Ani". Kalimatnya disusun di client, sehingga bahasanya bisa berubah tanpa
-- menulis ulang riwayat siapa pun.
--
-- Nama orangnya IKUT disalin ke dalam jsonb ini, dan itu disengaja. Orang yang
-- dikeluarkan tidak lagi ada di daftar anggota, jadi client tidak punya tempat
-- untuk mencari namanya — catatan yang berbunyi "Budi mengeluarkan (tidak
-- dikenal)" adalah catatan yang gagal justru pada satu hal yang ingin
-- diketahui orang. Ini juga sejalan dengan aturan salinan di 0005: yang boleh
-- disalin adalah yang tidak pernah berubah, dan sebuah catatan sejarah memang
-- dibekukan pada saat kejadiannya.
ALTER TABLE messages ADD COLUMN system_event jsonb;

-- Keduanya lahir dan mati bersama, pola yang sama dengan kolom turunan gambar
-- di 0004. Pesan sistem tanpa isi kejadian tampil sebagai baris kosong; pesan
-- biasa yang membawa isi kejadian berarti ada jalur kode yang menulis ke kolom
-- yang bukan miliknya.
ALTER TABLE messages
    ADD CONSTRAINT messages_system_event_lengkap CHECK (
        (kind = 'user'   AND system_event IS NULL) OR
        (kind = 'system' AND system_event IS NOT NULL)
    );

-- Urutan keluar-masuk anggota dibaca saat pemiliknya keluar: kepemilikan
-- berpindah ke anggota yang paling lama bergabung. Tanpa index ini, pencarian
-- itu memindai seluruh keanggotaan grup — murah pada grup kecil, dan tidak
-- perlu dibiarkan tumbuh jadi mahal pada grup besar.
CREATE INDEX conversation_members_joined_idx
    ON conversation_members (conversation_id, joined_at);

ANALYZE messages;
ANALYZE conversation_members;
