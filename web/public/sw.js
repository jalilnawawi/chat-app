// Service worker: satu-satunya bagian aplikasi yang tetap hidup saat semua
// tabnya sudah ditutup.
//
// Isinya sengaja sesedikit mungkin. Berkas ini tidak ikut proses build, tidak
// punya dependensi, dan dijalankan oleh browser di luar kendali aplikasi —
// apa pun yang rumit di sini akan sulit diperbaiki dan sulit dilihat saat rusak.
//
// Perhatian: aplikasi ini TIDAK memakai service worker untuk cache. Chat
// realtime tidak punya apa pun yang layak disajikan dari cache basi, dan
// service worker yang menyajikan bundel lama adalah salah satu bug paling
// membingungkan yang bisa dialami seseorang — halaman yang tidak mau berubah
// walaupun sudah di-reload berkali-kali.

self.addEventListener('install', () => {
  // Langsung ambil alih, jangan menunggu semua tab lama ditutup. Tanpa ini,
  // orang yang baru mengaktifkan notifikasi harus menutup seluruh tabnya dulu
  // sebelum notifikasi pertamanya bisa datang.
  self.skipWaiting();
});

self.addEventListener('activate', event => {
  event.waitUntil(self.clients.claim());
});

self.addEventListener('push', event => {
  if (!event.data) return;

  let payload;
  try {
    payload = event.data.json();
  } catch {
    return;
  }

  const title = payload.title || 'Pesan baru';
  const options = {
    body: payload.body || '',
    icon: '/icon.png',
    badge: '/icon.png',

    // tag membuat kabar berikutnya dari percakapan yang sama MENGGANTI yang
    // lama, bukan menumpuk di bawahnya. Sepuluh pesan beruntun dari satu orang
    // meninggalkan satu baris notifikasi, bukan sepuluh.
    tag: payload.conversationId || 'chat',
    renotify: true,

    data: {
      conversationId: payload.conversationId || null,
      messageId: payload.messageId || null,
    },
  };

  event.waitUntil(self.registration.showNotification(title, options));
});

self.addEventListener('notificationclick', event => {
  event.notification.close();
  const conversationId = event.notification.data && event.notification.data.conversationId;

  event.waitUntil(
    (async () => {
      const windows = await self.clients.matchAll({
        type: 'window',
        // Tab yang sedang dimuat pun ikut dihitung, bukan hanya yang sudah
        // selesai. Tanpa ini, mengklik notifikasi saat aplikasi baru saja
        // dibuka akan membuka jendela KEDUA.
        includeUncontrolled: true,
      });

      for (const client of windows) {
        if ('focus' in client) {
          await client.focus();
          // Membuka percakapannya lewat pesan, bukan lewat URL: aplikasi ini
          // tidak memakai rute per percakapan, dan memaksa navigasi akan
          // memuat ulang seluruh halaman beserta koneksi WebSocket-nya.
          client.postMessage({ type: 'open-conversation', conversationId });
          return;
        }
      }

      await self.clients.openWindow(conversationId ? `/?c=${conversationId}` : '/');
    })(),
  );
});
