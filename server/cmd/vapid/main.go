// Command vapid membuat sepasang kunci VAPID untuk push notification.
//
// Dipisah sebagai perintah sendiri, bukan dibuat otomatis saat server start,
// karena kunci ini harus TETAP. Browser mengunci langganannya pada kunci publik
// yang dipakai saat mendaftar: kunci baru membuat setiap langganan yang sudah
// ada berhenti bekerja diam-diam — tidak ada error, notifikasinya saja tidak
// pernah sampai lagi. Server yang membuat kunci baru tiap kali restart adalah
// server yang notifikasinya rusak setiap deploy.
//
// Jalankan sekali, simpan hasilnya di .env:
//
//	go run ./cmd/vapid
package main

import (
	"fmt"
	"os"

	webpush "github.com/SherClockHolmes/webpush-go"
)

func main() {
	private, public, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		fmt.Fprintln(os.Stderr, "gagal membuat kunci:", err)
		os.Exit(1)
	}

	fmt.Println("# Salin ke .env. Kunci privat tidak boleh ikut masuk git.")
	fmt.Println("VAPID_PUBLIC_KEY=" + public)
	fmt.Println("VAPID_PRIVATE_KEY=" + private)
	fmt.Println("VAPID_SUBJECT=mailto:admin@example.com")
}
