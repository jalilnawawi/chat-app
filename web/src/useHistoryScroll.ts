import { useCallback, useEffect, useLayoutEffect, useRef, useState, type RefObject } from 'react';

/** Sejauh ini dari bawah, pembaca dianggap sedang membaca ke atas. */
const AWAY_PX = 200;

/**
 * Semua aturan gulir riwayat pesan, di satu tempat.
 *
 * 1. Pindah percakapan: langsung ke pesan terbaru.
 * 2. Isian PERTAMA setelah membuka percakapan yang riwayatnya belum ada:
 *    selalu ke bawah. Saat itu posisi gulirnya masih nol, jadi "dekat bawah"
 *    selalu salah, dan tanpa aturan ini percakapan terbuka di pesan terlama.
 * 3. Pesan SENDIRI yang baru dikirim: selalu ke bawah, dari mana pun. Menekan
 *    Enter lalu tidak melihat apa pun adalah kabar yang hilang.
 * 4. Pesan orang lain: ikut turun hanya kalau pembaca memang di dekat bawah —
 *    jangan rebut posisi orang yang sedang membaca ke atas. Selama dia di atas,
 *    jumlah pesan yang datang dihitung untuk pil "pesan baru".
 * 5. Tinggi area baca ATAU isinya berubah (bilah yang datang belakangan,
 *    gambar yang selesai dimuat, papan ketik ponsel): tetap menempel di bawah
 *    bila tadinya di bawah.
 * 6. Pesan baru saja gagal: baris peringatannya setinggi tombol dan bisa jatuh
 *    di bawah tepi layar, jadi yang memicu gulir adalah perubahan STATUS.
 */
export function useHistoryScroll({
  scrollRef,
  contentRef,
  bottomRef,
  roomId,
  messageCount,
  lastMessageId,
  pendingCount,
  failedCount,
}: {
  scrollRef: RefObject<HTMLDivElement | null>;
  contentRef: RefObject<HTMLDivElement | null>;
  bottomRef: RefObject<HTMLDivElement | null>;
  roomId: string | null;
  messageCount: number;
  /**
   * Id pesan paling akhir. Pesan lama yang dimuat ke atas juga menambah
   * jumlah, tapi tidak mengubah ujung riwayat — dan itu bukan "pesan baru".
   */
  lastMessageId: string | null;
  pendingCount: number;
  failedCount: number;
}) {
  const lastMessages = useRef(0);
  const lastTail = useRef<string | null>(null);
  const lastPending = useRef(0);
  const lastFailed = useRef(0);
  const needsInitialScroll = useRef(false);
  const atBottom = useRef(true);
  // Kapan orangnya terakhir menggulir SENDIRI (roda, sentuh, papan ketik,
  // penggeser). Browser juga menggulir sendiri — scroll anchoring saat bilah
  // di atas riwayat muncul — dan gulir itu tidak boleh dianggap "pembaca
  // pergi dari bawah", kalau tidak riwayat berhenti menempel persis saat
  // bilah sematan datang belakangan.
  const lastIntent = useRef(0);
  const [away, setAway] = useState(false);
  const [unseen, setUnseen] = useState(0);

  const toBottom = useCallback(() => {
    bottomRef.current?.scrollIntoView({ block: 'end' });
    atBottom.current = true;
    setAway(false);
    setUnseen(0);
  }, [bottomRef]);

  useLayoutEffect(() => {
    const el = scrollRef.current;
    if (!el) return;
    const tailMoved = lastMessageId !== lastTail.current;
    const newMessages = tailMoved ? messageCount - lastMessages.current : 0;
    const newPending = pendingCount - lastPending.current;
    lastMessages.current = messageCount;
    lastTail.current = lastMessageId;
    lastPending.current = pendingCount;

    if (needsInitialScroll.current && messageCount > 0) {
      needsInitialScroll.current = false;
      toBottom();
      return;
    }
    if (newPending > 0) {
      toBottom();
      return;
    }
    if (newMessages <= 0) return;
    const nearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < AWAY_PX;
    if (nearBottom) toBottom();
    else setUnseen(n => n + newMessages);
  }, [messageCount, pendingCount, lastMessageId]); // eslint-disable-line react-hooks/exhaustive-deps

  useLayoutEffect(() => {
    const el = scrollRef.current;
    const more = failedCount > lastFailed.current;
    lastFailed.current = failedCount;
    if (!el || !more) return;
    if (el.scrollHeight - el.scrollTop - el.clientHeight < 2 * AWAY_PX) toBottom();
  }, [failedCount]); // eslint-disable-line react-hooks/exhaustive-deps

  // Didaftarkan SETELAH dua efek di atas, supaya pada render pindah ruang
  // penghitungnya direset sesudah mereka membaca angka ruang sebelumnya.
  useLayoutEffect(() => {
    lastMessages.current = messageCount;
    lastTail.current = lastMessageId;
    lastPending.current = pendingCount;
    lastFailed.current = failedCount;
    needsInitialScroll.current = messageCount === 0;
    toBottom();
  }, [roomId]); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    const el = scrollRef.current;
    if (!el) return;
    const intent = () => {
      lastIntent.current = performance.now();
    };
    const onScroll = () => {
      const distance = el.scrollHeight - el.scrollTop - el.clientHeight;
      if (distance < 24) atBottom.current = true;
      else if (performance.now() - lastIntent.current < 1000) atBottom.current = false;
      const isAway = distance > AWAY_PX;
      setAway(isAway);
      if (!isAway) setUnseen(0);
    };
    const stick = () => {
      if (atBottom.current) el.scrollTop = el.scrollHeight;
    };
    const observer = new ResizeObserver(stick);
    observer.observe(el);
    if (contentRef.current) observer.observe(contentRef.current);
    const intents = ['wheel', 'touchstart', 'keydown', 'pointerdown'] as const;
    for (const type of intents) el.addEventListener(type, intent, { passive: true });
    el.addEventListener('scroll', onScroll, { passive: true });
    return () => {
      observer.disconnect();
      for (const type of intents) el.removeEventListener(type, intent);
      el.removeEventListener('scroll', onScroll);
    };
  }, [roomId, scrollRef, contentRef]);

  /** Dipanggil sebelum aplikasi sendiri menggulir menjauh (lompat ke pesan). */
  const release = useCallback(() => {
    atBottom.current = false;
  }, []);

  return { away, unseen, toBottom, release };
}
