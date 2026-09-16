package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jalilnawawi/chat-app/server/internal/store"
)

// Test untuk jalur Fase 10: foto profil, email, password, sesi, dan status.
//
// Yang paling banyak diuji di sini adalah hal yang paling sulit dilihat dengan
// membuka halamannya: status yang KEDALUWARSA. Dia tidak dibersihkan oleh apa
// pun, jadi satu-satunya yang membuatnya tidak terlihat adalah penyaring di
// dalam SQL — dan penyaring yang terlewat di satu jalur baca tidak akan pernah
// mengeluh, dia cuma menampilkan "sedang rapat sampai 13.00" pada pukul empat
// sore.

// ---------- status ----------

func TestStatusYangSudahLewatDibacaSebagaiAvailable(t *testing.T) {
	s, ctx := newStore(t)
	me := newUser(t, s, ctx, "Rapat")
	pencari := newUser(t, s, ctx, "Pencari")

	// Dipasang dengan batas waktu yang masih berlaku, lalu dimundurkan langsung
	// di database. Memasangnya langsung kedaluwarsa akan ditolak SetStatus —
	// dan penolakan itu sendiri adalah perilaku yang benar, diuji terpisah.
	if _, err := s.SetStatus(ctx, me.ID, store.StatusBusy, "sedang rapat",
		ptr(time.Now().Add(time.Hour))); err != nil {
		t.Fatalf("pasang status: %v", err)
	}
	mundurkanStatus(t, me.ID, -time.Minute)

	found, err := s.SearchUsers(ctx, me.Username, pencari.ID, 10)
	if err != nil {
		t.Fatalf("cari pengguna: %v", err)
	}
	if len(found) != 1 {
		t.Fatalf("mau 1 hasil, dapat %d", len(found))
	}
	if found[0].Status != store.StatusAvailable {
		t.Errorf("status kedaluwarsa masih terbaca sebagai %q", found[0].Status)
	}
	if found[0].StatusText != "" {
		t.Errorf("teks status kedaluwarsa masih terbaca: %q", found[0].StatusText)
	}
	if found[0].StatusExpiresAt != nil {
		t.Errorf("batas waktu yang sudah lewat masih ikut terkirim")
	}
}

// Jalur baca kedua untuk hal yang sama. Sengaja diuji terpisah dari pencarian:
// lawan bicara sebuah DM datang lewat LEFT JOIN LATERAL, yaitu satu-satunya
// tempat daftar kolom pengguna pernah salah panjang.
func TestStatusPeerIkutTersaringDiDaftarPercakapan(t *testing.T) {
	s, ctx := newStore(t)
	a, b := newUser(t, s, ctx, "Ani"), newUser(t, s, ctx, "Budi")
	newDirect(t, s, ctx, a, b)

	if _, err := s.SetStatus(ctx, b.ID, store.StatusBusy, "rapat", ptr(time.Now().Add(time.Hour))); err != nil {
		t.Fatalf("pasang status: %v", err)
	}

	convs, err := s.ListConversations(ctx, a.ID)
	if err != nil {
		t.Fatalf("daftar percakapan: %v", err)
	}
	if len(convs) != 1 || convs[0].Peer == nil {
		t.Fatalf("lawan bicara tidak terbaca")
	}
	if convs[0].Peer.Status != store.StatusBusy || convs[0].Peer.StatusText != "rapat" {
		t.Fatalf("status peer salah: %+v", convs[0].Peer)
	}

	mundurkanStatus(t, b.ID, -time.Minute)

	convs, err = s.ListConversations(ctx, a.ID)
	if err != nil {
		t.Fatalf("daftar percakapan kedua: %v", err)
	}
	if convs[0].Peer.Status != store.StatusAvailable || convs[0].Peer.StatusText != "" {
		t.Errorf("status peer yang sudah lewat masih terbaca: %+v", convs[0].Peer)
	}
}

