package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/jalilnawawi/chat-app/server/internal/store"
)

// Bentuk primary key-nya yang memaksakan "satu orang, satu emoji, satu kali" —
// bukan kode aplikasi. Menekan tombol yang sama dua kali karena jaringan lambat
// harus menghasilkan keadaan yang sama, bukan hitungan ganda.
func TestReaksiKembarTidakMenambahHitungan(t *testing.T) {
	s, ctx := newStore(t)

	a := newUser(t, s, ctx, "udin")
	b := newUser(t, s, ctx, "vina")
	conv := newDirect(t, s, ctx, a, b)
	m := send(t, s, ctx, conv, a, "lucu banget")

	pertama, err := s.AddReaction(ctx, m.ID, b.ID, "👍")
	if err != nil {
		t.Fatalf("pasang reaksi: %v", err)
	}
	if !pertama.Changed {
		t.Fatal("reaksi pertama dilaporkan tidak mengubah apa pun")
	}

	kedua, err := s.AddReaction(ctx, m.ID, b.ID, "👍")
	if err != nil {
		t.Fatalf("pasang reaksi kedua kali: %v", err)
	}
	if kedua.Changed {
		t.Fatal("emoji yang sama dari orang yang sama dilaporkan sebagai perubahan")
	}
	// Jam tidak boleh maju untuk sesuatu yang tidak berubah: memajukannya
	// membuat setiap client menyusul kiriman yang isinya sama persis dengan
	// yang sudah dia punya.
	if kedua.ReactionSeq != 0 {
		t.Fatalf("jam reaksi maju untuk penekanan yang tidak mengubah apa pun: %d", kedua.ReactionSeq)
	}

	ringkas := reactionsOf(t, s, ctx, conv, b.ID, m.ID)
	if len(ringkas) != 1 || ringkas[0].Count != 1 {
		t.Fatalf("ringkasan salah: %+v", ringkas)
	}
}

func TestRingkasanReaksiMenghitungDanTahuMilikSiapa(t *testing.T) {
	s, ctx := newStore(t)

	a := newUser(t, s, ctx, "wawan")
	b := newUser(t, s, ctx, "yanti")
	c := newUser(t, s, ctx, "zaki")
	grup := newGroup(t, s, ctx, a, b, c)
	m := send(t, s, ctx, grup, a, "selesai")

	for _, u := range []store.User{a, b, c} {
		if _, err := s.AddReaction(ctx, m.ID, u.ID, "🎉"); err != nil {
			t.Fatalf("pasang reaksi %s: %v", u.DisplayName, err)
		}
	}
	if _, err := s.AddReaction(ctx, m.ID, b.ID, "❤️"); err != nil {
		t.Fatalf("pasang emoji kedua: %v", err)
	}

	// "Mine" adalah satu-satunya bagian sebuah pesan yang jawabannya berbeda
	// per pembaca, jadi dia diperiksa dari dua sudut.
	dariB := reactionsOf(t, s, ctx, grup, b.ID, m.ID)
	if len(dariB) != 2 {
		t.Fatalf("seharusnya dua emoji berbeda, dapat %+v", dariB)
	}
	if dariB[0].Emoji != "🎉" || dariB[0].Count != 3 || !dariB[0].Mine {
		t.Fatalf("ringkasan untuk b salah: %+v", dariB[0])
	}

	dariC := reactionsOf(t, s, ctx, grup, c.ID, m.ID)
	for _, r := range dariC {
		if r.Emoji == "❤️" && r.Mine {
			t.Fatal("reaksi milik orang lain tercatat sebagai milik pembaca")
		}
	}
}

