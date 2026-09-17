import { useEffect, useLayoutEffect, type KeyboardEvent, type RefObject } from 'react';

/** Semua yang bisa menerima fokus di dalam sebuah baris riwayat. */
const FOCUSABLE = 'a[href], button, input, select, textarea, video[controls], audio[controls], [tabindex]';

/**
 * Riwayat pesan sebagai SATU pemberhentian Tab.
 *
 * Setiap baris (`[data-msg]`) bisa difokus, tapi hanya satu yang ada di urutan
 * Tab: yang terakhir dipilih, atau pesan terbaru. Panah atas/bawah berpindah
 * baris, Home/End ke ujung, Enter atau tombol menu membuka tindakan baris itu.
 *
 * Tombol dan tautan DI DALAM baris hanya ikut urutan Tab untuk baris yang
 * terpilih, dan hanya selama fokus sudah berada di baris itu. Dengan begitu
 * Shift+Tab dari kolom tulis mendarat tepat di baris pesan, bukan di tombol
 * reaksinya; Tab dari baris itu baru masuk ke tombol-tombolnya.
 *
 * Aturan itu ditegakkan di sini, pada DOM, bukan lewat prop di
 * setiap komponen: chip reaksi, lampiran, pemutar suara, dan catatan sistem
 * masing-masing punya tombolnya sendiri, dan satu yang lupa diberi prop sudah
 * cukup untuk membuat riwayat panjang jadi ratusan pemberhentian Tab.
 */
export function useRovingLog({
  logRef,
  activeRow,
  onEmptyAction,
}: {
  logRef: RefObject<HTMLDivElement | null>;
  activeRow: string | null;
  /** Dipanggil saat Enter ditekan pada baris yang tidak punya tindakan. */
  onEmptyAction: () => void;
}) {
  const assign = () => {
    const log = logRef.current;
    if (!log) return;
    const focused = document.activeElement;
    for (const row of log.querySelectorAll<HTMLElement>('[data-msg]')) {
      const active = row.dataset.msg === activeRow;
      row.tabIndex = active ? 0 : -1;
      const inside = active && focused instanceof Node && row.contains(focused);
      for (const el of row.querySelectorAll<HTMLElement>(FOCUSABLE)) {
        el.tabIndex = inside ? 0 : -1;
      }
    }
  };

  // Tanpa daftar dependensi: isi baris berubah tanpa kabar ke sini (reaksi
  // baru, lampiran yang selesai dimuat), dan satu kueri per render murah.
  useLayoutEffect(assign);

  // Fokus masuk atau keluar dari sebuah baris mengubah siapa yang boleh
  // di-Tab, tanpa render ulang apa pun.
  useEffect(() => {
    const log = logRef.current;
    if (!log) return;
    // Fokus yang keluar baru tahu tujuannya setelah event selesai.
    const later = () => requestAnimationFrame(assign);
    log.addEventListener('focusin', assign);
    log.addEventListener('focusout', later);
    return () => {
      log.removeEventListener('focusin', assign);
      log.removeEventListener('focusout', later);
    };
  });

  function onKeyDown(e: KeyboardEvent<HTMLDivElement>) {
    const target = e.target as HTMLElement;
    if (!target.dataset.msg) return;
    const rows = Array.from(e.currentTarget.querySelectorAll<HTMLElement>('[data-msg]'));
    const at = rows.indexOf(target);
    const go = (i: number) => {
      const row = rows[Math.max(0, Math.min(rows.length - 1, i))];
      row?.focus();
      row?.scrollIntoView({ block: 'nearest' });
    };
    const openMenu = () => {
      const trigger = target.querySelector<HTMLButtonElement>('[data-menu-trigger]');
      if (trigger) trigger.click();
      else onEmptyAction();
    };

    switch (e.key) {
      case 'ArrowUp':
        e.preventDefault();
        go(at - 1);
        break;
      case 'ArrowDown':
        e.preventDefault();
        go(at + 1);
        break;
      case 'Home':
        e.preventDefault();
        go(0);
        break;
      case 'End':
        e.preventDefault();
        go(rows.length - 1);
        break;
      case 'Enter':
      case 'ContextMenu':
        e.preventDefault();
        openMenu();
        break;
      case 'F10':
        if (e.shiftKey) {
          e.preventDefault();
          openMenu();
        }
        break;
    }
  }

  return { onKeyDown };
}