// Snapshot hanya memuat yang punya sesuatu untuk diceritakan. Tanpa syarat itu,
// sebuah akun dengan dua ratus kontak menerima dua ratus baris yang tidak
// mengubah apa pun di layarnya setiap kali dia menyambung.
func TestSnapshotStatusHanyaMemuatYangBukanBawaan(t *testing.T) {
	s, ctx := newStore(t)
	diam := newUser(t, s, ctx, "Diam")
	sibuk := newUser(t, s, ctx, "Sibuk")
	berpesan := newUser(t, s, ctx, "Berpesan")
	lewat := newUser(t, s, ctx, "Lewat")

	if _, err := s.SetStatus(ctx, sibuk.ID, store.StatusBusy, "", nil); err != nil {
		t.Fatalf("pasang busy: %v", err)
	}
	// 'available' TAPI berteks: tetap harus ikut, karena teksnya adalah sesuatu
	// yang dinyatakan orangnya dengan sengaja.
	if _, err := s.SetStatus(ctx, berpesan.ID, store.StatusAvailable, "wfh hari ini", nil); err != nil {
		t.Fatalf("pasang teks: %v", err)
	}
	if _, err := s.SetStatus(ctx, lewat.ID, store.StatusAway, "cuti", ptr(time.Now().Add(time.Hour))); err != nil {
		t.Fatalf("pasang away: %v", err)
	}
	mundurkanStatus(t, lewat.ID, -time.Minute)

	got, err := s.StatusesOf(ctx, []uuid.UUID{diam.ID, sibuk.ID, berpesan.ID, lewat.ID})
	if err != nil {
		t.Fatalf("snapshot status: %v", err)
	}

	punya := map[uuid.UUID]store.UserStatus{}
	for _, st := range got {
		punya[st.UserID] = st
	}
	if _, ada := punya[diam.ID]; ada {
		t.Error("yang available tanpa teks ikut terkirim")
	}
	if _, ada := punya[lewat.ID]; ada {
		t.Error("status yang sudah lewat waktunya ikut terkirim")
	}
	if punya[sibuk.ID].Status != store.StatusBusy {
		t.Error("yang busy tidak ikut terkirim")
	}
	if punya[berpesan.ID].Text != "wfh hari ini" {
		t.Error("yang available berteks tidak ikut terkirim")
	}
}

// BusyAmong adalah sambungan status ke peredam dering Fase 7. Yang kedaluwarsa
// tidak boleh meredam apa pun: notifikasi yang hilang karena rapat yang sudah
// selesai dua jam lalu adalah kabar yang hilang tanpa sebab.
func TestBusyAmongTidakMemuatYangSudahLewat(t *testing.T) {
	s, ctx := newStore(t)
	sekarang := newUser(t, s, ctx, "Sibuk")
	tadi := newUser(t, s, ctx, "Tadi")
	santai := newUser(t, s, ctx, "Santai")

	for _, u := range []store.User{sekarang, tadi} {
		if _, err := s.SetStatus(ctx, u.ID, store.StatusBusy, "", ptr(time.Now().Add(time.Hour))); err != nil {
			t.Fatalf("pasang busy: %v", err)
		}
	}
	mundurkanStatus(t, tadi.ID, -time.Minute)

	got, err := s.BusyAmong(ctx, []uuid.UUID{sekarang.ID, tadi.ID, santai.ID})
	if err != nil {
		t.Fatalf("saring yang sibuk: %v", err)
	}
	if len(got) != 1 || got[0] != sekarang.ID {
		t.Fatalf("mau hanya yang masih sibuk, dapat %v", got)
	}
}

