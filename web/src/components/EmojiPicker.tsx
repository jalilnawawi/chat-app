import { useMemo, useRef, useState } from 'react';
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
export default function EmojiPicker({ onPick }: { onPick: (emoji: string) => void }) {
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
  const scroller = useRef<HTMLDivElement>(null);

  const jump = (id: string) => {
    setActive(id);
    const el = scroller.current?.querySelector<HTMLElement>(`[data-group="${id}"]`);
    if (el && scroller.current) scroller.current.scrollTop = el.offsetTop;
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
      className="w-[min(20rem,calc(100vw-1.5rem))] overflow-hidden rounded-2xl border border-line bg-surface shadow-pop"
      onMouseDown={e => e.preventDefault()}
    >
      <div role="tablist" className="flex gap-0.5 border-b border-line px-1.5 py-1">
        {groups.map(g => (
          <button
            key={g.id}
            type="button"
            role="tab"
            aria-selected={active === g.id}
            aria-label={g.label}
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

      <div ref={scroller} onScroll={onScroll} className="relative h-64 overflow-y-auto px-1.5 pb-2">
        {groups.map(g => (
          <section key={g.id} data-group={g.id}>
            <h3 className="sticky top-0 bg-surface/95 px-1 pt-2 pb-1 text-[12px] font-bold text-muted">
              {g.label}
            </h3>
            <div className="grid grid-cols-8">
              {g.items.map(emoji => (
                <button
                  key={emoji}
                  type="button"
                  onClick={() => pick(emoji)}
                  aria-label={emoji}
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
