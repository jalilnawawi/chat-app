import type { Member } from '../types';

export type MentionCandidate = { id: string; name: string; all: boolean };

/**
 * Calon sebutan untuk kata yang sedang diketik.
 *
 * "semua" hanya di grup: membangunkan seluruh anggota DM adalah membangunkan
 * satu orang, dan itu sudah dilakukan pesan biasa. Dia selalu PALING AKHIR dan
 * tidak pernah terpilih dengan sendirinya — di urutan pertama, "@" lalu Enter
 * yang tergesa membangunkan seluruh grup, dan tindakan paling berisik di
 * aplikasi ini tidak boleh jadi yang paling mudah dilakukan tanpa sengaja.
 */
export function mentionCandidates(
  query: string,
  roster: Member[],
  meId: string,
  isGroup: boolean,
): MentionCandidate[] {
  const q = query.toLowerCase();
  const people = roster
    .filter(m => m.userId !== meId && m.displayName.toLowerCase().includes(q))
    .map(m => ({ id: m.userId, name: m.displayName, all: false }));
  const all = isGroup && 'semua'.startsWith(q) ? [{ id: 'semua', name: 'semua', all: true }] : [];
  return [...people.slice(0, all.length > 0 ? 5 : 6), ...all];
}

/** Pilihan awal: orang pertama, atau tidak ada bila yang cocok hanya "semua". */
export const initialMentionIndex = (list: MentionCandidate[]) => list.findIndex(c => !c.all);

export const MENTION_LIST_ID = 'daftar-sebutan';
export const mentionOptionId = (i: number) => `sebutan-${i}`;

/**
 * Daftar sebutan di atas kolom tulis.
 *
 * Fokus tetap di kolom tulis; daftar ini dikendalikan dari sana lewat
 * `aria-activedescendant`, jadi pilihannya tidak pernah masuk urutan Tab.
 */
export default function MentionList({
  candidates,
  active,
  onHover,
  onChoose,
}: {
  candidates: MentionCandidate[];
  active: number;
  onHover: (i: number) => void;
  onChoose: (c: MentionCandidate) => void;
}) {
  return (
    <div
      id={MENTION_LIST_ID}
      role="listbox"
      aria-label="Sebut seseorang"
      className="absolute bottom-full left-3 z-20 mb-2 w-[min(20rem,calc(100%-1.5rem))] overflow-hidden rounded-2xl border border-line bg-surface p-1 shadow-pop"
    >
      {candidates.map((c, i) => (
        <button
          key={c.id}
          id={mentionOptionId(i)}
          type="button"
          role="option"
          aria-selected={i === active}
          tabIndex={-1}
          // onMouseDown, bukan onClick: klik biasa lebih dulu memicu blur pada
          // kolom tulis, dan posisi kursor yang dipakai untuk menyisipkan nama
          // sudah hilang saat handler-nya jalan.
          onMouseDown={e => {
            e.preventDefault();
            onChoose(c);
          }}
          onMouseEnter={() => onHover(i)}
          className={`flex min-h-11 w-full items-center gap-2 rounded-xl px-3 text-left text-[15px] transition ${
            i === active ? 'bg-accent-soft' : 'hover:bg-canvas'
          } ${c.all && i > 0 ? 'mt-1 border-t border-line' : ''}`}
        >
          {c.all ? (
            <span className="flex min-w-0 items-baseline gap-2 whitespace-nowrap">
              <span className="font-bold">@semua</span>
              <span className="truncate text-[13px] text-muted">bangunkan semua anggota</span>
            </span>
          ) : (
            <span className="truncate font-semibold">{c.name}</span>
          )}
        </button>
      ))}
    </div>
  );
}
