import { useCallback, useEffect, useRef } from 'react';
import { wsURL } from './api';
import { useStore } from './store';
import type { ServerEvent } from './types';

/** Jeda reconnect, naik bertahap supaya server tidak dibanjiri saat pulih. */
const BACKOFF_MS = [500, 1000, 2000, 4000, 8000, 15000];

/**
 * Jendela reconnect saat server pamit terencana.
 *
 * Sengaja pendek — instance pengganti sudah siap, tidak ada gunanya menunggu —
 * tapi tetap diacak. Server sudah menyebar penutupan koneksinya; kalau client
 * membalas dengan jeda yang seragam, sebaran itu dirapikan kembali jadi barisan
 * dan instance baru tetap menerima lonjakan yang sama.
 */
const RESTART_RECONNECT_MS = [200, 1200];

/** Jeda acak dalam satu rentang, dibulatkan ke milidetik. */
const between = ([lo, hi]: readonly [number, number] | number[]) =>
  lo! + Math.random() * (hi! - lo!);

/**
 * Koneksi WebSocket tunggal untuk seluruh aplikasi.
 *
 * Tiga hal yang membuatnya terasa "tidak pernah putus":
 *  - reconnect otomatis dengan jeda bertahap
 *  - setelah tersambung, kirim `sync` berisi seq terakhir yang dipunya tiap
 *    percakapan; server membalas hanya yang terlewat
 *  - browser sudah mengirim cookie sesi sendiri saat handshake, jadi tidak ada
 *    token yang perlu ditempel di URL
 */
export function useSocket(enabled: boolean) {
  const socketRef = useRef<WebSocket | null>(null);
  const attemptRef = useRef(0);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const closedByUs = useRef(false);
  const plannedRestart = useRef(false);

  const send = useCallback((type: string, payload: unknown) => {
    const ws = socketRef.current;
    if (ws?.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type, payload }));
    }
  }, []);

  useEffect(() => {
    if (!enabled) return;
    closedByUs.current = false;

    const store = useStore.getState;

    const connect = () => {
      const ws = new WebSocket(wsURL());
      socketRef.current = ws;

      ws.onopen = () => {
        attemptRef.current = 0;
        plannedRestart.current = false;
        store().setConnected(true);
        // Susulkan apa pun yang terlewat selama koneksi putus.
        ws.send(JSON.stringify({ type: 'sync', payload: { cursors: store().syncCursors() } }));
      };

      ws.onmessage = e => {
        const ev = JSON.parse(e.data as string) as ServerEvent;
        const s = store();

        switch (ev.type) {
          case 'message.new':
          case 'message.updated':
            s.applyMessage(ev.payload);
            if (ev.type === 'message.new' && ev.payload.conversationId === s.activeId) {
              // Kalau ruangnya sedang dibuka, langsung tandai terbaca.
              s.markReadUpTo(ev.payload.conversationId);
            }
            break;
          case 'sync.batch':
            // Susulan setelah reconnect. Diterapkan satu per satu seperti pesan
            // biasa; yang berbeda cuma cara datangnya — sekaligus, bukan satu
            // frame per pesan.
            for (const m of ev.payload.messages) s.applyMessage(m);
            if (ev.payload.conversationId === s.activeId) {
              s.markReadUpTo(ev.payload.conversationId);
            }
            break;
          case 'conversation.new':
            s.applyConversation(ev.payload);
            break;
          case 'read.updated':
            s.applyRead(ev.payload.conversationId, ev.payload.userId, ev.payload.lastReadSeq);
            break;
          case 'typing':
            s.applyTyping(
              ev.payload.conversationId,
              ev.payload.userId,
              ev.payload.displayName,
              ev.payload.typing,
            );
            break;
          case 'presence':
            s.setPresence(ev.payload.userId, ev.payload.online);
            break;
          case 'presence.snapshot':
            s.setOnline(ev.payload.online);
            break;
          case 'server.shutdown':
            // Penutupannya menyusul sebentar lagi; yang dicatat di sini cuma
            // SEBABNYA, supaya onclose tahu ini pamit terencana, bukan jaringan
            // yang bermasalah.
            plannedRestart.current = true;
            break;
          case 'sync.complete':
          case 'error':
            break;
        }
      };

      ws.onclose = () => {
        store().setConnected(false);
        socketRef.current = null;
        if (closedByUs.current) return;

        let delay: number;
        if (plannedRestart.current) {
          // Deploy terencana tidak menghabiskan jatah backoff: kalau instance
          // pengganti ternyata belum siap, percobaan berikutnya baru mundur
          // bertahap seperti biasa.
          plannedRestart.current = false;
          delay = between(RESTART_RECONNECT_MS);
        } else {
          const base = BACKOFF_MS[Math.min(attemptRef.current, BACKOFF_MS.length - 1)]!;
          attemptRef.current += 1;
          // Jeda diacak ±25%. Saat server pulih dari gangguan, semua client
          // yang putus bersamaan akan mencoba lagi bersamaan juga — dan
          // menjatuhkannya lagi tepat saat baru bangun.
          delay = base * (0.75 + Math.random() * 0.5);
        }
        timerRef.current = setTimeout(connect, delay);
      };

      // onerror selalu disusul onclose, jadi reconnect cukup ditangani di sana.
      ws.onerror = () => ws.close();
    };

    connect();

    return () => {
      closedByUs.current = true;
      if (timerRef.current) clearTimeout(timerRef.current);
      socketRef.current?.close();
      socketRef.current = null;
    };
  }, [enabled]);

  return { send };
}
