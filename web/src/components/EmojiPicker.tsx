import { useEffect, useId, useMemo, useRef, useState, type KeyboardEvent } from 'react';
import { EMOJI_GROUPS, recentEmoji, rememberEmoji } from '../emoji';

/**
 * Papan emoji: satu baris kategori di atas, kisi yang bisa digulir di bawah.
 *
 * Dipakai di dua tempat — kolom tulis dan tombol reaksi — dan keduanya hanya
 * butuh satu hal darinya: emoji yang dipilih. Menutup papannya adalah urusan
 * pemakai, karena keduanya berbeda: kolom tulis membiarkannya terbuka supaya
 * orang bisa memilih beberapa sekaligus, reaksi menutupnya setelah satu.
 *
 * Tombol-tombolnya menahan `mousedown`. Tanpa itu, menekan emoji lebih dulu
 * mencabut fokus dari kolom tulis, dan posisi kursor tempat emoji itu
 * seharusnya disisipkan ikut hilang.
 */
export default function EmojiPicker({
  onPick,
  autoFocus = false,
}: {
  onPick: (emoji: string) => void;
  /** Dibuka lewat papan ketik: fokus langsung masuk ke emoji pertama. */
  autoFocus?: boolean;
}) {
  // Dibaca sekali saat papan dibuka. Memperbaruinya di setiap pilihan akan
  // menggeser isi baris "Terakhir" persis di bawah jari yang sedang menekan.
  const recent = useMemo(() => recentEmoji(), []);
  const groups = useMemo(
    () =>
      recent.length > 0
        ? [{ id: 'terakhir', label: 'Terakhir dipakai', icon: '🕘', items: recent }, ...EMOJI_GROUPS]
        : EMOJI_GROUPS,
    [recent],
  );
  const [active, setActive] = useState(groups[0]!.id);
  // Indeks datar emoji pertama tiap bagian, untuk roving tabindex.
  const offsets = useMemo(() => {
    const out: number[] = [];
    let n = 0;
    for (const g of groups) {
      out.push(n);
      n += g.items.length;
    }
    return out;
  }, [groups]);
  const scroller = useRef<HTMLDivElement>(null);
  const uid = useId();

  // Kisi emoji adalah SATU pemberhentian Tab (roving tabindex). Lima ratus
  // tombol yang masing-masing masuk urutan Tab membuat orang yang hanya
  // memakai papan ketik terjebak di sini sampai menyerah.
  const [cursor, setCursor] = useState(0);

  useEffect(() => {
    if (!autoFocus) return;
    scroller.current?.querySelector<HTMLButtonElement>('[data-emoji]')?.focus();
  }, [autoFocus]);

  const cells = () =>
    Array.from(scroller.current?.querySelectorAll<HTMLButtonElement>('[data-emoji]') ?? []);

  /** Panah kiri/kanan berpindah satu; atas/bawah ke baris tetangga, ke kolom terdekat. */
  function onGridKey(e: KeyboardEvent<HTMLDivElement>) {
    const all = cells();
    const at = all.indexOf(document.activeElement as HTMLButtonElement);
    if (at < 0) return;
    let next = -1;
    const here = all[at]!.getBoundingClientRect();
    const rowStep = (dir: 1 | -1) => {
      const candidates = all
        .map((b, i) => ({ i, r: b.getBoundingClientRect() }))
        .filter(({ r }) => (dir === 1 ? r.top > here.top + 2 : r.top < here.top - 2));
      if (candidates.length === 0) return -1;
      const rowTop =
        dir === 1
          ? Math.min(...candidates.map(c => c.r.top))
          : Math.max(...candidates.map(c => c.r.top));
      const row = candidates.filter(c => Math.abs(c.r.top - rowTop) < 2);
      return row.reduce((best, c) =>
        Math.abs(c.r.left - here.left) < Math.abs(best.r.left - here.left) ? c : best,
      ).i;
    };
    switch (e.key) {
      case 'ArrowRight':
        next = Math.min(all.length - 1, at + 1);
        break;
      case 'ArrowLeft':
        next = Math.max(0, at - 1);
        break;
      case 'ArrowDown':
        next = rowStep(1);
        break;
      case 'ArrowUp':
        next = rowStep(-1);
        break;
      case 'Home':
        next = 0;
        break;
      case 'End':
        next = all.length - 1;
        break;
      default:
        return;
    }
    e.preventDefault();
    if (next < 0) return;
    setCursor(next);
    all[next]!.focus();
    all[next]!.scrollIntoView({ block: 'nearest' });
  }

  const jump = (id: string) => {
    setActive(id);
    const el = scroller.current?.querySelector<HTMLElement>(`[data-group="${id}"]`);
    if (el && scroller.current) scroller.current.scrollTop = el.offsetTop;
    // Lompatan lewat papan ketik ikut memindahkan kursor kisi ke bagian itu.
    const first = el?.querySelector<HTMLButtonElement>('[data-emoji]');
    if (first) setCursor(cells().indexOf(first));
  };

  // Tab yang menyala mengikuti guliran, bukan hanya klik: orang yang menggulir
  // dari wajah ke makanan perlu tahu di mana dia sekarang.
  const onScroll = () => {
    const box = scroller.current;
    if (!box) return;
    let current = groups[0]!.id;
    for (const section of box.querySelectorAll<HTMLElement>('[data-group]')) {
      if (section.offsetTop - box.scrollTop <= 8) current = section.dataset.group!;
    }
    if (current !== active) setActive(current);
  };

  const pick = (emoji: string) => {
    rememberEmoji(emoji);
    onPick(emoji);
  };

  return (
    <div
      role="dialog"
      aria-label="Pilih emoji"
      // Di layar sentuh papannya lebih lebar dan kisinya tujuh kolom: sel
      // ±48 px, bukan ±38 px yang meleset di bawah ibu jari.
      className="w-[min(20rem,calc(100vw-1.5rem))] overflow-hidden rounded-2xl border border-line bg-surface shadow-pop [@media(pointer:coarse)]:w-[min(22rem,calc(100vw-1.5rem))]"
      onMouseDown={e => e.preventDefault()}
    >
      {/* Lompatan ke bagian, bukan tab: semua bagian tetap ada di satu
          daftar yang digulir, dan tombol ini hanya menggulirnya. */}
      <div role="group" aria-label="Kategori emoji" className="flex gap-0.5 border-b border-line px-1.5 py-1">
        {groups.map(g => (
          <button
            key={g.id}
            type="button"
            aria-current={active === g.id ? 'true' : undefined}
            aria-controls={`${uid}-${g.id}`}
            aria-label={`Lompat ke ${g.label}`}
            title={g.label}
            onClick={() => jump(g.id)}
            className={`grid h-9 flex-1 place-items-center rounded-lg text-lg transition ${
              active === g.id ? 'bg-accent-soft' : 'opacity-60 hover:bg-canvas hover:opacity-100'
            }`}
          >
            {g.icon}
          </button>
        ))}
      </div>

      <div
        ref={scroller}
        onScroll={onScroll}
        onKeyDown={onGridKey}
        className="relative h-64 overflow-y-auto px-1.5 pb-2"
      >
        {groups.map((g, gi) => (
          <section key={g.id} data-group={g.id} id={`${uid}-${g.id}`} aria-labelledby={`${uid}-${g.id}-judul`}>
            <h3
              id={`${uid}-${g.id}-judul`}
              className="sticky top-0 bg-surface/95 px-1 pt-2 pb-1 text-[12px] font-bold text-muted"
            >
              {g.label}
            </h3>
            <div className="grid grid-cols-8 [@media(pointer:coarse)]:grid-cols-7">
              {g.items.map((emoji, ii) => (
                <button
                  key={emoji}
                  type="button"
                  data-emoji=""
                  tabIndex={offsets[gi]! + ii === cursor ? 0 : -1}
                  onFocus={() => setCursor(offsets[gi]! + ii)}
                  onClick={() => pick(emoji)}
                  className="grid aspect-square place-items-center rounded-lg text-[22px] leading-none transition hover:bg-accent-soft"
                >
                  {emoji}
                </button>
              ))}
            </div>
          </section>
        ))}
      </div>
    </div>
  );
}
