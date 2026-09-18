import { useEffect, useRef, useState } from 'react';
import { useStore } from '../store';
import EmojiPicker from './EmojiPicker';
import QuickReactions from './QuickReactions';
import Icon from './Icon';
import type { Message } from '../types';


/**
 * Deretan reaksi di bawah gelembung.
 *
 * Yang ditampilkan adalah JUMLAH, bukan daftar orangnya — itu memang yang
 * dikirim server. Sebuah grup dua ratus orang yang semuanya menekan emoji yang
 * sama akan menghasilkan dua ratus id yang tak satu pun tampil di layar, dan
 * riwayat yang memuatnya di setiap pesan membayar itu untuk seluruh halaman.
 *
 * Tombol untuk MEMBERI reaksi tidak tinggal di sini lagi — lihat ReactionButton.
 * Deretan ini kini hanya ada kalau memang ada reaksi, sehingga pesan tanpa
 * reaksi tidak lagi membawa baris kosong yang cuma menunggu kursor lewat.
 */
export default function ReactionRow({ message, mine }: { message: Message; mine: boolean }) {
  const toggleReaction = useStore(s => s.toggleReaction);
  if (message.reactions.length === 0) return null;

  return (
    <div className={`mt-1 flex flex-wrap items-center gap-1 ${mine ? 'justify-end' : 'justify-start'}`}>
      {message.reactions.map(r => (
        <button
          key={r.emoji}
          onClick={() => void toggleReaction(message.conversationId, message.id, r.emoji)}
          title={
            r.mine
              ? `Kamu${r.count > 1 ? ` dan ${r.count - 1} lainnya` : ''} — klik untuk melepas`
              : `${r.count} orang`
          }
          aria-pressed={r.mine}
          aria-label={`${r.emoji} ${r.count} orang${r.mine ? ', termasuk kamu' : ''}`}
          // Setinggi 40 px di semua layar — lantai sasaran di DESIGN.md. Chip
          // ini tombol, bukan hiasan di bawah gelembung.
          className={`flex min-h-10 items-center gap-1 rounded-full border px-3 text-[14px] transition ${
            r.mine
              ? 'border-accent bg-accent-soft font-bold text-accent-text'
              : 'border-line bg-surface text-muted hover:border-accent hover:text-ink'
          }`}
        >
          <span>{r.emoji}</span>
          <span className="tabular-nums">{r.count}</span>
        </button>
      ))}
    </div>
  );
}

/**
 * Tombol reaksi di SAMPING gelembung — di kiri untuk pesan sendiri, di kanan
 * untuk pesan orang.
 *
 * Sebelumnya dia duduk di baris reaksi di bawah gelembung, dan baris itu
 * harus selalu ada demi tombolnya: setiap pesan membawa ruang kosong yang
 * hanya terisi saat kursor lewat, dan isi percakapan melompat setinggi baris
 * itu setiap kali tombolnya muncul. Di samping gelembung, dia mengisi ruang
 * yang memang sudah kosong, dan dekat dengan benda yang hendak diberi reaksi.
 *
 * Papannya ditambatkan ke leluhur terdekat yang `relative` — kolom gelembung —
 * bukan ke tombol ini. Tombol pesan sendiri bisa berdiri jauh di kiri layar
 * sempit, dan papan selebar delapan emoji yang ditambatkan ke sana akan keluar
 * dari layar; kolom gelembung selalu menempel ke tepinya sendiri.
 */
