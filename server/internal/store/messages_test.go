package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/jalilnawawi/chat-app/server/internal/store"
)

// ---------- balas / kutip ----------

// Pemeriksaan yang paling penting dari seluruh fase ini.
//
// Tanpa "pesan yang dibalas WAJIB di percakapan yang sama", siapa pun bisa
// membuat DM dengan dirinya sendiri, mengirim pesan yang mengutip id pesan dari
// percakapan yang tidak dia ikuti, dan membaca isinya dari dalam gelembung
// kutipan. Bentuknya persis seperti fitur; akibatnya membaca percakapan orang
// lain satu pesan pada satu waktu.
func TestBalasLintasPercakapanDitolak(t *testing.T) {
	s, ctx := newStore(t)

	penyusup := newUser(t, s, ctx, "penyusup")
	korbanA := newUser(t, s, ctx, "korban_a")
	korbanB := newUser(t, s, ctx, "korban_b")

	rahasia := newDirect(t, s, ctx, korbanA, korbanB)
	target := send(t, s, ctx, rahasia, korbanA, "nomor rekeningnya 123")

	milikPenyusup := newDirect(t, s, ctx, penyusup, korbanA)

	_, _, err := s.SendMessage(ctx, store.SendParams{
		ID: uuid.New(), ConversationID: milikPenyusup, SenderID: penyusup.ID,
		Body: "lihat ini", ReplyToID: &target.ID,
	})
	if !errors.Is(err, store.ErrForbidden) {
		t.Fatalf("mengutip pesan dari percakapan lain seharusnya ErrForbidden, dapat %v", err)
	}

	// Dan pesannya benar-benar batal, bukan tersimpan tanpa kutipan.
	page, err := s.ListMessages(ctx, milikPenyusup, penyusup.ID, 0, 50)
	if err != nil {
		t.Fatalf("baca riwayat: %v", err)
	}
	if len(page) != 0 {
		t.Fatalf("pesan yang kutipannya ditolak tetap tersimpan: %d pesan", len(page))
	}
}

// Isi kutipan TIDAK disalin, jadi pesan yang diedit harus tampil versi
// terbarunya dan yang dihapus harus tampil sebagai "pesan dihapus" — keduanya
// tanpa satu pun jalur sinkronisasi tambahan.
func TestKutipanMengikutiEditDanHapus(t *testing.T) {
	s, ctx := newStore(t)

	a := newUser(t, s, ctx, "anton")
	b := newUser(t, s, ctx, "bima")
	conv := newDirect(t, s, ctx, a, b)

	asli := send(t, s, ctx, conv, a, "versi pertama")
	_, _, err := s.SendMessage(ctx, store.SendParams{
		ID: uuid.New(), ConversationID: conv, SenderID: b.ID,
		Body: "setuju", ReplyToID: &asli.ID,
	})
	if err != nil {
		t.Fatalf("balas: %v", err)
	}

	if _, err := s.EditMessage(ctx, asli.ID, a.ID, "versi kedua"); err != nil {
		t.Fatalf("edit: %v", err)
	}

	balasan := lastMessage(t, s, ctx, conv, b.ID)
	if balasan.ReplyTo == nil {
		t.Fatal("kutipan hilang setelah pesan aslinya diedit")
	}
	if balasan.ReplyTo.Body != "versi kedua" {
		t.Fatalf("kutipan masih menampilkan isi lama: %q", balasan.ReplyTo.Body)
	}

	if _, err := s.DeleteMessage(ctx, asli.ID, a.ID); err != nil {
		t.Fatalf("hapus: %v", err)
	}

	balasan = lastMessage(t, s, ctx, conv, b.ID)
	if balasan.ReplyTo == nil {
		t.Fatal("kutipan hilang setelah pesan aslinya dihapus — seharusnya tetap ada")
	}
	if !balasan.ReplyTo.Deleted {
		t.Fatal("kutipan tidak ditandai sebagai pesan yang dihapus")
	}
}

// Mengedit pesan yang MEMBALAS tidak boleh menghilangkan kutipannya. Jalur ini
// memakai CTE karena RETURNING tidak bisa menjangkau self-join — dan tanpa CTE
// itu, kutipan lenyap begitu pengirimnya memperbaiki satu salah ketik.
func TestEditPesanTidakMenghilangkanKutipan(t *testing.T) {
	s, ctx := newStore(t)

	a := newUser(t, s, ctx, "cindy")
	b := newUser(t, s, ctx, "dodi")
	conv := newDirect(t, s, ctx, a, b)

	asli := send(t, s, ctx, conv, a, "pertanyaannya apa")
	balasan, _, err := s.SendMessage(ctx, store.SendParams{
		ID: uuid.New(), ConversationID: conv, SenderID: b.ID,
		Body: "jawabnaya ini", ReplyToID: &asli.ID,
	})
	if err != nil {
		t.Fatalf("balas: %v", err)
	}
	if balasan.ReplyTo == nil {
		t.Fatal("kutipan tidak ikut pada jawaban SendMessage")
	}

	diperbaiki, err := s.EditMessage(ctx, balasan.ID, b.ID, "jawabannya ini")
	if err != nil {
		t.Fatalf("edit: %v", err)
	}
	if diperbaiki.ReplyTo == nil || diperbaiki.ReplyTo.ID != asli.ID {
		t.Fatal("kutipan hilang dari jawaban EditMessage")
	}
}