func TestSetStatusMenolakMasukanYangTidakMasukAkal(t *testing.T) {
	s, ctx := newStore(t)
	me := newUser(t, s, ctx, "Uji")

	panjang := make([]rune, 121)
	for i := range panjang {
		panjang[i] = 'a'
	}

	kasus := []struct {
		nama    string
		status  string
		teks    string
		expires *time.Time
	}{
		{"status yang tidak dikenal", "menghilang", "", nil},
		{"teks melewati batas", store.StatusBusy, string(panjang), nil},
		// Batas waktu yang sudah lewat akan langsung disaring habis oleh
		// pembacaan, jadi menyimpannya berarti menyimpan sesuatu yang tidak akan
		// pernah terlihat — dan orangnya akan mengira fiturnya rusak.
		{"batas waktu sudah lewat", store.StatusBusy, "", ptr(time.Now().Add(-time.Minute))},
	}

	for _, k := range kasus {
		t.Run(k.nama, func(t *testing.T) {
			if _, err := s.SetStatus(ctx, me.ID, k.status, k.teks, k.expires); !errors.Is(err, store.ErrInvalid) {
				t.Fatalf("mau ErrInvalid, dapat %v", err)
			}
		})
	}
}

// ---------- foto profil ----------

// Keputusan yang paling mudah dilanggar tanpa sadar: alamat avatar HARUS
// berubah setiap fotonya berubah. Alamat tetap berarti cache setahun yang
// dipasang di jalur unduhnya menyimpan foto lama selama setahun.
func TestAlamatAvatarBerubahSetiapFotoDiganti(t *testing.T) {
	s, ctx := newStore(t)
	me := newUser(t, s, ctx, "Berfoto")

	pertama := avatarPalsu()
	kedua := avatarPalsu()

	a, err := s.SetAvatar(ctx, me.ID, pertama)
	if err != nil {
		t.Fatalf("pasang avatar pertama: %v", err)
	}
	b, err := s.SetAvatar(ctx, me.ID, kedua)
	if err != nil {
		t.Fatalf("pasang avatar kedua: %v", err)
	}

	if a.AvatarURL == "" || a.AvatarURL == b.AvatarURL {
		t.Fatalf("alamat avatar tidak berubah: %q lalu %q", a.AvatarURL, b.AvatarURL)
	}
	if b.AvatarURL != store.AvatarURL(kedua.ID) {
		t.Errorf("alamat tidak menunjuk id unggahan terbaru: %q", b.AvatarURL)
	}
}

// Foto lama dititipkan ke penyapu, bukan dihapus di tempat. Yang diuji di sini
// adalah titipannya: tanpa baris itu, kuncinya tidak pernah disebut lagi oleh
// siapa pun dan byte-nya tinggal di penyimpanan selamanya.
func TestAvatarLamaDititipkanKePenyapu(t *testing.T) {
	s, ctx := newStore(t)
	me := newUser(t, s, ctx, "Berganti")

	lama, baru := avatarPalsu(), avatarPalsu()

	// Titipan yang ditinggalkan test lain dibuang lebih dulu. Seluruh test di
	// paket ini berbagi satu schema, dan blob_garbage adalah satu-satunya tabel
	// di sini yang dibaca dengan cara "ambil semuanya" — jadi dia satu-satunya
	// yang bisa melihat sisa pekerjaan tetangganya.
	ambilSampah(t, s, ctx)

	if _, err := s.SetAvatar(ctx, me.ID, lama); err != nil {
		t.Fatalf("pasang avatar lama: %v", err)
	}
	// Foto PERTAMA tidak menitipkan apa pun: tidak ada yang digantikan.
	if keys := ambilSampah(t, s, ctx); len(keys) != 0 {
		t.Fatalf("foto pertama menitipkan sampah: %v", keys)
	}

	if _, err := s.SetAvatar(ctx, me.ID, baru); err != nil {
		t.Fatalf("pasang avatar baru: %v", err)
	}
	if keys := ambilSampah(t, s, ctx); len(keys) != 1 || keys[0] != lama.Key {
		t.Fatalf("mau titipan %q, dapat %v", lama.Key, keys)
	}

	// Melepas foto menitipkan yang sedang terpasang, lewat jalur yang sama.
	if _, err := s.RemoveAvatar(ctx, me.ID); err != nil {
		t.Fatalf("lepas avatar: %v", err)
	}
	if keys := ambilSampah(t, s, ctx); len(keys) != 1 || keys[0] != baru.Key {
		t.Fatalf("mau titipan %q, dapat %v", baru.Key, keys)
	}

	user, err := s.SearchUsers(ctx, me.Username, uuid.New(), 1)
	if err != nil {
		t.Fatalf("cari pengguna: %v", err)
	}
	if len(user) != 1 || user[0].AvatarURL != "" {
		t.Errorf("avatar masih terpasang setelah dilepas: %+v", user)
	}
}

