package store_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/jalilnawawi/chat-app/server/internal/store"
)

// Pemeriksaan terpenting dari seluruh berkas ini.
//
// Menambahkan orang ketiga ke sebuah DM akan mempertahankan `direct_key` milik
// dua orang pertama — jadi percakapan itu tetap dianggap DM antara mereka
// berdua, sekaligus berisi orang yang tidak pernah diajak siapa pun. Percakapan
// pribadi yang diam-diam bertambah pendengarnya adalah bentuk kegagalan
// terburuk yang bisa dimiliki aplikasi chat.
func TestDMTidakBisaDikelola(t *testing.T) {
	s, ctx := newStore(t)

	a := newUser(t, s, ctx, "grup_dm_a")
	b := newUser(t, s, ctx, "grup_dm_b")
	luar := newUser(t, s, ctx, "grup_dm_luar")
	dm := newDirect(t, s, ctx, a, b)

	if _, err := s.AddMembers(ctx, dm, a.ID, []uuid.UUID{luar.ID}); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("menambah orang ke DM seharusnya ErrNotFound, dapat %v", err)
	}
	if _, err := s.RenameGroup(ctx, dm, a.ID, "DM jadi grup"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("mengganti judul DM seharusnya ErrNotFound, dapat %v", err)
	}
	if _, err := s.RemoveMember(ctx, dm, a.ID, b.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("mengeluarkan dari DM seharusnya ErrNotFound, dapat %v", err)
	}
	if _, err := s.TransferOwnership(ctx, dm, a.ID, b.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("memindah kepemilikan DM seharusnya ErrNotFound, dapat %v", err)
	}

	// Dan DM-nya benar-benar tidak berubah.
	members, err := s.Members(ctx, dm)
	if err != nil {
		t.Fatalf("baca anggota: %v", err)
	}
	if len(members) != 2 {
		t.Fatalf("DM berisi %d orang, seharusnya tetap 2", len(members))
	}
}

func TestHanyaPemilikYangMengelola(t *testing.T) {
	s, ctx := newStore(t)

	owner := newUser(t, s, ctx, "grup_owner")
	anggota := newUser(t, s, ctx, "grup_anggota")
	calon := newUser(t, s, ctx, "grup_calon")
	grup := newGroup(t, s, ctx, owner, anggota)

	if _, err := s.RenameGroup(ctx, grup, anggota.ID, "judul baru"); !errors.Is(err, store.ErrForbidden) {
		t.Fatalf("anggota mengganti judul seharusnya ErrForbidden, dapat %v", err)
	}
	if _, err := s.AddMembers(ctx, grup, anggota.ID, []uuid.UUID{calon.ID}); !errors.Is(err, store.ErrForbidden) {
		t.Fatalf("anggota menambah orang seharusnya ErrForbidden, dapat %v", err)
	}
	if _, err := s.RemoveMember(ctx, grup, anggota.ID, owner.ID); !errors.Is(err, store.ErrForbidden) {
		t.Fatalf("anggota mengeluarkan orang seharusnya ErrForbidden, dapat %v", err)
	}

	// Orang di luar grup dijawab sama dengan anggota biasa: tidak boleh.
	if _, err := s.RenameGroup(ctx, grup, calon.ID, "judul baru"); !errors.Is(err, store.ErrForbidden) {
		t.Fatalf("orang luar seharusnya ErrForbidden, dapat %v", err)
	}
}

// Pemilik tidak bisa dikeluarkan, dan tidak bisa mengeluarkan dirinya sendiri
// lewat jalur ini — keduanya akan meninggalkan grup tanpa pemilik.
func TestPemilikTidakBisaDikeluarkan(t *testing.T) {
	s, ctx := newStore(t)

	owner := newUser(t, s, ctx, "kick_owner")
	lain := newUser(t, s, ctx, "kick_lain")
	grup := newGroup(t, s, ctx, owner, lain)

	if _, err := s.RemoveMember(ctx, grup, owner.ID, owner.ID); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("mengeluarkan diri sendiri seharusnya ErrConflict, dapat %v", err)
	}

	// Setelah kepemilikan pindah, yang lama jadi anggota biasa dan bisa
	// dikeluarkan oleh pemilik baru.
	if _, err := s.TransferOwnership(ctx, grup, owner.ID, lain.ID); err != nil {
		t.Fatalf("pindah kepemilikan: %v", err)
	}
	if _, err := s.RemoveMember(ctx, grup, lain.ID, owner.ID); err != nil {
		t.Fatalf("pemilik baru mengeluarkan pemilik lama: %v", err)
	}
}

