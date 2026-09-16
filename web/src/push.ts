import { useCallback, useEffect, useState } from 'react';
import { api } from './api';

/**
 * Push notification di sisi browser.
 *
 * Tiga hal yang membuat bagian ini lebih rumit dari kelihatannya:
 *
 *  1. Izin notifikasi hanya boleh diminta sebagai jawaban atas tindakan orang.
 *     Browser modern menghukum situs yang meminta izin saat halaman dibuka —
 *     Firefox menolaknya diam-diam, Chrome menghitungnya sebagai gangguan.
 *     Karena itu tidak ada permintaan izin otomatis di sini; semuanya berangkat
 *     dari satu tombol.
 *  2. Kunci publik VAPID diminta ke server, tidak ditulis di sini. Kunci adalah
 *     urusan deployment, dan kunci yang tertinggal di bundel frontend akan jadi
 *     kunci yang salah begitu server diganti.
 *  3. Langganan bisa hilang tanpa memberi tahu siapa pun — browser mencabutnya
 *     saat data situs dibersihkan. Karena itu keadaan sebenarnya selalu dibaca
 *     dari browser, bukan disimpan sendiri.
 */

export const pushSupported = () =>
  'serviceWorker' in navigator && 'PushManager' in window && 'Notification' in window;

/**
 * Mendaftarkan service worker.
 *
 * Dipanggil sekali saat aplikasi siap, terpisah dari permintaan izin: mendaftar
 * tidak menampilkan apa pun ke orang, dan service worker yang sudah siap
 * membuat penekanan tombol nanti terasa seketika.
 */
export async function registerServiceWorker(): Promise<ServiceWorkerRegistration | null> {
  if (!pushSupported()) return null;
  try {
    return await navigator.serviceWorker.register('/sw.js');
  } catch {
    return null;
  }
}

/**
 * Kunci VAPID datang sebagai base64url, sedangkan PushManager menuntut byte
 * mentah. Konversinya harus dilakukan sendiri — tidak ada API browser untuk ini.
 */
function decodeKey(base64url: string): Uint8Array<ArrayBuffer> {
  const padded = base64url.padEnd(base64url.length + ((4 - (base64url.length % 4)) % 4), '=');
  const raw = atob(padded.replace(/-/g, '+').replace(/_/g, '/'));

  // Buffer dibuat eksplisit sebagai ArrayBuffer: PushManager menolak view di
  // atas SharedArrayBuffer, dan Uint8Array polos bisa jadi salah satunya di
  // mata TypeScript.
  const bytes = new Uint8Array(new ArrayBuffer(raw.length));
  for (let i = 0; i < raw.length; i++) bytes[i] = raw.charCodeAt(i);
  return bytes;
}

/**
 * Mencabut langganan perangkat ini, tanpa React.
 *
 * Dipanggil saat logout. Tanpa ini, langganan orang sebelumnya tetap hidup di
 * komputer yang dipakai bergantian, dan pratinjau pesannya terus muncul di layar
 * kunci orang berikutnya — kebocoran yang tidak pernah berhenti sendiri, karena
 * tidak ada apa pun di aplikasi yang akan mencabutnya nanti.
 */
export async function unsubscribeThisDevice(): Promise<void> {
  if (!pushSupported()) return;
  try {
    const registration = await navigator.serviceWorker.getRegistration();
    const sub = await registration?.pushManager.getSubscription();
    if (!sub) return;

    // Server dulu, browser belakangan — sama seperti di tombol notifikasi.
    await api.pushUnsubscribe(sub.endpoint).catch(() => {});
    await sub.unsubscribe();
  } catch {
    // Logout tidak boleh gagal gara-gara ini.
  }
}

type PushState = {
  /** Browser ini mendukung Web Push sama sekali. */
  supported: boolean;
  /** Server ini menyalakan notifikasi. */
  available: boolean;
  /** Orang ini sudah berlangganan di browser ini. */
  enabled: boolean;
  /** Izin sudah ditolak permanen — tombol tidak akan menolong. */
  blocked: boolean;
  busy: boolean;
  error: string | null;
};

export function usePush() {
  const [state, setState] = useState<PushState>({
    supported: pushSupported(),
    available: false,
    enabled: false,
    blocked: false,
    busy: false,
    error: null,
  });

  // Keadaan awal dibaca dari browser, bukan diingat sendiri: langganan bisa
  // dicabut dari pengaturan situs tanpa aplikasi ini pernah tahu.
  useEffect(() => {
    if (!pushSupported()) return;
    let cancelled = false;

    void (async () => {
      const [config, registration] = await Promise.all([
        api.serverConfig().catch(() => null),
        navigator.serviceWorker.ready,
      ]);
      const sub = await registration.pushManager.getSubscription();
      if (cancelled) return;

      setState(s => ({
        ...s,
        available: config?.push ?? false,
        enabled: sub !== null,
        blocked: Notification.permission === 'denied',
      }));
    })();

    return () => {
      cancelled = true;
    };
  }, []);

  const enable = useCallback(async () => {
    setState(s => ({ ...s, busy: true, error: null }));
    try {
      // Ditanyakan lagi, bukan dipakai dari keadaan awal: antara halaman dibuka
      // dan tombol ditekan bisa lewat berjam-jam, dan kunci yang dipakai
      // mendaftar harus kunci yang BENAR-BENAR dipegang server saat ini.
      const config = await api.serverConfig();
      if (!config.push) throw new Error('Notifikasi tidak aktif di server ini');

      const permission = await Notification.requestPermission();
      if (permission !== 'granted') {
        setState(s => ({
          ...s,
          busy: false,
          blocked: permission === 'denied',
          error: 'Izin notifikasi tidak diberikan',
        }));
        return;
      }

      const registration = await navigator.serviceWorker.ready;
      const sub = await registration.pushManager.subscribe({
        // Wajib true, dan browser memang tidak mengizinkan yang sebaliknya:
        // langganan yang bisa dipakai diam-diam tanpa menampilkan apa pun ke
        // orangnya adalah pelacak, bukan notifikasi.
        userVisibleOnly: true,
        applicationServerKey: decodeKey(config.vapidPublicKey),
      });

      await api.pushSubscribe(sub.toJSON());
      setState(s => ({ ...s, busy: false, enabled: true }));
    } catch (err) {
      setState(s => ({
        ...s,
        busy: false,
        error: err instanceof Error ? err.message : 'Gagal menyalakan notifikasi',
      }));
    }
  }, []);

  const disable = useCallback(async () => {
    setState(s => ({ ...s, busy: true, error: null }));
    try {
      // Urutannya — server dulu, browser belakangan — ada di satu tempat.
      // Urutan sebaliknya menyisakan baris di server yang menunjuk langganan
      // yang sudah tidak ada: alamat mati yang baru ketahuan saat notifikasi
      // pertama gagal.
      await unsubscribeThisDevice();
      setState(s => ({ ...s, busy: false, enabled: false }));
    } catch (err) {
      setState(s => ({
        ...s,
        busy: false,
        error: err instanceof Error ? err.message : 'Gagal mematikan notifikasi',
      }));
    }
  }, []);

  const toggle = useCallback(() => {
    void (state.enabled ? disable() : enable());
  }, [state.enabled, enable, disable]);

  return { ...state, toggle };
}