export function ReactionButton({
  message,
  mine,
  label = 'Beri reaksi',
}: {
  message: Message;
  mine: boolean;
  label?: string;
}) {
  const toggleReaction = useStore(s => s.toggleReaction);
  const [open, setOpen] = useState<null | 'cepat' | 'semua'>(null);
  const [byKeyboard, setByKeyboard] = useState(false);
  const boxRef = useRef<HTMLDivElement>(null);
  const trigger = useRef<HTMLButtonElement>(null);

  // Klik di luar dan Escape menutup papan. Tanpa ini, papan yang terbuka
  // menempel di layar sampai salah satu emojinya ditekan — termasuk saat
  // orangnya sudah berpindah ke pesan lain.
  useEffect(() => {
    if (!open) return;
    const onDown = (e: PointerEvent) => {
      if (!boxRef.current?.contains(e.target as Node)) setOpen(null);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== 'Escape') return;
      setOpen(null);
      if (boxRef.current?.contains(document.activeElement)) trigger.current?.focus();
    };
    document.addEventListener('pointerdown', onDown);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('pointerdown', onDown);
      document.removeEventListener('keydown', onKey);
    };
  }, [open]);

  const pick = (emoji: string) => {
    setOpen(null);
    // Dari papan lengkap: emoji yang SUDAH kita berikan tidak dilepas. Orang
    // yang mencarinya di antara lima ratus pilihan sedang ingin memberi, dan
    // melepas reaksi punya jalannya sendiri — chip di bawah gelembung.
    if (message.reactions.some(r => r.emoji === emoji && r.mine)) return;
    void toggleReaction(message.conversationId, message.id, emoji);
  };

  return (
    <div ref={boxRef} className="contents">
      {/* Muncul saat kursor ada di gelembungnya, kecuali kalau papannya sedang
          terbuka — kalau tidak, dia hilang persis saat kursor bergerak ke
          arah pilihannya. Baris yang difokus lewat papan ketik memunculkannya
          juga, lewat `.aksi-pesan` di index.css.

          Di layar sentuh dia tidak ada sama sekali: `group-hover` di Tailwind
          v4 hidup di dalam @media (hover: hover), jadi di sana dia tak akan
          pernah muncul — dan reaksi cepat sudah punya rumahnya sendiri di
          baris teratas lembar tindakan. */}
      <button
        ref={trigger}
        type="button"
        onClick={e => {
          setByKeyboard(e.detail === 0);
          setOpen(v => (v ? null : 'cepat'));
        }}
        aria-label={label}
        aria-expanded={open !== null}
        title="Beri reaksi"
        className={`aksi-pesan grid size-9 shrink-0 place-items-center self-center rounded-full text-muted transition hover:bg-surface hover:text-accent-text focus-visible:opacity-100 [@media(pointer:coarse)]:size-11 ${
          open
            ? 'bg-surface text-accent-text opacity-100'
            : // Di layar sentuh tombol ini tidak ada: reaksi cepat tinggal di
              // baris teratas lembar tindakan, di bawah ibu jari, bersama
              // tindakan lain untuk pesan yang sama.
              'opacity-0 group-hover:opacity-100 [@media(hover:none)]:hidden'
        }`}
      >
        <Icon name="reaksi" size={18} />
      </button>

      {open && (
        <div
          className={
            open === 'semua'
              ? // Papan lengkap selebar layar sempit tidak muat di mana pun
                // kalau ditambatkan ke gelembung — di sana dia berdiri di
                // tengah atas layar, jauh dari kolom tulis dan papan ketik
                // yang bisa saja sedang terbuka di bawahnya.
                `fixed inset-x-3 top-20 z-30 flex justify-center sm:absolute sm:inset-x-auto sm:top-auto sm:bottom-full sm:mb-1.5 ${
                  mine ? 'sm:right-0' : 'sm:left-0'
                }`
              : // Papan cepat di layar sempit juga berdiri di atas, selebar
                // layar: ditambatkan ke gelembung, sembilan sasaran 40 px
                // membungkus jadi dua baris dan menutupi gelembungnya sendiri.
                `fixed inset-x-3 top-20 z-30 sm:absolute sm:inset-x-auto sm:top-auto sm:bottom-full sm:mb-1.5 ${
                  mine ? 'sm:right-0' : 'sm:left-0'
                }`
          }
        >
          {open === 'semua' ? (
            <EmojiPicker onPick={pick} autoFocus={byKeyboard} />
          ) : (
            <div className="rounded-2xl border border-line bg-surface p-1 shadow-pop sm:p-1.5">
              <QuickReactions
                given={message.reactions.filter(r => r.mine).map(r => r.emoji)}
                onReact={emoji => {
                  setOpen(null);
                  void toggleReaction(message.conversationId, message.id, emoji);
                }}
                onMore={() => setOpen('semua')}
              />
            </div>
          )}
        </div>
      )}
    </div>
  );
}