// Setiap perubahan meninggalkan catatan, dan catatan itu adalah pesan biasa
// dengan `seq` — jadi dia ikut terbaca oleh riwayat tanpa jalur baru.
func TestPerubahanMeninggalkanCatatanDiRiwayat(t *testing.T) {
	s, ctx := newStore(t)

	owner := newUser(t, s, ctx, "catat_owner")
	anggota := newUser(t, s, ctx, "catat_anggota")
	baru := newUser(t, s, ctx, "catat_baru")
	grup := newGroup(t, s, ctx, owner, anggota)

	send(t, s, ctx, grup, owner, "halo semuanya")

	change, err := s.AddMembers(ctx, grup, owner.ID, []uuid.UUID{baru.ID})
	if err != nil {
		t.Fatalf("tambah anggota: %v", err)
	}
	if change.Notice.Kind != "system" {
		t.Fatalf("catatan bukan pesan sistem: %q", change.Notice.Kind)
	}
	if change.Notice.SystemEvent == nil || change.Notice.SystemEvent.Type != store.SystemMemberAdded {
		t.Fatalf("jenis kejadian salah: %+v", change.Notice.SystemEvent)
	}
	if len(change.Members) != 3 {
		t.Fatalf("anggota setelah ditambah = %d, seharusnya 3", len(change.Members))
	}

	// Nama IKUT tersalin ke dalam catatan. Tanpa itu, catatan tentang orang
	// yang sudah keluar berbunyi "mengeluarkan (tidak dikenal)".
	if got := change.Notice.SystemEvent.Targets; len(got) != 1 || got[0].Name != baru.DisplayName {
		t.Fatalf("nama tidak tersalin ke catatan: %+v", got)
	}

	page, err := s.ListMessages(ctx, grup, anggota.ID, 0, 50)
	if err != nil {
		t.Fatalf("baca riwayat: %v", err)
	}
	last := page[len(page)-1]
	if last.ID != change.Notice.ID {
		t.Fatal("catatan sistem tidak muncul di riwayat")
	}
	if last.Seq <= page[len(page)-2].Seq {
		t.Fatalf("catatan sistem tidak mendapat seq berikutnya: %d setelah %d",
			last.Seq, page[len(page)-2].Seq)
	}
}

// Catatan sistem bukan ucapan siapa pun: tidak bisa disunting, dihapus,
// dibalas, atau direaksi. Empat jalur, satu aturan.
func TestCatatanSistemTidakBisaDisentuh(t *testing.T) {
	s, ctx := newStore(t)

	owner := newUser(t, s, ctx, "sentuh_owner")
	lain := newUser(t, s, ctx, "sentuh_lain")
	baru := newUser(t, s, ctx, "sentuh_baru")
	grup := newGroup(t, s, ctx, owner, lain)

	change, err := s.AddMembers(ctx, grup, owner.ID, []uuid.UUID{baru.ID})
	if err != nil {
		t.Fatalf("tambah anggota: %v", err)
	}
	notice := change.Notice

	// Pelakunya adalah `sender_id` catatan itu, jadi dialah satu-satunya yang
	// bisa lolos pemeriksaan kepemilikan pada jalur edit dan hapus. Kalau
	// `kind` tidak ikut diperiksa, dia bisa menyunting catatan resmi jadi
	// kalimat apa pun.
	if _, err := s.EditMessage(ctx, notice.ID, owner.ID, "aku tidak menambahkan siapa-siapa"); !errors.Is(err, store.ErrForbidden) {
		t.Fatalf("menyunting catatan sistem seharusnya ErrForbidden, dapat %v", err)
	}
	if _, err := s.DeleteMessage(ctx, notice.ID, owner.ID); !errors.Is(err, store.ErrForbidden) {
		t.Fatalf("menghapus catatan sistem seharusnya ErrForbidden, dapat %v", err)
	}
	if _, err := s.AddReaction(ctx, notice.ID, lain.ID, "👍"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("bereaksi ke catatan sistem seharusnya ErrNotFound, dapat %v", err)
	}
	if _, _, err := s.SendMessage(ctx, store.SendParams{
		ID: uuid.New(), ConversationID: grup, SenderID: lain.ID,
		Body: "membalas catatan", ReplyToID: &notice.ID,
	}); !errors.Is(err, store.ErrForbidden) {
		t.Fatalf("membalas catatan sistem seharusnya ErrForbidden, dapat %v", err)
	}
}

// Pemilik yang keluar tidak ditahan; kepemilikan berpindah ke anggota terlama.
// Grup yang pemiliknya berhenti memakai aplikasi ini tidak boleh jadi grup yang
// tidak bisa dikelola siapa pun, selamanya.
func TestPemilikKeluarMemindahkanKepemilikan(t *testing.T) {
	s, ctx := newStore(t)

	owner := newUser(t, s, ctx, "pergi_owner")
	lama := newUser(t, s, ctx, "pergi_lama")
	muda := newUser(t, s, ctx, "pergi_muda")

	grup := newGroup(t, s, ctx, owner, lama)
	// `muda` bergabung belakangan, jadi `lama` yang seharusnya mewarisi.
	if _, err := s.AddMembers(ctx, grup, owner.ID, []uuid.UUID{muda.ID}); err != nil {
		t.Fatalf("tambah anggota: %v", err)
	}

	change, err := s.LeaveGroup(ctx, grup, owner.ID)
	if err != nil {
		t.Fatalf("keluar grup: %v", err)
	}
	if change.Departed == nil || *change.Departed != owner.ID {
		t.Fatalf("yang keluar tidak dilaporkan: %+v", change.Departed)
	}

	var pemilikBaru string
	for _, m := range change.Members {
		if m.UserID == owner.ID {
			t.Fatal("pemilik lama masih terdaftar sebagai anggota")
		}
		if m.Role == "owner" {
			pemilikBaru = m.DisplayName
		}
	}
	if pemilikBaru != lama.DisplayName {
		t.Fatalf("kepemilikan jatuh ke %q, seharusnya ke anggota terlama %q",
			pemilikBaru, lama.DisplayName)
	}
}

