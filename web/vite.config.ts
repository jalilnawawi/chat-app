import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';

const BACKEND = process.env.BACKEND_URL ?? 'http://127.0.0.1:8090';

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    // 5174 karena 5173 sering sudah dipakai project lain.
    port: 5174,
    strictPort: true,

    // '::' membuat Node mendengarkan dual-stack: localhost yang di-resolve ke
    // ::1 MAUPUN ke 127.0.0.1 sama-sama tersambung. Default Vite hanya mengikat
    // IPv6 loopback, sehingga browser yang memilih IPv4 lebih dulu gagal konek.
    host: '::',

    // Backend diteruskan lewat origin yang sama dengan halaman.
    //
    // Ini bukan sekadar kenyamanan. Kalau frontend di 127.0.0.1:5174 memanggil
    // API di localhost:8090, browser menganggapnya LINTAS SITE — "localhost"
    // dan "127.0.0.1" adalah site berbeda walau menunjuk mesin yang sama —
    // sehingga cookie sesi SameSite=Lax tidak ikut terkirim dan semua
    // permintaan setelah login jadi 401.
    //
    // Dengan proxy, apa pun yang diketik di address bar (localhost, 127.0.0.1,
    // atau IP LAN) selalu satu origin: cookie jalan, CORS tidak terlibat, dan
    // pemeriksaan origin WebSocket otomatis lolos.
    proxy: {
      '/api': { target: BACKEND },
      '/ws': { target: BACKEND, ws: true },
    },
  },
});
