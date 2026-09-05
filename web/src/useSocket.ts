import { useCallback, useEffect, useRef } from 'react';
import { wsURL } from './api';
import { useStore } from './store';
import type { ServerEvent } from './types';

/** Jeda reconnect, naik bertahap supaya server tidak dibanjiri saat pulih. */
const BACKOFF_MS = [500, 1000, 2000, 4000, 8000, 15000];

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
          case 'sync.complete':
          case 'error':
            break;
        }
      };

      ws.onclose = () => {
        store().setConnected(false);
        socketRef.current = null;
        if (closedByUs.current) return;

        const delay = BACKOFF_MS[Math.min(attemptRef.current, BACKOFF_MS.length - 1)]!;
        attemptRef.current += 1;
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