// ---------- sebutan ----------

func TestSebutanHanyaUntukAnggota(t *testing.T) {
	s, ctx := newStore(t)

	a := newUser(t, s, ctx, "erna")
	b := newUser(t, s, ctx, "farid")
	orangLuar := newUser(t, s, ctx, "gita")
	conv := newDirect(t, s, ctx, a, b)

	_, _, err := s.SendMessage(ctx, store.SendParams{
		ID: uuid.New(), ConversationID: conv, SenderID: a.ID,
		Body: "halo", Mentions: []uuid.UUID{orangLuar.ID},
	})
	if !errors.Is(err, store.ErrForbidden) {
		t.Fatalf("menyebut orang luar seharusnya ErrForbidden, dapat %v", err)
	}

	page, err := s.ListMessages(ctx, conv, a.ID, 0, 50)
	if err != nil {
		t.Fatalf("baca riwayat: %v", err)
	}
	if len(page) != 0 {
		t.Fatalf("pesan dengan sebutan tidak sah tetap tersimpan: %d pesan", len(page))
	}
}

// Penanda sebutan harus BERTAHAN walau percakapannya sudah dibaca. Itu seluruh
// alasan mention_ack_seq terpisah dari last_read_seq.
func TestPenandaSebutanBertahanSetelahDibaca(t *testing.T) {
	s, ctx := newStore(t)

	a := newUser(t, s, ctx, "hadi")
	b := newUser(t, s, ctx, "indri")
	conv := newDirect(t, s, ctx, a, b)

	m, _, err := s.SendMessage(ctx, store.SendParams{
		ID: uuid.New(), ConversationID: conv, SenderID: a.ID,
		Body: "tolong dicek ya", Mentions: []uuid.UUID{b.ID},
	})
	if err != nil {
		t.Fatalf("kirim dengan sebutan: %v", err)
	}

	if _, err := s.MarkRead(ctx, conv, b.ID, m.Seq); err != nil {
		t.Fatalf("tandai terbaca: %v", err)
	}

	c := conversationOf(t, s, ctx, b.ID, conv)
	if c.Unread != 0 {
		t.Fatalf("unread seharusnya nol setelah dibaca, dapat %d", c.Unread)
	}
	if c.MentionSeq <= c.MentionAckSeq {
		t.Fatalf("penanda sebutan ikut hilang saat percakapan dibaca (mention %d, ack %d)",
			c.MentionSeq, c.MentionAckSeq)
	}

	if _, err := s.AckMentions(ctx, conv, b.ID, m.Seq); err != nil {
		t.Fatalf("ack sebutan: %v", err)
	}
	c = conversationOf(t, s, ctx, b.ID, conv)
	if c.MentionSeq > c.MentionAckSeq {
		t.Fatalf("penanda sebutan tidak padam setelah di-ack (mention %d, ack %d)",
			c.MentionSeq, c.MentionAckSeq)
	}
}

// @semua tidak menyentuh daftar sebutan per orang, dan hanya berlaku di grup.
// Di DM dia cuma cara lain menembus peredam dering tanpa menyebut siapa pun.
func TestSebutSemuaHanyaDiGrup(t *testing.T) {
	s, ctx := newStore(t)

	a := newUser(t, s, ctx, "joko")
	b := newUser(t, s, ctx, "kiki")
	c := newUser(t, s, ctx, "lina")

	grup := newGroup(t, s, ctx, a, b, c)
	di, _, err := s.SendMessage(ctx, store.SendParams{
		ID: uuid.New(), ConversationID: grup, SenderID: a.ID,
		Body: "rapat jam 3", MentionsAll: true,
	})
	if err != nil {
		t.Fatalf("kirim @semua di grup: %v", err)
	}
	if !di.MentionsAll {
		t.Fatal("@semua tidak tercatat di grup")
	}
	for _, orang := range []store.User{b, c} {
		got := conversationOf(t, s, ctx, orang.ID, grup)
		if got.MentionSeq != di.Seq {
			t.Fatalf("@semua tidak menandai %s (mention_seq %d, seq pesan %d)",
				orang.DisplayName, got.MentionSeq, di.Seq)
		}
	}
	// Pengirimnya sendiri tidak ikut dibangunkan.
	if got := conversationOf(t, s, ctx, a.ID, grup); got.MentionSeq != 0 {
		t.Fatalf("@semua ikut menandai pengirimnya sendiri (mention_seq %d)", got.MentionSeq)
	}

	dm := newDirect(t, s, ctx, a, b)
	diDM, _, err := s.SendMessage(ctx, store.SendParams{
		ID: uuid.New(), ConversationID: dm, SenderID: a.ID,
		Body: "halo", MentionsAll: true,
	})
	if err != nil {
		t.Fatalf("kirim @semua di DM: %v", err)
	}
	if diDM.MentionsAll {
		t.Fatal("@semua seharusnya diabaikan di DM")
	}
}

