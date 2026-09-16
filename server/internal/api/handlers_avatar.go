package api

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/jalilnawawi/chat-app/server/internal/blob"
	"github.com/jalilnawawi/chat-app/server/internal/imaging"
	"github.com/jalilnawawi/chat-app/server/internal/store"
)

// Foto profil.
//
// Dua hal yang membedakannya dari lampiran, dan keduanya disengaja:
//
//  1. **Berkas aslinya DIBUANG.** Yang tersimpan cuma hasil perkecilannya.
//     Tidak ada yang butuh avatar dua belas megapiksel, dan menyimpannya berarti
//     membayar selamanya untuk sesuatu yang selalu ditampilkan selebar empat
//     puluh piksel. Karena itu, perkecilannya di sini WAJIB — kegagalannya
//     menggagalkan unggahannya, berbeda dari turunan lampiran di Fase 8 yang
//     boleh tidak ada.
//
//  2. **Izin bacanya lebih longgar.** Siapa pun yang sudah login boleh
//     melihatnya. Alasannya ditulis di store.AvatarForRead.

// avatarThumbWait adalah berapa lama unggahan avatar mau menunggu giliran
// mendekode gambar.
//
// Lebih panjang dari thumbWait milik lampiran, dan itu karena akibat habisnya
// waktu berbeda: di sana lampirannya tetap tersimpan tanpa turunan, di sini
// tidak ada apa-apa yang tersimpan sama sekali. Menunggu lebih lama adalah
// harga yang pantas untuk sesuatu yang dilakukan orang beberapa kali seumur
// akun.
const avatarThumbWait = 10 * time.Second

