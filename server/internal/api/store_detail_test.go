package api

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jalilnawawi/chat-app/server/internal/store"
)

// Hanya keterangan yang ditempelkan dengan sengaja — `%w: keterangan` — yang
// boleh sampai ke orang. Error yang membungkus dari arah lain bisa membawa
// isi kueri, dan yang itu dijawab dengan kalimat umum.
func TestStoreDetail(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{fmt.Errorf("%w: paling banyak 20 pesan disematkan", store.ErrConflict), "paling banyak 20 pesan disematkan"},
		{store.ErrConflict, "umum"},
		{fmt.Errorf("simpan: %w", store.ErrConflict), "umum"},
		{fmt.Errorf("insert pinned_messages: %w", fmt.Errorf("%w: rahasia", store.ErrConflict)), "umum"},
	}
	for _, c := range cases {
		if !errors.Is(c.err, store.ErrConflict) {
			t.Fatalf("kasus uji salah susun: %v", c.err)
		}
		if got := storeDetail(c.err, store.ErrConflict, "umum"); got != c.want {
			t.Errorf("%v: dapat %q, mau %q", c.err, got, c.want)
		}
	}
}
