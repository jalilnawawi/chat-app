import Icon from './Icon';
import { QUICK_REACTIONS } from '../emoji';

/**
 * Delapan reaksi cepat dan satu tombol "lainnya" — satu baris yang sama di
 * papan reaksi samping gelembung dan di menu/lembar tindakan pesan.
 *
 * `variant`:
 * - `popover` — papan kecil di samping gelembung dan di menu desktop (40px).
 * - `sheet`   — lembar tindakan di layar sentuh (48px, emoji besar).
 *
 * Tombol biasa, bukan butir menu: di dalam menu, panah atas/bawah melompati
 * baris ini dan Tab yang masuk ke sana.
 */
export default function QuickReactions({
  given,
  onReact,
  onMore,
  variant = 'popover',
}: {
  given: string[];
  onReact: (emoji: string) => void;
  onMore: () => void;
  variant?: 'popover' | 'sheet';
}) {
  const cell =
    variant === 'sheet'
      ? 'h-12 min-w-0 flex-1 text-[24px]'
      : // Di layar sempit sembilan sasaran berbagi lebar (±40 px × 44 px);
        // di layar lebar 40 × 40.
        'h-11 min-w-0 flex-1 text-lg sm:size-10 sm:flex-none';
  return (
    <div role="group" aria-label="Beri reaksi" className="flex items-center gap-0.5">
      {QUICK_REACTIONS.map(emoji => {
        const on = given.includes(emoji);
        return (
          <button
            key={emoji}
            type="button"
            aria-label={`Reaksi ${emoji}`}
            aria-pressed={on}
            onClick={() => onReact(emoji)}
            className={`grid place-items-center rounded-xl transition ${cell} ${
              on ? 'bg-accent-soft' : 'hover:bg-canvas'
            }`}
          >
            {emoji}
          </button>
        );
      })}
      <button
        type="button"
        aria-label="Emoji lainnya"
        title="Emoji lainnya"
        onClick={onMore}
        className={`grid place-items-center rounded-xl text-muted transition hover:bg-canvas hover:text-ink ${cell}`}
      >
        <Icon name="tambah" size={variant === 'sheet' ? 22 : 18} />
      </button>
    </div>
  );
}
