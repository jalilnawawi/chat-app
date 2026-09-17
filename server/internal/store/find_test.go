package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/jalilnawawi/chat-app/server/internal/store"
)

// Test untuk Fase 12: mencari, meneruskan, menyematkan.
//
// Ketiganya punya gerbang kebocoran yang bentuknya sama dengan kutipan balasan
// di Fase 9 — sebuah id pesan yang menunjuk ke percakapan lain — jadi
// sebagian besar test di bawah bertanya hal yang sama dari tiga arah: apakah
// orang luar bisa melihat sesuatu yang tidak akan ditampilkan riwayatnya.

// ---------- mencari ----------

func search(t *testing.T, s *store.Store, ctx context.Context, viewer store.User, text string, conv *uuid.UUID) store.SearchResult {
	t.Helper()
	res, err := s.SearchMessages(ctx, store.SearchQuery{
		ViewerID: viewer.ID, Text: text, ConversationID: conv, Limit: 20,
	})
	if err != nil {
		t.Fatalf("cari %q: %v", text, err)
	}
	return res
}

func hitIDs(res store.SearchResult) map[uuid.UUID]bool {
	out := map[uuid.UUID]bool{}
	for _, h := range res.Hits {
		out[h.Message.ID] = true
	}
	return out
}

// Aturan utama pencarian: tidak pernah menemukan apa yang tidak akan
// ditampilkan riwayat.
func TestPencarianHanyaDiPercakapanSendiri(t *testing.T) {
	s, ctx := newStore(t)

	a := newUser(t, s, ctx, "cari_a")
	b := newUser(t, s, ctx, "cari_b")
	luar := newUser(t, s, ctx, "cari_luar")
	dm := newDirect(t, s, ctx, a, b)

	kata := "zamrud" + uuid.NewString()[:6]
	m := send(t, s, ctx, dm, a, "rapat soal "+kata+" besok")

	for _, orang := range []store.User{a, b} {
		if got := hitIDs(search(t, s, ctx, orang, kata, nil)); !got[m.ID] {
			t.Fatalf("anggota %s tidak menemukan pesannya sendiri", orang.DisplayName)
		}
	}

	if got := search(t, s, ctx, luar, kata, nil); len(got.Hits) != 0 {
		t.Fatalf("orang luar menemukan %d pesan dari percakapan orang lain", len(got.Hits))
	}
	// Menyebut id percakapan orang lain dengan tegas pun tidak membuka apa-apa.
	if got := search(t, s, ctx, luar, kata, &dm); len(got.Hits) != 0 {
		t.Fatalf("orang luar menemukan pesan lewat id percakapan yang disebut langsung")
	}
}

// Yang dikeluarkan dari grup kehilangan pencariannya juga, bukan cuma
// riwayatnya.
func TestYangDikeluarkanTidakBisaMencari(t *testing.T) {
	s, ctx := newStore(t)

	owner := newUser(t, s, ctx, "cari_owner")
	pergi := newUser(t, s, ctx, "cari_pergi")
	grup := newGroup(t, s, ctx, owner, pergi)

	kata := "safir" + uuid.NewString()[:6]
	send(t, s, ctx, grup, owner, "kode "+kata)

	if len(search(t, s, ctx, pergi, kata, nil).Hits) != 1 {
		t.Fatalf("anggota tidak menemukan pesan grupnya")
	}
	if _, err := s.RemoveMember(ctx, grup, owner.ID, pergi.ID); err != nil {
		t.Fatalf("keluarkan: %v", err)
	}
	if n := len(search(t, s, ctx, pergi, kata, nil).Hits); n != 0 {
		t.Fatalf("yang sudah dikeluarkan masih menemukan %d pesan", n)
	}
}