// Mencabut hanya menyentuh milik sendiri. Baris yang dihapus selalu dibatasi
// user_id pemanggil — tanpa itu, siapa pun bisa menghapus emoji orang lain.
func TestCabutReaksiTidakMenyentuhMilikOrangLain(t *testing.T) {
	s, ctx := newStore(t)

	a := newUser(t, s, ctx, "adit")
	b := newUser(t, s, ctx, "bela")
	conv := newDirect(t, s, ctx, a, b)
	m := send(t, s, ctx, conv, a, "oke")

	if _, err := s.AddReaction(ctx, m.ID, a.ID, "👍"); err != nil {
		t.Fatalf("pasang: %v", err)
	}

	// b mencabut emoji yang tidak pernah dia pasang.
	change, err := s.RemoveReaction(ctx, m.ID, b.ID, "👍")
	if err != nil {
		t.Fatalf("cabut: %v", err)
	}
	if change.Changed {
		t.Fatal("mencabut emoji milik orang lain dilaporkan berhasil")
	}

	ringkas := reactionsOf(t, s, ctx, conv, a.ID, m.ID)
	if len(ringkas) != 1 || ringkas[0].Count != 1 {
		t.Fatalf("reaksi orang lain ikut terhapus: %+v", ringkas)
	}
}

// Orang di luar percakapan tidak boleh bereaksi, dan jawabannya sama dengan
// jawaban untuk pesan yang memang tidak ada — membedakan keduanya berarti
// memberi tahu orang asing bahwa pesan itu eksis.
func TestReaksiOrangLuarDitolak(t *testing.T) {
	s, ctx := newStore(t)

	a := newUser(t, s, ctx, "cahya")
	b := newUser(t, s, ctx, "dewi")
	luar := newUser(t, s, ctx, "elang")
	conv := newDirect(t, s, ctx, a, b)
	m := send(t, s, ctx, conv, a, "rahasia")

	if _, err := s.AddReaction(ctx, m.ID, luar.ID, "👍"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("orang luar seharusnya ErrNotFound, dapat %v", err)
	}
	if _, err := s.AddReaction(ctx, uuid.New(), a.ID, "👍"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("pesan tak ada seharusnya ErrNotFound, dapat %v", err)
	}
}

func TestPesanDihapusTidakBisaDireaksiDanReaksinyaIkutHilang(t *testing.T) {
	s, ctx := newStore(t)

	a := newUser(t, s, ctx, "fajar")
	b := newUser(t, s, ctx, "gina")
	conv := newDirect(t, s, ctx, a, b)
	m := send(t, s, ctx, conv, a, "salah kirim")

	if _, err := s.AddReaction(ctx, m.ID, b.ID, "😂"); err != nil {
		t.Fatalf("pasang: %v", err)
	}
	if _, err := s.DeleteMessage(ctx, m.ID, a.ID); err != nil {
		t.Fatalf("hapus pesan: %v", err)
	}

	if ringkas := reactionsOf(t, s, ctx, conv, b.ID, m.ID); len(ringkas) != 0 {
		t.Fatalf("reaksi masih menempel pada pesan yang dihapus: %+v", ringkas)
	}
	if _, err := s.AddReaction(ctx, m.ID, b.ID, "😂"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("bereaksi pada pesan yang dihapus seharusnya ErrNotFound, dapat %v", err)
	}
}