func TestAvatarDibacaLewatIdUnggahannya(t *testing.T) {
	s, ctx := newStore(t)
	me := newUser(t, s, ctx, "Berfoto")

	av := avatarPalsu()
	if _, err := s.SetAvatar(ctx, me.ID, av); err != nil {
		t.Fatalf("pasang avatar: %v", err)
	}

	got, err := s.AvatarForRead(ctx, av.ID)
	if err != nil {
		t.Fatalf("baca avatar: %v", err)
	}
	if got.Key != av.Key || got.MIME != av.MIME || got.Size != av.Size {
		t.Fatalf("isi avatar tidak cocok: %+v", got)
	}

	// Id lama tidak menunjuk apa pun setelah fotonya diganti — itulah yang
	// membuat cache setahun tetap aman.
	if _, err := s.SetAvatar(ctx, me.ID, avatarPalsu()); err != nil {
		t.Fatalf("ganti avatar: %v", err)
	}
	if _, err := s.AvatarForRead(ctx, av.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("mau ErrNotFound untuk id avatar lama, dapat %v", err)
	}
}

// ---------- email ----------

// Aturan yang menentukan seluruh jalur pemulihan: mengganti alamat MENURUNKAN
// keadaan verifikasinya. Tanpa itu, seseorang bisa memverifikasi alamatnya
// sendiri lalu menggantinya dengan alamat orang lain yang ikut terbawa
// "terverifikasi".
func TestGantiEmailMenurunkanVerifikasinya(t *testing.T) {
	s, ctx := newStore(t)
	me := newUser(t, s, ctx, "Berlangganan")

	pertama := uuid.NewString() + "@contoh.test"
	if err := s.SetEmail(ctx, me.ID, pertama); err != nil {
		t.Fatalf("pasang email: %v", err)
	}
	token := buatToken(t, s, ctx, me.ID, "verify", pertama, time.Hour)
	if _, err := s.VerifyEmail(ctx, token); err != nil {
		t.Fatalf("verifikasi: %v", err)
	}
	if _, _, err := s.UserByVerifiedEmail(ctx, pertama); err != nil {
		t.Fatalf("alamat terverifikasi tidak ditemukan: %v", err)
	}

	kedua := uuid.NewString() + "@contoh.test"
	if err := s.SetEmail(ctx, me.ID, kedua); err != nil {
		t.Fatalf("ganti email: %v", err)
	}
	if _, _, err := s.UserByVerifiedEmail(ctx, kedua); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("alamat baru langsung dianggap terverifikasi: %v", err)
	}
	if _, _, err := s.UserByVerifiedEmail(ctx, pertama); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("alamat lama masih dianggap terverifikasi: %v", err)
	}
}

