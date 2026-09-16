import { useEffect, useRef, useState } from 'react';
import { useStore } from '../store';
import Icon from './Icon';
import type { Message } from '../types';

/**
 * Pilihan cepat.
 *
 * Sengaja pendek dan tetap. Pemilih emoji lengkap adalah komponen besar yang
 * memuat ribuan gambar untuk pekerjaan yang sembilan dari sepuluh kali selesai
 * dengan salah satu dari delapan ini — dan server tetap menerima emoji apa pun
 * yang lolos pemeriksaan satu grafem, jadi pilihan ini tidak mengunci apa-apa.
 */
const QUICK = ['👍', '❤️', '😂', '🎉', '🙏', '😮', '😢', '🔥'];

/**
 * Deretan reaksi di bawah gelembung.
 *
 * Yang ditampilkan adalah JUMLAH, bukan daftar orangnya — itu memang yang
 * dikirim server. Sebuah grup dua ratus orang yang semuanya menekan emoji yang
 * sama akan menghasilkan dua ratus id yang tak satu pun tampil di layar, dan
 * riwayat yang memuatnya di setiap pesan membayar itu untuk seluruh halaman.
 */
export default function ReactionRow({ message, mine }: { message: Message; mine: boolean }) {
  const toggleReaction = useStore(s => s.toggleReaction);
  const [picking, setPicking] = useState(false);
  const boxRef = useRef<HTMLDivElement>(null);

  // Klik di luar menutup pemilih. Tanpa ini, pemilih yang terbuka menempel di
  // layar sampai salah satu emojinya ditekan — termasuk saat orangnya sudah
  // berpindah ke pesan lain.
  useEffect(() => {
    if (!picking) return;
    const onDown = (e: MouseEvent) => {
      if (!boxRef.current?.contains(e.target as Node)) setPicking(false);
    };
    document.addEventListener('mousedown', onDown);
    return () => document.removeEventListener('mousedown', onDown);
  }, [picking]);

  const pick = (emoji: string) => {
    setPicking(false);
    void toggleReaction(message.conversationId, message.id, emoji);
  };

  return (
    <div
      ref={boxRef}
      className={`relative mt-1.5 flex flex-wrap items-center gap-1 ${
        mine ? 'justify-end' : 'justify-start'
      }`}
    >
      {message.reactions.map(r => (
        <button
          key={r.emoji}
          onClick={() => pick(r.emoji)}
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

      {/* Tombol tambah hanya muncul saat kursor ada di gelembungnya, kecuali
          kalau pemilihnya sedang terbuka — kalau tidak, dia hilang persis saat
          kursor bergerak ke arah pilihannya.

          Dan di perangkat yang tidak punya kursor sama sekali, dia selalu
          tampil: `group-hover` di Tailwind v4 hidup di dalam
          @media (hover: hover), jadi tanpa syarat kedua ini seluruh fitur
          reaksi tidak bisa dijangkau dari ponsel. */}
      <button
        onClick={() => setPicking(v => !v)}
        aria-label="Beri reaksi"
        title="Beri reaksi"
        className={`grid size-6 place-items-center rounded-full border border-line bg-surface text-muted transition hover:border-accent hover:text-accent-text ${
          picking ? '' : 'hidden group-hover:grid [@media(hover:none)]:grid'
        }`}
      >
        <Icon name="reaksi" size={14} />
      </button>

      {picking && (
        <div
          className={`absolute bottom-full z-20 mb-1.5 flex gap-0.5 rounded-2xl border border-line bg-surface p-1.5 shadow-pop ${
            mine ? 'right-0' : 'left-0'
          }`}
        >
          {QUICK.map(emoji => (
            <button
              key={emoji}
              onClick={() => pick(emoji)}
              // Sasaran sentuh yang cukup besar: barisan delapan emoji yang
              // masing-masing selebar emojinya saja adalah delapan tombol yang
              // saling bersebelahan terlalu rapat untuk ibu jari.
              className="grid size-9 place-items-center rounded-xl text-lg transition hover:bg-accent-soft"
            >
              {emoji}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