func TestPencarianMelewatiYangDihapusDanCatatanSistem(t *testing.T) {
	s, ctx := newStore(t)

	owner := newUser(t, s, ctx, "cari_hapus_o")
	lain := newUser(t, s, ctx, "cari_hapus_l")
	grup := newGroup(t, s, ctx, owner, lain)

	kata := "opal" + uuid.NewString()[:6]
	hapus := send(t, s, ctx, grup, owner, "rahasia "+kata)
	if _, err := s.DeleteMessage(ctx, hapus.ID, owner.ID); err != nil {
		t.Fatalf("hapus: %v", err)
	}
	// Judul grup masuk ke catatan sistem sebagai isi kejadian, bukan sebagai
	// teks pesan — dan catatan itu tidak boleh muncul di pencarian.
	if _, err := s.RenameGroup(ctx, grup, owner.ID, "Grup "+kata); err != nil {
		t.Fatalf("ganti judul: %v", err)
	}

	if n := len(search(t, s, ctx, owner, kata, nil).Hits); n != 0 {
		t.Fatalf("pencarian menemukan %d pesan yang dihapus atau catatan sistem", n)
	}
}

// Pencocokan awalan, dan semua kata harus ada.
func TestPencarianAwalanDanSemuaKata(t *testing.T) {
	s, ctx := newStore(t)

	a := newUser(t, s, ctx, "cari_awal_a")
	b := newUser(t, s, ctx, "cari_awal_b")
	dm := newDirect(t, s, ctx, a, b)

	tag := "t" + uuid.NewString()[:6]
	lengkap := send(t, s, ctx, dm, a, "Tolong KIRIMKAN laporan "+tag)
	separuh := send(t, s, ctx, dm, a, "kirimkan saja "+tag)

	// Huruf besar-kecil tidak berpengaruh, dan "kirim" menemukan "kirimkan".
	got := hitIDs(search(t, s, ctx, b, "kirim "+tag, nil))
	if !got[lengkap.ID] || !got[separuh.ID] {
		t.Fatalf("awalan tidak mencocokkan kata utuhnya: %v", got)
	}

	// Semua kata wajib ada.
	got = hitIDs(search(t, s, ctx, b, "laporan kirim "+tag, nil))
	if !got[lengkap.ID] || got[separuh.ID] {
		t.Fatalf("pencarian dua kata salah: %v", got)
	}

	// Terbaru di atas.
	res := search(t, s, ctx, b, tag, nil)
	if len(res.Hits) != 2 || res.Hits[0].Message.ID != separuh.ID {
		t.Fatalf("urutan hasil bukan terbaru dulu")
	}
	if res.Hits[0].SenderName != a.DisplayName {
		t.Fatalf("nama pengirim tidak ikut: %q", res.Hits[0].SenderName)
	}
}

// Kata pendek dicocokkan utuh, bukan sebagai awalan.
func TestKataPendekTidakJadiAwalan(t *testing.T) {
	s, ctx := newStore(t)

	a := newUser(t, s, ctx, "cari_pendek_a")
	b := newUser(t, s, ctx, "cari_pendek_b")
	dm := newDirect(t, s, ctx, a, b)

	tag := "p" + uuid.NewString()[:6]
	utuh := send(t, s, ctx, dm, a, "ke "+tag)
	panjang := send(t, s, ctx, dm, a, "kemarin "+tag)

	got := hitIDs(search(t, s, ctx, a, "ke "+tag, nil))
	if !got[utuh.ID] || got[panjang.ID] {
		t.Fatalf("kata dua huruf diperlakukan sebagai awalan: %v", got)
	}
}