// ---------- yang paling mudah rusak diam-diam dari fase sebelumnya ----------

// Idempotensi adalah dasar seluruh pengiriman optimistik di client: `id` dibuat
// di sana, dan retry setelah timeout jaringan harus mengembalikan pesan yang
// sama — bukan pesan kedua.
func TestKirimUlangIdSamaTidakMenggandakan(t *testing.T) {
	s, ctx := newStore(t)

	a := newUser(t, s, ctx, "mira")
	b := newUser(t, s, ctx, "nanda")
	conv := newDirect(t, s, ctx, a, b)

	id := uuid.New()
	p := store.SendParams{ID: id, ConversationID: conv, SenderID: a.ID, Body: "sekali saja"}

	pertama, created, err := s.SendMessage(ctx, p)
	if err != nil || !created {
		t.Fatalf("kiriman pertama: created=%v err=%v", created, err)
	}

	kedua, created, err := s.SendMessage(ctx, p)
	if err != nil {
		t.Fatalf("kiriman ulang: %v", err)
	}
	if created {
		t.Fatal("kiriman ulang dilaporkan sebagai pesan baru")
	}
	if kedua.Seq != pertama.Seq {
		t.Fatalf("kiriman ulang mengalokasikan seq baru: %d lalu %d", pertama.Seq, kedua.Seq)
	}

	page, err := s.ListMessages(ctx, conv, a.ID, 0, 50)
	if err != nil {
		t.Fatalf("baca riwayat: %v", err)
	}
	if len(page) != 1 {
		t.Fatalf("riwayat berisi %d pesan, seharusnya 1", len(page))
	}
}

// Id pesan yang sama dipakai untuk percakapan lain adalah tabrakan, bukan
// retry — dan diperlakukan sebagai bentrok, bukan diam-diam diterima.
func TestKirimUlangDiPercakapanLainBentrok(t *testing.T) {
	s, ctx := newStore(t)

	a := newUser(t, s, ctx, "omar")
	b := newUser(t, s, ctx, "putri")
	c := newUser(t, s, ctx, "rudi")
	satu := newDirect(t, s, ctx, a, b)
	dua := newDirect(t, s, ctx, a, c)

	id := uuid.New()
	if _, _, err := s.SendMessage(ctx, store.SendParams{
		ID: id, ConversationID: satu, SenderID: a.ID, Body: "halo",
	}); err != nil {
		t.Fatalf("kiriman pertama: %v", err)
	}

	_, _, err := s.SendMessage(ctx, store.SendParams{
		ID: id, ConversationID: dua, SenderID: a.ID, Body: "halo",
	})
	if !errors.Is(err, store.ErrConflict) {
		t.Fatalf("id yang sama di percakapan lain seharusnya ErrConflict, dapat %v", err)
	}
}

func TestMarkReadTidakPernahMundur(t *testing.T) {
	s, ctx := newStore(t)

	a := newUser(t, s, ctx, "sari")
	b := newUser(t, s, ctx, "tono")
	conv := newDirect(t, s, ctx, a, b)

	send(t, s, ctx, conv, a, "satu")
	dua := send(t, s, ctx, conv, a, "dua")

	if got, err := s.MarkRead(ctx, conv, b.ID, dua.Seq); err != nil || got != dua.Seq {
		t.Fatalf("maju ke seq %d: dapat %d, err %v", dua.Seq, got, err)
	}
	// Pesan yang datang tidak berurutan tidak boleh memunculkan badge lagi.
	if got, err := s.MarkRead(ctx, conv, b.ID, 1); err != nil || got != dua.Seq {
		t.Fatalf("mundur ke seq 1: dapat %d, err %v", got, err)
	}
}

// ---------- pembantu ----------

func lastMessage(t *testing.T, s *store.Store, ctx context.Context, convID, viewer uuid.UUID) store.Message {
	t.Helper()
	page, err := s.ListMessages(ctx, convID, viewer, 0, 50)
	if err != nil {
		t.Fatalf("baca riwayat: %v", err)
	}
	if len(page) == 0 {
		t.Fatal("riwayat kosong")
	}
	return page[len(page)-1]
}

func conversationOf(t *testing.T, s *store.Store, ctx context.Context, userID, convID uuid.UUID) store.Conversation {
	t.Helper()
	list, err := s.ListConversations(ctx, userID)
	if err != nil {
		t.Fatalf("daftar percakapan: %v", err)
	}
	for _, c := range list {
		if c.ID == convID {
			return c
		}
	}
	t.Fatalf("percakapan %s tidak ada di daftar milik %s", convID, userID)
	return store.Conversation{}
}