// Tautan yang dikirim ke alamat A tidak boleh memverifikasi alamat B. Tanpa
// pemeriksaan ini, membuktikan kepemilikan sebuah alamat berubah jadi
// membuktikan kepemilikan alamat apa pun yang diketik sesudahnya.
func TestTautanVerifikasiTidakBerlakuUntukAlamatYangSudahDiganti(t *testing.T) {
	s, ctx := newStore(t)
	me := newUser(t, s, ctx, "Berpindah")

	lama := uuid.NewString() + "@contoh.test"
	if err := s.SetEmail(ctx, me.ID, lama); err != nil {
		t.Fatalf("pasang email: %v", err)
	}
	token := buatToken(t, s, ctx, me.ID, "verify", lama, time.Hour)

	if err := s.SetEmail(ctx, me.ID, uuid.NewString()+"@contoh.test"); err != nil {
		t.Fatalf("ganti email: %v", err)
	}

	if _, err := s.VerifyEmail(ctx, token); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("mau ErrConflict, dapat %v", err)
	}
}

func TestTokenVerifikasiHanyaSekaliPakai(t *testing.T) {
	s, ctx := newStore(t)
	me := newUser(t, s, ctx, "Sekali")

	email := uuid.NewString() + "@contoh.test"
	if err := s.SetEmail(ctx, me.ID, email); err != nil {
		t.Fatalf("pasang email: %v", err)
	}
	token := buatToken(t, s, ctx, me.ID, "verify", email, time.Hour)

	if _, err := s.VerifyEmail(ctx, token); err != nil {
		t.Fatalf("verifikasi pertama: %v", err)
	}
	if _, err := s.VerifyEmail(ctx, token); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("token yang sama masih berlaku kedua kali: %v", err)
	}
}

func TestEmailYangSudahDipakaiAkunLainDitolak(t *testing.T) {
	s, ctx := newStore(t)
	a, b := newUser(t, s, ctx, "Duluan"), newUser(t, s, ctx, "Belakangan")

	email := uuid.NewString() + "@contoh.test"
	if err := s.SetEmail(ctx, a.ID, email); err != nil {
		t.Fatalf("pasang email pertama: %v", err)
	}
	if err := s.SetEmail(ctx, b.ID, email); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("mau ErrConflict, dapat %v", err)
	}
	// Huruf besar-kecil tidak membuatnya jadi alamat yang berbeda.
	if err := s.SetEmail(ctx, b.ID, upper(email)); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("mau ErrConflict untuk alamat beda kapitalisasi, dapat %v", err)
	}
}

func TestNormalizeEmailMenolakBentukYangJelasSalah(t *testing.T) {
	for _, raw := range []string{"", "tanpa-at", "a@b", "@contoh.test", "a@", "a b@contoh.test"} {
		if _, err := store.NormalizeEmail(raw); !errors.Is(err, store.ErrInvalid) {
			t.Errorf("%q: mau ErrInvalid, dapat %v", raw, err)
		}
	}
	if got, err := store.NormalizeEmail("  Budi+tag@Contoh.Test  "); err != nil || got != "Budi+tag@Contoh.Test" {
		t.Errorf("alamat sah ditolak atau berubah: %q, %v", got, err)
	}
}

// ---------- password & sesi ----------

// Ganti password mencabut semua sesi LAIN, dan menyisakan yang sedang dipakai.
// Mencabut semuanya berarti orang yang baru saja mengamankan akunnya langsung
// dikeluarkan dari layar yang sedang dia buka.
func TestGantiPasswordMencabutSesiLainSaja(t *testing.T) {
	s, ctx := newStore(t)
	me := newUser(t, s, ctx, "Berganti")

	ini := buatSesi(t, s, ctx, me.ID, "laptop")
	buatSesi(t, s, ctx, me.ID, "ponsel")
	buatSesi(t, s, ctx, me.ID, "asing")

	if err := s.ChangePassword(ctx, me.ID, "hash-baru", ini); err != nil {
		t.Fatalf("ganti password: %v", err)
	}

	sesi, err := s.Sessions(ctx, me.ID, uuid.Nil)
	if err != nil {
		t.Fatalf("daftar sesi: %v", err)
	}
	if len(sesi) != 1 {
		t.Fatalf("mau 1 sesi tersisa, dapat %d", len(sesi))
	}
	if sesi[0].UserAgent != "laptop" {
		t.Errorf("sesi yang tersisa bukan yang dipakai: %q", sesi[0].UserAgent)
	}

	hash, err := s.PasswordHashOf(ctx, me.ID)
	if err != nil || hash != "hash-baru" {
		t.Fatalf("password tidak tersimpan: %q, %v", hash, err)
	}
}

