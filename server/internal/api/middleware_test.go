package api

import (
	"net/http/httptest"
	"testing"
)

// Label metrik harus berkardinalitas rendah. Kalau id percakapan bocor jadi
// label, Prometheus menyimpan satu deret waktu per percakapan yang pernah
// dibuka — cara klasik sistem monitoring menjatuhkan sistem yang dipantaunya.
func TestRouteLabelCollapsesIdentifiers(t *testing.T) {
	cases := map[string]string{
		"/api/conversations": "/api/conversations",
		"/api/conversations/018f3a2b-1c4d-7e8f-9a0b-1c2d3e4f5a6b/messages": "/api/conversations/{id}/messages",
		"/api/messages/018f3a2b-1c4d-7e8f-9a0b-1c2d3e4f5a6b":               "/api/messages/{id}",
		"/api/conversations/018f3a2b-1c4d-7e8f-9a0b-1c2d3e4f5a6b/read":     "/api/conversations/{id}/read",
		"/ws": "/ws",
	}

	for path, want := range cases {
		if got := routeLabel(path); got != want {
			t.Errorf("routeLabel(%q) = %q, harusnya %q", path, got, want)
		}
	}
}

// Di belakang load balancer, RemoteAddr berisi alamat proxy — satu nilai untuk
// semua orang. Tanpa membaca X-Forwarded-For, kuota per-IP diam-diam berubah
// jadi kuota global dan satu penyerang bisa mengunci semua orang.
func TestClientIPPrefersForwardedHeader(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.0.0.1:5555"

	if got := clientIP(r); got != "10.0.0.1" {
		t.Fatalf("tanpa header = %q, harusnya alamat asli 10.0.0.1", got)
	}

	r.Header.Set("X-Forwarded-For", "203.0.113.7, 70.41.3.18")
	if got := clientIP(r); got != "203.0.113.7" {
		t.Fatalf("dengan header = %q, harusnya client paling depan 203.0.113.7", got)
	}
}
