import { useEffect, useRef, useState } from 'react';
import { useStore } from '../store';
import EmojiPicker from './EmojiPicker';
import Icon from './Icon';
import type { Message } from '../types';

/**
 * Pilihan cepat.
 *
 * Sengaja pendek dan tetap: sembilan dari sepuluh reaksi selesai dengan salah
 * satu dari delapan ini. Sisanya ada di balik tombol tambah, di papan emoji
 * yang sama dengan kolom tulis — dan server tetap menerima emoji apa pun yang
 * lolos pemeriksaan satu grafem, jadi pilihan ini tidak mengunci apa-apa.
 */
const QUICK = ['👍', '❤️', '😂', '🎉', '🙏', '😮', '😢', '🔥'];

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
          className={`flex items-center gap-1 rounded-full border px-2 py-0.5 text-[13px] transition ${
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
export function ReactionButton({ message, mine }: { message: Message; mine: boolean }) {
  const toggleReaction = useStore(s => s.toggleReaction);
  const [open, setOpen] = useState<null | 'cepat' | 'semua'>(null);
  const boxRef = useRef<HTMLDivElement>(null);

  // Klik di luar dan Escape menutup papan. Tanpa ini, papan yang terbuka
  // menempel di layar sampai salah satu emojinya ditekan — termasuk saat
  // orangnya sudah berpindah ke pesan lain.
  useEffect(() => {
    if (!open) return;
    const onDown = (e: PointerEvent) => {
      if (!boxRef.current?.contains(e.target as Node)) setOpen(null);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(null);
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
      {/* Muncul saat kursor ada di pesannya, kecuali kalau papannya sedang
          terbuka — kalau tidak, dia hilang persis saat kursor bergerak ke
          arah pilihannya. Di perangkat tanpa kursor dia selalu tampil:
          `group-hover` di Tailwind v4 hidup di dalam @media (hover: hover),
          dan tanpa syarat kedua seluruh fitur reaksi tak terjangkau dari
          ponsel. */}
      <button
        type="button"
        onClick={() => setOpen(v => (v ? null : 'cepat'))}
        aria-label="Beri reaksi"
        aria-expanded={open !== null}
        title="Beri reaksi"
        className={`grid size-8 shrink-0 place-items-center self-center rounded-full text-muted transition hover:bg-surface hover:text-accent-text focus-visible:opacity-100 ${
          open
            ? 'bg-surface text-accent-text opacity-100'
            : 'opacity-0 group-hover:opacity-100 [@media(hover:none)]:opacity-100'
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
                // tengah layar, di atas kolom tulis.
                `fixed inset-x-3 bottom-24 z-30 flex justify-center sm:absolute sm:inset-x-auto sm:bottom-full sm:mb-1.5 ${
                  mine ? 'sm:right-0' : 'sm:left-0'
                }`
              : `absolute bottom-full z-30 mb-1.5 ${mine ? 'right-0' : 'left-0'}`
          }
        >
          {open === 'semua' ? (
            <EmojiPicker onPick={pick} />
          ) : (
            <div className="flex items-center gap-0.5 rounded-2xl border border-line bg-surface p-1.5 shadow-pop">
              {QUICK.map(emoji => {
                const given = message.reactions.some(r => r.emoji === emoji && r.mine);
                return (
                  <button
                    key={emoji}
                    type="button"
                    onClick={() => {
                      setOpen(null);
                      void toggleReaction(message.conversationId, message.id, emoji);
                    }}
                    aria-pressed={given}
                    // Sasaran sentuh 32 px di layar sempit, 36 px di tempat lain:
                    // sembilan tombol selebar 36 px tidak muat di layar 360 px
                    // bila gelembungnya berdiri di samping foto pengirim.
                    className={`grid size-8 place-items-center rounded-xl text-lg transition sm:size-9 ${
                      given ? 'bg-accent-soft' : 'hover:bg-accent-soft'
                    }`}
                  >
                    {emoji}
                  </button>
                );
              })}
              <button
                type="button"
                onClick={() => setOpen('semua')}
                aria-label="Emoji lainnya"
                title="Emoji lainnya"
                className="grid size-8 place-items-center rounded-xl text-muted transition hover:bg-canvas hover:text-ink sm:size-9"
              >
                <Icon name="tambah" size={18} />
              </button>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