// Kueri yang memuat sintaks tsquery tidak boleh jadi kesalahan server, dan
// kata yang diurai Postgres jadi satu — angka jam, alamat email — tetap
// ditemukan.
func TestKueriBerisiTandaBaca(t *testing.T) {
	s, ctx := newStore(t)

	a := newUser(t, s, ctx, "cari_tanda_a")
	b := newUser(t, s, ctx, "cari_tanda_b")
	dm := newDirect(t, s, ctx, a, b)

	tag := "q" + uuid.NewString()[:6]
	jam := send(t, s, ctx, dm, a, "mulai jam 13.00 "+tag)
	surel := send(t, s, ctx, dm, a, "kirim e-mail ke budi@contoh.id "+tag)
	kutip := send(t, s, ctx, dm, a, "kata O'Brien "+tag)

	for _, kasus := range []struct {
		q    string
		want uuid.UUID
	}{
		{"13.00 " + tag, jam.ID},
		{"e-mail " + tag, surel.ID},
		{"budi@contoh.id " + tag, surel.ID},
		{"O'Brien " + tag, kutip.ID},
	} {
		if got := hitIDs(search(t, s, ctx, a, kasus.q, nil)); !got[kasus.want] {
			t.Errorf("%q tidak menemukan pesannya: %v", kasus.q, got)
		}
	}

	for _, aneh := range []string{`a:*|!b`, `'`, `\`, `x & (y`, `rapat:`, `!!!`, `' | '`} {
		if _, err := s.SearchMessages(ctx, store.SearchQuery{
			ViewerID: a.ID, Text: aneh, Limit: 5,
		}); err != nil && !errors.Is(err, store.ErrInvalid) {
			t.Errorf("kueri %q menghasilkan kesalahan server: %v", aneh, err)
		}
	}

	if _, err := s.SearchMessages(ctx, store.SearchQuery{
		ViewerID: a.ID, Text: "  ?! ", Limit: 5,
	}); !errors.Is(err, store.ErrInvalid) {
		t.Fatalf("kueri tanpa kata seharusnya ErrInvalid, dapat %v", err)
	}
}

func TestPencarianBerhalaman(t *testing.T) {
	s, ctx := newStore(t)

	a := newUser(t, s, ctx, "cari_hal_a")
	b := newUser(t, s, ctx, "cari_hal_b")
	dm := newDirect(t, s, ctx, a, b)
	lain := newDirect(t, s, ctx, a, newUser(t, s, ctx, "cari_hal_c"))

	tag := "h" + uuid.NewString()[:6]
	want := map[uuid.UUID]bool{}
	for range 5 {
		want[send(t, s, ctx, dm, a, "isi "+tag).ID] = true
	}
	// Pesan di percakapan lain tidak ikut saat pencariannya dibatasi.
	send(t, s, ctx, lain, a, "isi "+tag)

	seen := map[uuid.UUID]bool{}
	cursor := ""
	for page := 0; ; page++ {
		res, err := s.SearchMessages(ctx, store.SearchQuery{
			ViewerID: a.ID, Text: tag, ConversationID: &dm, Cursor: cursor, Limit: 2,
		})
		if err != nil {
			t.Fatalf("halaman %d: %v", page, err)
		}
		for _, h := range res.Hits {
			if seen[h.Message.ID] {
				t.Fatalf("pesan %s muncul di dua halaman", h.Message.ID)
			}
			seen[h.Message.ID] = true
		}
		if res.NextCursor == "" {
			break
		}
		if page > 5 {
			t.Fatalf("halaman tidak pernah habis")
		}
		cursor = res.NextCursor
	}
	if len(seen) != len(want) {
		t.Fatalf("dapat %d pesan, mau %d", len(seen), len(want))
	}
	for id := range want {
		if !seen[id] {
			t.Fatalf("pesan %s tidak pernah muncul", id)
		}
	}

	if _, err := s.SearchMessages(ctx, store.SearchQuery{
		ViewerID: a.ID, Text: tag, Cursor: "bukan-cursor", Limit: 2,
	}); !errors.Is(err, store.ErrInvalid) {
		t.Fatalf("cursor rusak seharusnya ErrInvalid, dapat %v", err)
	}
}

// ---------- jendela riwayat ----------

func TestJendelaDiSekitarPesan(t *testing.T) {
	s, ctx := newStore(t)

	a := newUser(t, s, ctx, "jendela_a")
	b := newUser(t, s, ctx, "jendela_b")
	dm := newDirect(t, s, ctx, a, b)
	for i := range 30 {
		send(t, s, ctx, dm, a, "pesan "+string(rune('a'+i%26)))
	}

	w, err := s.MessagesAround(ctx, dm, a.ID, 10, 10)
	if err != nil {
		t.Fatalf("jendela: %v", err)
	}
	if len(w.Messages) != 10 || w.Messages[0].Seq != 5 || w.Messages[9].Seq != 14 {
		t.Fatalf("jendela tengah salah: %d pesan, %d..%d",
			len(w.Messages), w.Messages[0].Seq, w.Messages[len(w.Messages)-1].Seq)
	}
	if !w.HasOlder || !w.HasNewer {
		t.Fatalf("jendela tengah harus punya kedua sisi: %+v", w)
	}

	// Menabrak ujung terbaru: digeser ke belakang, tetap penuh.
	w, err = s.MessagesAround(ctx, dm, a.ID, 29, 10)
	if err != nil {
		t.Fatalf("jendela ujung: %v", err)
	}
	if len(w.Messages) != 10 || w.Messages[9].Seq != 30 || w.HasNewer || !w.HasOlder {
		t.Fatalf("jendela ujung salah: %d pesan, berakhir %d, newer=%v",
			len(w.Messages), w.Messages[len(w.Messages)-1].Seq, w.HasNewer)
	}

	// Menabrak awal.
	w, err = s.MessagesAround(ctx, dm, a.ID, 1, 10)
	if err != nil {
		t.Fatalf("jendela awal: %v", err)
	}
	if w.Messages[0].Seq != 1 || w.HasOlder || !w.HasNewer {
		t.Fatalf("jendela awal salah: mulai %d, older=%v", w.Messages[0].Seq, w.HasOlder)
	}
}

// ---------- meneruskan ----------

func forward(s *store.Store, ctx context.Context, to uuid.UUID, sender store.User, from uuid.UUID) (store.Message, bool, error) {
	return s.SendMessage(ctx, store.SendParams{
		ID: uuid.New(), ConversationID: to, SenderID: sender.ID, ForwardFromID: &from,
	})
}

func TestMeneruskanMenyalinIsiDanLampiran(t *testing.T) {
	s, ctx := newStore(t)

	a := newUser(t, s, ctx, "terus_a")
	b := newUser(t, s, ctx, "terus_b")
	c := newUser(t, s, ctx, "terus_c")
	asal := newDirect(t, s, ctx, a, b)
	tujuan := newDirect(t, s, ctx, b, c)

	att1 := newAttachment(t, s, ctx, a, "satu.png", "image/png")
	att2 := newAttachment(t, s, ctx, a, "dua.pdf", "application/pdf")
	src, _, err := s.SendMessage(ctx, store.SendParams{
		ID: uuid.New(), ConversationID: asal, SenderID: a.ID,
		Body: "dokumen rapat", AttachmentIDs: []uuid.UUID{att2, att1},
	})
	if err != nil {
		t.Fatalf("kirim sumber: %v", err)
	}

	got, created, err := forward(s, ctx, tujuan, b, src.ID)
	if err != nil || !created {
		t.Fatalf("teruskan: created=%v err=%v", created, err)
	}
	if !got.Forwarded || got.Body != "dokumen rapat" || got.SenderID != b.ID {
		t.Fatalf("pesan terusan salah: %+v", got)
	}
	if len(got.Attachments) != 2 || got.Attachments[0].Name != "dua.pdf" || got.Attachments[1].Name != "satu.png" {
		t.Fatalf("lampiran terusan salah urut atau hilang: %+v", got.Attachments)
	}
	if got.Attachments[0].ID == att2 {
		t.Fatalf("lampiran terusan memakai baris yang sama dengan sumbernya")
	}

	// c tidak pernah jadi anggota percakapan asal, tapi bisa membuka
	// lampiran yang diteruskan kepadanya — dan tetap tidak bisa membuka
	// lampiran aslinya.
	stored, err := s.AttachmentForRead(ctx, got.Attachments[0].ID, c.ID)
	if err != nil {
		t.Fatalf("penerima terusan tidak bisa membuka lampirannya: %v", err)
	}
	if stored.Key != "/uji/"+att2.String() {
		t.Fatalf("kunci penyimpanan tidak dipakai bersama: %s", stored.Key)
	}
	if _, err := s.AttachmentForRead(ctx, att2, c.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("penerima terusan bisa membuka lampiran di percakapan asal (err %v)", err)
	}

	// Riwayat membawa penandanya.
	if last := lastMessage(t, s, ctx, tujuan, c.ID); !last.Forwarded {
		t.Fatalf("penanda terusan hilang di riwayat")
	}

	// Mengedit sumbernya tidak mengubah salinan.
	if _, err := s.EditMessage(ctx, src.ID, a.ID, "sudah diubah"); err != nil {
		t.Fatalf("edit sumber: %v", err)
	}
	if last := lastMessage(t, s, ctx, tujuan, c.ID); last.Body != "dokumen rapat" {
		t.Fatalf("salinan terusan ikut berubah: %q", last.Body)
	}
}

// Gerbang kebocoran: meneruskan pesan yang tidak bisa dibaca pengirimnya.
func TestMeneruskanPesanOrangLainDitolak(t *testing.T) {
	s, ctx := newStore(t)

	a := newUser(t, s, ctx, "terus_tolak_a")
	b := newUser(t, s, ctx, "terus_tolak_b")
	luar := newUser(t, s, ctx, "terus_tolak_luar")
	rahasia := newDirect(t, s, ctx, a, b)
	milikLuar := newDirect(t, s, ctx, luar, a)

	src := send(t, s, ctx, rahasia, a, "rahasia a dan b")

	if _, _, err := forward(s, ctx, milikLuar, luar, src.ID); !errors.Is(err, store.ErrForbidden) {
		t.Fatalf("orang luar meneruskan pesan yang tidak bisa dia baca (err %v)", err)
	}
	if page, err := s.ListMessages(ctx, milikLuar, luar.ID, 0, 10); err != nil || len(page) != 0 {
		t.Fatalf("pesan tetap tercatat walau ditolak: %d pesan, err %v", len(page), err)
	}

	// Pesan yang dihapus dan catatan sistem juga tidak bisa diteruskan.
	hapus := send(t, s, ctx, rahasia, a, "akan dihapus")
	if _, err := s.DeleteMessage(ctx, hapus.ID, a.ID); err != nil {
		t.Fatalf("hapus: %v", err)
	}
	if _, _, err := forward(s, ctx, rahasia, a, hapus.ID); !errors.Is(err, store.ErrForbidden) {
		t.Fatalf("pesan terhapus bisa diteruskan (err %v)", err)
	}

	grup := newGroup(t, s, ctx, a, b)
	change, err := s.RenameGroup(ctx, grup, a.ID, "judul lain")
	if err != nil {
		t.Fatalf("ganti judul: %v", err)
	}
	if _, _, err := forward(s, ctx, rahasia, a, change.Notice.ID); !errors.Is(err, store.ErrForbidden) {
		t.Fatalf("catatan sistem bisa diteruskan (err %v)", err)
	}
}

// Pengiriman ulang terusan yang sudah tersimpan tidak menggandakan — dan
// tetap berhasil walau sumbernya dihapus di antaranya.
func TestTerusanIdempotenWalauSumbernyaDihapus(t *testing.T) {
	s, ctx := newStore(t)

	a := newUser(t, s, ctx, "terus_ulang_a")
	b := newUser(t, s, ctx, "terus_ulang_b")
	dm := newDirect(t, s, ctx, a, b)
	src := send(t, s, ctx, dm, a, "sekali saja")

	p := store.SendParams{ID: uuid.New(), ConversationID: dm, SenderID: b.ID, ForwardFromID: &src.ID}
	first, created, err := s.SendMessage(ctx, p)
	if err != nil || !created {
		t.Fatalf("teruskan: %v", err)
	}
	if _, err := s.DeleteMessage(ctx, src.ID, a.ID); err != nil {
		t.Fatalf("hapus sumber: %v", err)
	}
	again, created, err := s.SendMessage(ctx, p)
	if err != nil {
		t.Fatalf("kirim ulang terusan ditolak setelah sumbernya dihapus: %v", err)
	}
	if created || again.Seq != first.Seq {
		t.Fatalf("kirim ulang terusan menggandakan pesan")
	}
}

// Menghapus pesan asli sebuah foto tidak boleh merusak salinan terusannya
// begitu penyapu lewat — dan byte-nya tetap dibuang setelah salinan terakhir
// ikut pergi.
func TestPenyapuMenghormatiKunciBersama(t *testing.T) {
	s, ctx := newStore(t)

	a := newUser(t, s, ctx, "sapu_a")
	b := newUser(t, s, ctx, "sapu_b")
	dm := newDirect(t, s, ctx, a, b)

	att := newAttachment(t, s, ctx, a, "foto.png", "image/png")
	key := "/uji/" + att.String()
	src, _, err := s.SendMessage(ctx, store.SendParams{
		ID: uuid.New(), ConversationID: dm, SenderID: a.ID, AttachmentIDs: []uuid.UUID{att},
	})
	if err != nil {
		t.Fatalf("kirim: %v", err)
	}
	copy1, _, err := forward(s, ctx, dm, b, src.ID)
	if err != nil {
		t.Fatalf("teruskan: %v", err)
	}

	sweep := func() map[uuid.UUID]store.Orphan {
		t.Helper()
		got, err := s.TakeOrphanAttachments(ctx, 0, 1000)
		if err != nil {
			t.Fatalf("sapu: %v", err)
		}
		out := map[uuid.UUID]store.Orphan{}
		for _, o := range got {
			out[o.ID] = o
		}
		return out
	}

	// Sumber dihapus: barisnya yatim dan tersapu, byte-nya TIDAK.
	if _, err := s.DeleteMessage(ctx, src.ID, a.ID); err != nil {
		t.Fatalf("hapus sumber: %v", err)
	}
	swept := sweep()
	o, ok := swept[att]
	if !ok {
		t.Fatalf("baris lampiran sumber tidak tersapu")
	}
	if o.StorageKey != "" {
		t.Fatalf("penyapu mau membuang byte yang masih dipakai salinan terusan")
	}
	if _, err := s.AttachmentForRead(ctx, copy1.Attachments[0].ID, a.ID); err != nil {
		t.Fatalf("salinan terusan rusak setelah sumbernya disapu: %v", err)
	}

	// Salinan terakhir dihapus: sekarang byte-nya ikut dibuang.
	if _, err := s.DeleteMessage(ctx, copy1.ID, b.ID); err != nil {
		t.Fatalf("hapus salinan: %v", err)
	}
	swept = sweep()
	o, ok = swept[copy1.Attachments[0].ID]
	if !ok || o.StorageKey != key {
		t.Fatalf("byte tidak dibuang setelah salinan terakhir pergi: %+v", o)
	}
}

// ---------- menyematkan ----------

func pins(t *testing.T, s *store.Store, ctx context.Context, conv, viewer uuid.UUID) []store.Pin {
	t.Helper()
	got, err := s.Pins(ctx, conv, viewer)
	if err != nil {
		t.Fatalf("daftar sematan: %v", err)
	}
	return got
}

func TestSematanDiDMDanCatatannya(t *testing.T) {
	s, ctx := newStore(t)

	a := newUser(t, s, ctx, "pin_a")
	b := newUser(t, s, ctx, "pin_b")
	dm := newDirect(t, s, ctx, a, b)
	m := send(t, s, ctx, dm, a, "alamat kantor baru")

	// Di DM keduanya boleh.
	change, err := s.PinMessage(ctx, m.ID, b.ID)
	if err != nil || !change.Changed {
		t.Fatalf("sematkan: changed=%v err=%v", change.Changed, err)
	}
	ev := change.Notice.SystemEvent
	if change.Notice.Kind != "system" || ev == nil || ev.Type != store.SystemMessagePinned ||
		ev.MessageID == nil || *ev.MessageID != m.ID || ev.MessageSeq != m.Seq {
		t.Fatalf("catatan sematan salah: %+v", change.Notice)
	}
	if last := lastMessage(t, s, ctx, dm, a.ID); last.ID != change.Notice.ID {
		t.Fatalf("catatan sematan tidak masuk riwayat")
	}

	got := pins(t, s, ctx, dm, a.ID)
	if len(got) != 1 || got[0].Message.ID != m.ID || got[0].PinnedByName != b.DisplayName {
		t.Fatalf("daftar sematan salah: %+v", got)
	}

	// Menyematkan ulang tidak mencatat apa pun.
	again, err := s.PinMessage(ctx, m.ID, a.ID)
	if err != nil || again.Changed {
		t.Fatalf("sematan kembar: changed=%v err=%v", again.Changed, err)
	}

	// Isi sematan mengikuti edit.
	if _, err := s.EditMessage(ctx, m.ID, a.ID, "alamat kantor pindah lagi"); err != nil {
		t.Fatalf("edit: %v", err)
	}
	if got := pins(t, s, ctx, dm, b.ID); got[0].Message.Body != "alamat kantor pindah lagi" {
		t.Fatalf("sematan tidak mengikuti edit: %q", got[0].Message.Body)
	}

	un, err := s.UnpinMessage(ctx, m.ID, a.ID)
	if err != nil || !un.Changed || un.Notice.SystemEvent.Type != store.SystemMessageUnpinned {
		t.Fatalf("lepas sematan: %+v err=%v", un, err)
	}
	if got := pins(t, s, ctx, dm, a.ID); len(got) != 0 {
		t.Fatalf("sematan masih ada setelah dilepas")
	}
	if un2, err := s.UnpinMessage(ctx, m.ID, a.ID); err != nil || un2.Changed {
		t.Fatalf("lepas kembar: changed=%v err=%v", un2.Changed, err)
	}
}

func TestSematanGrupHakPemilik(t *testing.T) {
	s, ctx := newStore(t)

	owner := newUser(t, s, ctx, "pin_owner")
	anggota := newUser(t, s, ctx, "pin_anggota")
	luar := newUser(t, s, ctx, "pin_luar")
	grup := newGroup(t, s, ctx, owner, anggota)
	m := send(t, s, ctx, grup, anggota, "jadwal piket")

	if _, err := s.PinMessage(ctx, m.ID, anggota.ID); !errors.Is(err, store.ErrForbidden) {
		t.Fatalf("anggota biasa menyematkan di grup (err %v)", err)
	}
	// Orang luar tidak diberi tahu bahwa pesannya ada.
	if _, err := s.PinMessage(ctx, m.ID, luar.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("orang luar seharusnya ErrNotFound, dapat %v", err)
	}
	if _, err := s.PinMessage(ctx, m.ID, owner.ID); err != nil {
		t.Fatalf("pemilik menyematkan: %v", err)
	}
	if _, err := s.UnpinMessage(ctx, m.ID, anggota.ID); !errors.Is(err, store.ErrForbidden) {
		t.Fatalf("anggota biasa melepas sematan (err %v)", err)
	}
}

func TestSematanPesanTerhapusDanSistem(t *testing.T) {
	s, ctx := newStore(t)

	owner := newUser(t, s, ctx, "pin_hapus_o")
	lain := newUser(t, s, ctx, "pin_hapus_l")
	grup := newGroup(t, s, ctx, owner, lain)

	m := send(t, s, ctx, grup, owner, "segera dihapus")
	if _, err := s.PinMessage(ctx, m.ID, owner.ID); err != nil {
		t.Fatalf("sematkan: %v", err)
	}
	if _, err := s.DeleteMessage(ctx, m.ID, owner.ID); err != nil {
		t.Fatalf("hapus: %v", err)
	}
	if got := pins(t, s, ctx, grup, owner.ID); len(got) != 0 {
		t.Fatalf("pesan terhapus tetap tersemat")
	}
	if _, err := s.PinMessage(ctx, m.ID, owner.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("pesan terhapus bisa disematkan (err %v)", err)
	}

	change, err := s.RenameGroup(ctx, grup, owner.ID, "judul pin")
	if err != nil {
		t.Fatalf("ganti judul: %v", err)
	}
	if _, err := s.PinMessage(ctx, change.Notice.ID, owner.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("catatan sistem bisa disematkan (err %v)", err)
	}
}

func TestBatasSematan(t *testing.T) {
	s, ctx := newStore(t)

	a := newUser(t, s, ctx, "pin_batas_a")
	b := newUser(t, s, ctx, "pin_batas_b")
	dm := newDirect(t, s, ctx, a, b)

	var first store.Message
	for i := range 20 {
		m := send(t, s, ctx, dm, a, "penting")
		if i == 0 {
			first = m
		}
		if _, err := s.PinMessage(ctx, m.ID, a.ID); err != nil {
			t.Fatalf("sematan ke-%d: %v", i+1, err)
		}
	}

	lebih := send(t, s, ctx, dm, a, "satu lagi")
	if _, err := s.PinMessage(ctx, lebih.ID, a.ID); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("sematan ke-21 seharusnya ErrConflict, dapat %v", err)
	}
	// Menyematkan ulang yang sudah tersemat tetap "tidak berubah", bukan "penuh".
	if again, err := s.PinMessage(ctx, first.ID, a.ID); err != nil || again.Changed {
		t.Fatalf("sematan kembar di percakapan penuh: changed=%v err=%v", again.Changed, err)
	}
	if got := pins(t, s, ctx, dm, a.ID); len(got) != 20 {
		t.Fatalf("jumlah sematan %d, mau 20", len(got))
	}
}
