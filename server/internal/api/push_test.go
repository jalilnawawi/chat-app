package api

import (
	"strings"
	"testing"

	"github.com/jalilnawawi/chat-app/server/internal/store"
)

func TestNotificationTextDM(t *testing.T) {
	title, body := notificationText(
		store.ConversationBrief{Type: "direct"},
		store.User{DisplayName: "Rina"},
		store.Message{Body: "jadi ketemu jam 3?"},
	)

	// Di DM, yang ingin diketahui orang saat layar menyala adalah SIAPA.
	if title != "Rina" {
		t.Errorf("judul = %q, mau Rina", title)
	}
	if body != "jadi ketemu jam 3?" {
		t.Errorf("isi = %q", body)
	}
}

func TestNotificationTextGrupMenyebutPengirim(t *testing.T) {
	title, body := notificationText(
		store.ConversationBrief{Type: "group", Title: "Tim Produk"},
		store.User{DisplayName: "Rina"},
		store.Message{Body: "sudah aku kirim"},
	)

	// Nama grup saja tidak memberi tahu apa-apa, jadi pengirim pindah ke isi.
	if title != "Tim Produk" {
		t.Errorf("judul = %q, mau Tim Produk", title)
	}
	if body != "Rina: sudah aku kirim" {
		t.Errorf("isi = %q", body)
	}
}

// Mengirim foto tanpa keterangan adalah hal yang paling biasa dilakukan orang,
// dan notifikasinya tidak boleh kosong.
func TestNotificationTextPesanTanpaTeks(t *testing.T) {
	cases := []struct {
		nama string
		atts []store.Attachment
		mau  string
	}{
		{"satu gambar", []store.Attachment{{MIME: "image/png", Name: "foto.png"}}, "📷 Mengirim gambar"},
		{"satu berkas", []store.Attachment{{MIME: "application/pdf", Name: "kontrak.pdf"}}, "📎 kontrak.pdf"},
		{"banyak", []store.Attachment{
			{MIME: "image/png", Name: "a.png"},
			{MIME: "image/png", Name: "b.png"},
		}, "📎 Mengirim 2 lampiran"},
	}

	for _, c := range cases {
		_, body := notificationText(
			store.ConversationBrief{Type: "direct"},
			store.User{DisplayName: "Rina"},
			store.Message{Body: "   ", Attachments: c.atts},
		)
		if body != c.mau {
			t.Errorf("%s: isi = %q, mau %q", c.nama, body, c.mau)
		}
	}
}

func TestTruncateMemotongPadaBatasRune(t *testing.T) {
	// "é" dua byte. Memotong per byte akan membelahnya dan menghasilkan byte
	// rusak yang tampil sebagai kotak di layar orang.
	got := truncate(strings.Repeat("é", 50), 25)

	if !strings.HasSuffix(got, "…") {
		t.Errorf("hasil = %q, mau berakhiran elipsis", got)
	}
	for _, r := range got {
		if r == '�' {
			t.Fatalf("hasil mengandung rune rusak: %q", got)
		}
	}
}

func TestTruncateMembiarkanTeksPendek(t *testing.T) {
	if got := truncate("halo", 100); got != "halo" {
		t.Errorf("hasil = %q, mau halo", got)
	}
}