// Pemulihan mencabut SEMUA sesi tanpa kecuali: orang yang sampai ke jalur itu
// tidak sedang memegang sesi mana pun yang layak dipercaya.
func TestPulihkanPasswordMencabutSemuaSesi(t *testing.T) {
	s, ctx := newStore(t)
	me := newUser(t, s, ctx, "Lupa")

	buatSesi(t, s, ctx, me.ID, "laptop")
	buatSesi(t, s, ctx, me.ID, "ponsel")

	email := uuid.NewString() + "@contoh.test"
	token := buatToken(t, s, ctx, me.ID, "reset", email, time.Hour)

	got, err := s.ResetPassword(ctx, token, "hash-pulih")
	if err != nil {
		t.Fatalf("pulihkan password: %v", err)
	}
	if got != me.ID {
		t.Fatalf("token menunjuk orang yang salah")
	}

	sesi, err := s.Sessions(ctx, me.ID, uuid.Nil)
	if err != nil {
		t.Fatalf("daftar sesi: %v", err)
	}
	if len(sesi) != 0 {
		t.Fatalf("masih ada %d sesi setelah pemulihan", len(sesi))
	}
	if _, err := s.ResetPassword(ctx, token, "hash-lagi"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("token pemulihan masih berlaku kedua kali: %v", err)
	}
}

func TestTokenPemulihanKedaluwarsaDitolak(t *testing.T) {
	s, ctx := newStore(t)
	me := newUser(t, s, ctx, "Terlambat")

	token := buatToken(t, s, ctx, me.ID, "reset", "a@contoh.test", -time.Minute)
	if _, err := s.ResetPassword(ctx, token, "hash"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("mau ErrNotFound, dapat %v", err)
	}
}

// Ganti password juga membuang tautan pemulihan yang masih menggantung: tautan
// yang masih berlaku di kotak masuk adalah jalan masuk kedua yang tidak diminta
// siapa pun.
func TestGantiPasswordMembuangTautanPemulihanYangMenggantung(t *testing.T) {
	s, ctx := newStore(t)
	me := newUser(t, s, ctx, "Ragu")

	token := buatToken(t, s, ctx, me.ID, "reset", "a@contoh.test", time.Hour)
	if err := s.ChangePassword(ctx, me.ID, "hash-baru", buatSesi(t, s, ctx, me.ID, "laptop")); err != nil {
		t.Fatalf("ganti password: %v", err)
	}
	if _, err := s.ResetPassword(ctx, token, "hash-lain"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("tautan pemulihan lama masih berlaku: %v", err)
	}
}

// Id sesi yang bocor dari mana pun tidak boleh bisa dipakai mengeluarkan orang
// lain dari aplikasinya.
func TestSesiMilikOrangLainTidakBisaDicabut(t *testing.T) {
	s, ctx := newStore(t)
	pemilik, orangLain := newUser(t, s, ctx, "Pemilik"), newUser(t, s, ctx, "Asing")

	buatSesi(t, s, ctx, pemilik.ID, "laptop")
	sesi, err := s.Sessions(ctx, pemilik.ID, uuid.Nil)
	if err != nil || len(sesi) != 1 {
		t.Fatalf("daftar sesi: %v (%d)", err, len(sesi))
	}

	if _, err := s.RevokeSession(ctx, sesi[0].ID, orangLain.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("mau ErrNotFound, dapat %v", err)
	}

	hash, err := s.RevokeSession(ctx, sesi[0].ID, pemilik.ID)
	if err != nil {
		t.Fatalf("cabut sesi sendiri: %v", err)
	}
	if len(hash) == 0 {
		t.Error("hash token tidak dikembalikan; hub tidak punya nama untuk menutup koneksinya")
	}
}