// Menambah orang yang sudah jadi anggota dilewati diam-diam — dua orang yang
// menambahkan orang yang sama bersamaan adalah kejadian biasa. Id yang TIDAK
// menunjuk pengguna mana pun ditolak: itu bukan balapan, itu id yang dikarang.
func TestTambahAnggotaMembedakanKembarDariKarangan(t *testing.T) {
	s, ctx := newStore(t)

	owner := newUser(t, s, ctx, "tambah_owner")
	sudah := newUser(t, s, ctx, "tambah_sudah")
	grup := newGroup(t, s, ctx, owner, sudah)

	change, err := s.AddMembers(ctx, grup, owner.ID, []uuid.UUID{sudah.ID})
	if err != nil {
		t.Fatalf("menambah anggota yang sudah ada: %v", err)
	}
	if change.Notice.ID != uuid.Nil {
		t.Fatal("tidak ada yang berubah, tapi tetap membuat catatan")
	}
	if len(change.Members) != 2 {
		t.Fatalf("jumlah anggota berubah jadi %d", len(change.Members))
	}

	if _, err := s.AddMembers(ctx, grup, owner.ID, []uuid.UUID{uuid.New()}); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("id yang bukan pengguna seharusnya ErrNotFound, dapat %v", err)
	}
}

// Judul yang diganti jadi judul yang sama tidak menaruh baris "judul diganti"
// di tengah percakapan.
func TestGantiJudulSamaTidakMencatatApaPun(t *testing.T) {
	s, ctx := newStore(t)

	owner := newUser(t, s, ctx, "judul_owner")
	lain := newUser(t, s, ctx, "judul_lain")
	grup := newGroup(t, s, ctx, owner, lain)

	change, err := s.RenameGroup(ctx, grup, owner.ID, "Grup Uji")
	if err != nil {
		t.Fatalf("ganti judul: %v", err)
	}
	if change.Notice.ID != uuid.Nil {
		t.Fatal("judul tidak berubah, tapi tetap membuat catatan")
	}

	change, err = s.RenameGroup(ctx, grup, owner.ID, "Nama Lain")
	if err != nil {
		t.Fatalf("ganti judul beneran: %v", err)
	}
	if change.Title != "Nama Lain" {
		t.Fatalf("judul setelah ganti = %q", change.Title)
	}
	if change.Notice.SystemEvent == nil || change.Notice.SystemEvent.Title != "Nama Lain" {
		t.Fatalf("catatan tidak membawa judul barunya: %+v", change.Notice.SystemEvent)
	}
}

// Orang yang sudah dikeluarkan kehilangan seluruh aksesnya — termasuk membaca
// riwayat yang dulu bisa dia lihat.
func TestYangDikeluarkanKehilanganAkses(t *testing.T) {
	s, ctx := newStore(t)

	owner := newUser(t, s, ctx, "akses_owner")
	korban := newUser(t, s, ctx, "akses_korban")
	grup := newGroup(t, s, ctx, owner, korban)

	m := send(t, s, ctx, grup, owner, "rahasia internal")

	if _, err := s.RemoveMember(ctx, grup, owner.ID, korban.ID); err != nil {
		t.Fatalf("keluarkan: %v", err)
	}

	ok, err := s.IsMember(ctx, grup, korban.ID)
	if err != nil {
		t.Fatalf("cek keanggotaan: %v", err)
	}
	if ok {
		t.Fatal("yang dikeluarkan masih tercatat sebagai anggota")
	}
	if _, err := s.AddReaction(ctx, m.ID, korban.ID, "👍"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("yang dikeluarkan masih bisa bereaksi (err %v)", err)
	}
	if _, _, err := s.SendMessage(ctx, store.SendParams{
		ID: uuid.New(), ConversationID: grup, SenderID: korban.ID,
		Body: "masih di sini", Mentions: []uuid.UUID{owner.ID},
	}); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("yang dikeluarkan masih bisa mengirim pesan (err %v)", err)
	}

	// Percakapannya pun sudah tidak ada di daftarnya.
	list, err := s.ListConversations(ctx, korban.ID)
	if err != nil {
		t.Fatalf("daftar percakapan: %v", err)
	}
	for _, c := range list {
		if c.ID == grup {
			t.Fatal("grup masih muncul di daftar orang yang sudah dikeluarkan")
		}
	}
}
