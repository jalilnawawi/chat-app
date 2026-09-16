package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/jalilnawawi/chat-app/server/internal/store"
)

// Test untuk jalur Fase 7 yang paling mudah rusak diam-diam.
//
// Izin baca lampiran ditulis sebagai satu klausa WHERE di dalam query, bukan
// sebagai `if` di handler — dan itu keputusan yang benar, karena query itulah
// satu-satunya jalan menuju byte lampiran. Konsekuensinya: kesalahan di sana
// tidak akan pernah dilaporkan compiler, dan bentuk kegagalannya adalah berkas
// orang lain yang terkirim dengan status 200.

func TestLampiranHanyaBisaDibacaAnggota(t *testing.T) {
	s, ctx := newStore(t)

	a := newUser(t, s, ctx, "lampir_a")
	b := newUser(t, s, ctx, "lampir_b")
	luar := newUser(t, s, ctx, "lampir_luar")
	conv := newDirect(t, s, ctx, a, b)

	att := newAttachment(t, s, ctx, a, "foto.png", "image/png")

	m, _, err := s.SendMessage(ctx, store.SendParams{
		ID: uuid.New(), ConversationID: conv, SenderID: a.ID,
		Body: "ini fotonya", AttachmentIDs: []uuid.UUID{att},
	})
	if err != nil {
		t.Fatalf("kirim dengan lampiran: %v", err)
	}
	if len(m.Attachments) != 1 || m.Attachments[0].ID != att {
		t.Fatalf("lampiran tidak terpasang: %+v", m.Attachments)
	}

	for _, orang := range []store.User{a, b} {
		if _, err := s.AttachmentForRead(ctx, att, orang.ID); err != nil {
			t.Fatalf("anggota %s tidak bisa membaca lampiran: %v", orang.DisplayName, err)
		}
	}
	if _, err := s.AttachmentForRead(ctx, att, luar.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("orang luar bisa membaca lampiran (err %v)", err)
	}
}

// Lampiran yang belum terpasang ke pesan mana pun hanya boleh dibaca
// pengunggahnya — supaya pratinjau sebelum kirim tetap jalan tanpa membuka
// berkas itu untuk orang lain.
func TestLampiranYatimHanyaUntukPemiliknya(t *testing.T) {
	s, ctx := newStore(t)

	a := newUser(t, s, ctx, "yatim_a")
	b := newUser(t, s, ctx, "yatim_b")
	newDirect(t, s, ctx, a, b)

	att := newAttachment(t, s, ctx, a, "draft.pdf", "application/pdf")

	if _, err := s.AttachmentForRead(ctx, att, a.ID); err != nil {
		t.Fatalf("pengunggah tidak bisa membaca lampirannya sendiri: %v", err)
	}
	// b ada di percakapan yang sama, tapi lampirannya belum pernah dikirim.
	if _, err := s.AttachmentForRead(ctx, att, b.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("lampiran yang belum dikirim bisa dibaca orang lain (err %v)", err)
	}
}

// Tiga syarat dipaksakan oleh satu UPDATE di claimAttachments: milik pengirim,
// belum pernah dipakai, dan memang ada. Ketiganya diperiksa di sini karena
// ketiganya berakhir pada jawaban yang sama, dan jawaban yang sama adalah
// tempat di mana satu syarat bisa hilang tanpa terlihat.
func TestLampiranTidakBisaDipakaiUlangAtauDicuri(t *testing.T) {
	s, ctx := newStore(t)

	a := newUser(t, s, ctx, "pakai_a")
	b := newUser(t, s, ctx, "pakai_b")
	conv := newDirect(t, s, ctx, a, b)

	att := newAttachment(t, s, ctx, a, "sekali.png", "image/png")

	// Milik orang lain.
	if _, _, err := s.SendMessage(ctx, store.SendParams{
		ID: uuid.New(), ConversationID: conv, SenderID: b.ID,
		Body: "punyamu aku pakai", AttachmentIDs: []uuid.UUID{att},
	}); !errors.Is(err, store.ErrForbidden) {
		t.Fatalf("memakai lampiran orang lain seharusnya ErrForbidden, dapat %v", err)
	}

	// Tidak ada.
	if _, _, err := s.SendMessage(ctx, store.SendParams{
		ID: uuid.New(), ConversationID: conv, SenderID: a.ID,
		Body: "hantu", AttachmentIDs: []uuid.UUID{uuid.New()},
	}); !errors.Is(err, store.ErrForbidden) {
		t.Fatalf("lampiran yang tidak ada seharusnya ErrForbidden, dapat %v", err)
	}

	if _, _, err := s.SendMessage(ctx, store.SendParams{
		ID: uuid.New(), ConversationID: conv, SenderID: a.ID,
		Body: "pertama", AttachmentIDs: []uuid.UUID{att},
	}); err != nil {
		t.Fatalf("kiriman pertama: %v", err)
	}

	// Sudah dipakai.
	if _, _, err := s.SendMessage(ctx, store.SendParams{
		ID: uuid.New(), ConversationID: conv, SenderID: a.ID,
		Body: "kedua", AttachmentIDs: []uuid.UUID{att},
	}); !errors.Is(err, store.ErrForbidden) {
		t.Fatalf("memakai ulang lampiran seharusnya ErrForbidden, dapat %v", err)
	}
}

// Menghapus pesan melepas lampirannya kembali jadi yatim, supaya penyapu yang
// sudah ada bisa membuangnya — dan sejak saat itu anggota lain tidak bisa lagi
// membacanya.
func TestHapusPesanMelepasLampirannya(t *testing.T) {
	s, ctx := newStore(t)

	a := newUser(t, s, ctx, "hapus_a")
	b := newUser(t, s, ctx, "hapus_b")
	conv := newDirect(t, s, ctx, a, b)

	att := newAttachment(t, s, ctx, a, "salah.png", "image/png")
	m, _, err := s.SendMessage(ctx, store.SendParams{
		ID: uuid.New(), ConversationID: conv, SenderID: a.ID,
		Body: "", AttachmentIDs: []uuid.UUID{att},
	})
	if err != nil {
		t.Fatalf("kirim: %v", err)
	}

	if _, err := s.DeleteMessage(ctx, m.ID, a.ID); err != nil {
		t.Fatalf("hapus: %v", err)
	}

	if _, err := s.AttachmentForRead(ctx, att, b.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("lampiran pesan yang dihapus masih terbaca anggota lain (err %v)", err)
	}
	// Pemiliknya masih bisa — barisnya kembali ke keadaan yatim, bukan hilang,
	// dan penyapulah yang akan membuangnya setelah lewat umur.
	if _, err := s.AttachmentForRead(ctx, att, a.ID); err != nil {
		t.Fatalf("pengunggah kehilangan lampirannya sendiri: %v", err)
	}
}

func newAttachment(t *testing.T, s *store.Store, ctx context.Context, owner store.User, name, mime string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	err := s.CreateAttachment(ctx, owner.ID, store.StoredAttachment{
		Attachment: store.Attachment{ID: id, Name: name, MIME: mime, Size: 1234},
		Key:        "/uji/" + id.String(),
	})
	if err != nil {
		t.Fatalf("catat lampiran: %v", err)
	}
	return id
}