func (s *Server) handleUploadAvatar(w http.ResponseWriter, r *http.Request) {
	me := userFrom(r.Context())

	if s.blobs == nil {
		writeError(w, http.StatusServiceUnavailable, "foto profil tidak aktif di server ini")
		return
	}
	// Turunan dimatikan berarti tidak ada cara memperkecil, dan tanpa
	// perkecilan tidak ada avatar — lihat keputusan pertama di atas.
	if s.thumbSem == nil {
		writeError(w, http.StatusServiceUnavailable,
			"foto profil butuh pengolahan gambar, yang sedang dimatikan di server ini")
		return
	}

	// Batasnya dipasang pada BODY, bukan diperiksa setelah berkasnya masuk —
	// alasan yang sama dengan jalur lampiran. Angkanya sendiri jauh lebih kecil:
	// yang masuk ke sini SELALU dibentangkan di memori.
	r.Body = http.MaxBytesReader(w, r.Body, s.cfg.AvatarMaxBytes+(64<<10))

	// Nama berkasnya sengaja dibuang. Avatar tidak punya nama yang pernah
	// ditampilkan atau dikirim balik saat diunduh — berbeda dari lampiran, yang
	// memang disimpan namanya — jadi satu-satunya hal yang bisa dilakukan nama
	// kiriman di sini adalah jadi bahan yang harus dibersihkan.
	part, _, err := firstFilePart(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	defer part.Close() //nolint:errcheck

	// Seluruh berkas dibaca ke memori, dan di sini itu bukan kompromi melainkan
	// satu-satunya cara: tidak ada cara memperkecil sebuah gambar sambil hanya
	// melihat sepotong kecilnya. Yang menjaga ini tetap terbatas adalah
	// MaxBytesReader di atas.
	src, err := io.ReadAll(part)
	if err != nil {
		s.writeAvatarError(w, err)
		return
	}
	if len(src) == 0 {
		writeError(w, http.StatusBadRequest, "berkasnya kosong")
		return
	}

	// Tipe ditentukan dari ISINYA, bukan dari yang diakui client — aturan yang
	// sama dengan lampiran, dan di sini dia bahkan lebih ketat: hanya empat tipe
	// gambar yang boleh dirender inline yang diterima sama sekali.
	contentType := http.DetectContentType(src)
	if !inlineTypes[contentType] {
		writeError(w, http.StatusBadRequest, "foto profil harus berupa gambar PNG, JPEG, GIF, atau WebP")
		return
	}

	thumb, err := s.shrinkAvatar(r.Context(), src)
	if err != nil {
		s.log.Warn("memperkecil foto profil gagal", "user", me.ID, "mime", contentType, "err", err)
		s.m.Thumbnails.WithLabelValues("gagal").Inc()
		writeError(w, http.StatusBadRequest, "gambarnya tidak bisa dibaca sebagai foto")
		return
	}

	id, err := uuid.NewV7()
	if err != nil {
		s.writeStoreError(w, err, "id avatar")
		return
	}
	key := avatarStorageKey(id, thumb.MIME)

	putCtx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Byte dulu, baru barisnya — urutan yang sama dengan lampiran di Fase 7.
	// Yang tertinggal saat proses mati di antara keduanya adalah berkas tanpa
	// baris: sampah diam yang tidak dilihat siapa pun, bukan foto rusak di layar
	// orang.
	if err := s.blobs.Put(putCtx, key, thumb.MIME, bytes.NewReader(thumb.Bytes), int64(len(thumb.Bytes))); err != nil {
		s.deleteBlobQuietly(key)
		s.writeAvatarError(w, err)
		return
	}

	user, err := s.store.SetAvatar(r.Context(), me.ID, store.StoredAvatar{
		ID: id, Key: key, MIME: thumb.MIME, Size: int64(len(thumb.Bytes)),
	})
	if err != nil {
		s.deleteBlobQuietly(key)
		s.writeStoreError(w, err, "pasang avatar")
		return
	}

	s.m.Thumbnails.WithLabelValues("ok").Inc()
	s.m.ThumbnailBytes.Add(float64(len(thumb.Bytes)))

	// Tanpa siaran ini, foto baru memang punya alamat baru — tapi tidak seorang
	// pun tahu alamat itu sampai halamannya dimuat ulang.
	s.broadcastUser(r, user)
	writeJSON(w, http.StatusCreated, s.meOf(user, me))
}

func (s *Server) handleRemoveAvatar(w http.ResponseWriter, r *http.Request) {
	me := userFrom(r.Context())

	user, err := s.store.RemoveAvatar(r.Context(), me.ID)
	if err != nil {
		s.writeStoreError(w, err, "lepas avatar")
		return
	}

	s.broadcastUser(r, user)
	writeJSON(w, http.StatusOK, s.meOf(user, me))
}

// shrinkAvatar memperkecil gambar di bawah pembatas yang sama dengan turunan
// lampiran.
//
// Pembatasnya dipakai bersama dengan sengaja: yang dijaga adalah MEMORI proses
// ini, dan dua penjaga terpisah untuk satu sumber daya berarti batas
// sebenarnya adalah jumlah keduanya — angka yang tidak pernah dipilih siapa pun.
func (s *Server) shrinkAvatar(ctx context.Context, src []byte) (imaging.Thumbnail, error) {
	select {
	case s.thumbSem <- struct{}{}:
		defer func() { <-s.thumbSem }()
	case <-time.After(avatarThumbWait):
		s.m.Thumbnails.WithLabelValues("sibuk").Inc()
		return imaging.Thumbnail{}, errors.New("server sedang sibuk mengolah gambar")
	case <-ctx.Done():
		return imaging.Thumbnail{}, ctx.Err()
	}

	start := time.Now()
	// Normalize, bukan Make: yang kedua menolak bekerja untuk gambar yang sudah
	// kecil, dan di jalur ini penolakan itu berarti tidak ada apa pun yang
	// tersimpan. Lihat catatannya di paket imaging.
	thumb, err := imaging.Normalize(src, s.cfg.AvatarMaxDim, s.cfg.ThumbMaxPixels)
	s.m.ThumbnailSeconds.Observe(time.Since(start).Seconds())
	return thumb, err
}

// handleDownloadAvatar menyajikan byte foto profil.
//
// Izinnya cuma "sudah login", dan itu ditulis sebagai satu baris tanpa
// pemeriksaan tambahan justru supaya tidak ada yang mengira ada pemeriksaan
// lain yang kebetulan terlewat. Alasannya di store.AvatarForRead.
func (s *Server) handleDownloadAvatar(w http.ResponseWriter, r *http.Request) {
	if s.blobs == nil {
		writeError(w, http.StatusServiceUnavailable, "foto profil tidak aktif di server ini")
		return
	}

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "id avatar tidak valid")
		return
	}

	av, err := s.store.AvatarForRead(r.Context(), id)
	if err != nil {
		s.writeStoreError(w, err, "baca avatar")
		return
	}

	obj, err := s.blobs.Get(r.Context(), av.Key, nil)
	if errors.Is(err, blob.ErrNotFound) {
		s.log.Error("isi avatar tidak ada di penyimpanan", "avatar", id, "key", av.Key)
		writeError(w, http.StatusNotFound, "isi foto tidak ditemukan")
		return
	}
	if err != nil {
		s.log.Error("ambil avatar", "avatar", id, "err", err)
		writeError(w, http.StatusBadGateway, "penyimpanan tidak bisa dihubungi")
		return
	}
	defer obj.Body.Close() //nolint:errcheck

	w.Header().Set("Content-Type", av.MIME)
	w.Header().Set("Content-Length", strconv.FormatInt(av.Size, 10))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")

	// Cache setahun yang bertanda immutable, dan di sini dia BENAR justru karena
	// alamatnya memuat id unggahan: byte di balik alamat ini tidak akan pernah
	// berubah, karena mengganti foto menghasilkan alamat yang lain sama sekali.
	// Itulah seluruh alasan `avatar_id` ada — lihat store.AvatarURL.
	//
	// "private", sama seperti lampiran: cache bersama (proxy perusahaan, CDN)
	// tidak ikut menyimpannya, karena membacanya tetap menuntut sesi.
	w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")

	if _, err := io.Copy(w, obj.Body); err != nil {
		s.log.Debug("unduhan avatar terputus", "avatar", id, "err", err)
	}
}

// avatarStorageKey menyusun tempat penyimpanan dari id unggahannya, BUKAN dari
// id penggunanya.
//
// Nama yang memuat id pengguna berarti foto lama dan foto baru berbagi alamat
// penyimpanan, dan dengan itu seluruh keputusan "alamatnya harus berubah setiap
// fotonya berubah" runtuh satu lapis di bawah tempat dia dibuat.
func avatarStorageKey(id uuid.UUID, mime string) string {
	return "avatars/" + id.String() + commonExtensions[mime]
}

func (s *Server) writeAvatarError(w http.ResponseWriter, err error) {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		writeError(w, http.StatusRequestEntityTooLarge,
			"foto terlalu besar, maksimal "+strconv.FormatInt(s.cfg.AvatarMaxBytes>>20, 10)+" MB")
		return
	}
	s.log.Error("unggah foto profil", "err", err)
	writeError(w, http.StatusBadGateway, "penyimpanan tidak bisa dihubungi")
}