// Inti dari jam kedua: reaksi pada pesan LAMA harus tetap menyusul setelah
// reconnect, termasuk PENCABUTAN — yang barisnya sudah tidak ada lagi untuk
// diceritakan.
func TestReaksiMenyusulSetelahOffline(t *testing.T) {
	s, ctx := newStore(t)

	a := newUser(t, s, ctx, "hana")
	b := newUser(t, s, ctx, "irfan")
	conv := newDirect(t, s, ctx, a, b)

	lama := send(t, s, ctx, conv, a, "pesan kemarin")

	// Keadaan saat client masih tersambung: sudah tahu semuanya.
	awal, err := s.ReactionsSince(ctx, conv, b.ID, 0, 100)
	if err != nil {
		t.Fatalf("baca susulan awal: %v", err)
	}
	if len(awal) != 0 {
		t.Fatalf("belum ada reaksi, tapi sudah ada susulan: %+v", awal)
	}

	pasang, err := s.AddReaction(ctx, lama.ID, a.ID, "👍")
	if err != nil {
		t.Fatalf("pasang: %v", err)
	}

	susulan, err := s.ReactionsSince(ctx, conv, b.ID, 0, 100)
	if err != nil {
		t.Fatalf("susulan setelah pasang: %v", err)
	}
	if len(susulan) != 1 || len(susulan[0].Reactions) != 1 || susulan[0].Reactions[0].Count != 1 {
		t.Fatalf("susulan setelah pasang salah: %+v", susulan)
	}
	if susulan[0].ReactionSeq != pasang.ReactionSeq {
		t.Fatalf("jam susulan %d tidak cocok dengan jam perubahan %d",
			susulan[0].ReactionSeq, pasang.ReactionSeq)
	}

	// Client sudah menyusul sampai sini; setelah ini dia putus lagi.
	cursor := susulan[0].ReactionSeq
	if kosong, err := s.ReactionsSince(ctx, conv, b.ID, cursor, 100); err != nil || len(kosong) != 0 {
		t.Fatalf("susulan berulang tanpa perubahan: %+v, err %v", kosong, err)
	}

	// Pencabutan menghapus barisnya. Tanpa jam yang tetap maju, tidak ada
	// apa pun yang tersisa untuk memberi tahu client bahwa hitungannya turun.
	if _, err := s.RemoveReaction(ctx, lama.ID, a.ID, "👍"); err != nil {
		t.Fatalf("cabut: %v", err)
	}

	setelahCabut, err := s.ReactionsSince(ctx, conv, b.ID, cursor, 100)
	if err != nil {
		t.Fatalf("susulan setelah cabut: %v", err)
	}
	if len(setelahCabut) != 1 {
		t.Fatalf("pencabutan tidak ikut menyusul: %+v", setelahCabut)
	}
	if len(setelahCabut[0].Reactions) != 0 {
		t.Fatalf("pesan yang reaksinya habis seharusnya menyusul dengan daftar kosong: %+v",
			setelahCabut[0].Reactions)
	}
}

// Riwayat mengisi reaksinya sendiri, jadi client yang baru membuka percakapan
// tidak perlu bertanya dua kali.
func TestRiwayatMembawaReaksinyaSendiri(t *testing.T) {
	s, ctx := newStore(t)

	a := newUser(t, s, ctx, "jamal")
	b := newUser(t, s, ctx, "kirana")
	conv := newDirect(t, s, ctx, a, b)

	tanpa := send(t, s, ctx, conv, a, "biasa saja")
	dengan := send(t, s, ctx, conv, a, "yang ini dikomentari")
	if _, err := s.AddReaction(ctx, dengan.ID, b.ID, "🔥"); err != nil {
		t.Fatalf("pasang: %v", err)
	}

	page, err := s.ListMessages(ctx, conv, b.ID, 0, 50)
	if err != nil {
		t.Fatalf("baca riwayat: %v", err)
	}
	for _, m := range page {
		switch m.ID {
		case tanpa.ID:
			if len(m.Reactions) != 0 {
				t.Fatalf("pesan tanpa reaksi membawa ringkasan: %+v", m.Reactions)
			}
			if m.ReactionSeq != 0 {
				t.Fatalf("pesan tanpa reaksi punya jam %d", m.ReactionSeq)
			}
		case dengan.ID:
			if len(m.Reactions) != 1 || !m.Reactions[0].Mine {
				t.Fatalf("ringkasan riwayat salah: %+v", m.Reactions)
			}
			if m.ReactionSeq == 0 {
				t.Fatal("pesan yang direaksikan tidak membawa jam reaksinya")
			}
		}
	}
}

// ---------- pembantu ----------

func reactionsOf(t *testing.T, s *store.Store, ctx context.Context, convID, viewer, msgID uuid.UUID) []store.ReactionSummary {
	t.Helper()
	page, err := s.ListMessages(ctx, convID, viewer, 0, 100)
	if err != nil {
		t.Fatalf("baca riwayat: %v", err)
	}
	for _, m := range page {
		if m.ID == msgID {
			return m.Reactions
		}
	}
	t.Fatalf("pesan %s tidak ada di riwayat", msgID)
	return nil
}