// ---------- nama tampilan ----------

func TestGantiNamaTampilanMenolakYangKosongDanTerlaluPanjang(t *testing.T) {
	s, ctx := newStore(t)
	me := newUser(t, s, ctx, "Bernama")

	panjang := make([]rune, 61)
	for i := range panjang {
		panjang[i] = 'x'
	}

	for _, nama := range []string{"", "   ", string(panjang)} {
		if _, err := s.UpdateDisplayName(ctx, me.ID, nama); !errors.Is(err, store.ErrInvalid) {
			t.Errorf("%q: mau ErrInvalid, dapat %v", nama, err)
		}
	}

	got, err := s.UpdateDisplayName(ctx, me.ID, "  Nama Baru  ")
	if err != nil {
		t.Fatalf("ganti nama: %v", err)
	}
	if got.DisplayName != "Nama Baru" {
		t.Errorf("nama tidak dirapikan: %q", got.DisplayName)
	}
}

// ---------- pembantu ----------

func ptr[T any](v T) *T { return &v }

// avatarPalsu menghasilkan keterangan foto yang id dan kuncinya pasti unik.
// Kunci yang ditulis tetap ("avatars/satu.jpg") membuat dua test yang berbagi
// schema saling melihat titipan yang bukan miliknya.
func avatarPalsu() store.StoredAvatar {
	id := uuid.New()
	return store.StoredAvatar{
		ID: id, Key: "avatars/" + id.String() + ".jpg", MIME: "image/jpeg", Size: 42,
	}
}

func upper(s string) string {
	out := []rune(s)
	for i, r := range out {
		if r >= 'a' && r <= 'z' {
			out[i] = r - 32
		}
	}
	return string(out)
}

// mundurkanStatus memajukan waktu dengan cara satu-satunya yang tersedia:
// memundurkan batas waktunya langsung di database.
//
// SetStatus sendiri menolak batas waktu yang sudah lewat — perilaku yang benar,
// dan diuji terpisah — jadi keadaan "status yang sudah kedaluwarsa" tidak bisa
// dibuat lewat jalur mana pun yang dipakai aplikasi. Padahal justru keadaan
// itulah yang paling perlu diuji: dia lahir dari waktu yang berjalan, bukan
// dari tindakan siapa pun.
func mundurkanStatus(t *testing.T, userID uuid.UUID, delta time.Duration) {
	t.Helper()
	_, err := testPool.Exec(t.Context(),
		`UPDATE users SET status_expires_at = now() + $1::interval WHERE id = $2`,
		delta.String(), userID)
	if err != nil {
		t.Fatalf("mundurkan batas waktu status: %v", err)
	}
}

func buatSesi(t *testing.T, s *store.Store, ctx context.Context, userID uuid.UUID, agent string) []byte {
	t.Helper()
	hash := []byte(uuid.NewString() + "-" + agent)
	if err := s.CreateSession(ctx, hash, userID, agent, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("buat sesi: %v", err)
	}
	return hash
}

func buatToken(t *testing.T, s *store.Store, ctx context.Context, userID uuid.UUID, kind, email string, ttl time.Duration) []byte {
	t.Helper()
	hash := []byte(uuid.NewString())
	if err := s.CreateEmailToken(ctx, userID, kind, email, hash, time.Now().Add(ttl)); err != nil {
		t.Fatalf("buat token %s: %v", kind, err)
	}
	return hash
}

func ambilSampah(t *testing.T, s *store.Store, ctx context.Context) []string {
	t.Helper()
	keys, err := s.TakeBlobGarbage(ctx, 10)
	if err != nil {
		t.Fatalf("ambil sampah penyimpanan: %v", err)
	}
	return keys
}
