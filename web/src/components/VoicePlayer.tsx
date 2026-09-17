import { useEffect, useRef, useState } from 'react';
import { attachmentURL } from '../api';
import Icon from './Icon';
import type { Attachment } from '../types';

/** "0:07", "12:40" — panjang rekaman seperti yang biasa dibaca orang. */
export function formatDuration(ms: number): string {
  const total = Math.max(0, Math.round(ms / 1000));
  const m = Math.floor(total / 60);
  const s = total % 60;
  return `${m}:${String(s).padStart(2, '0')}`;
}

const SPEEDS = [1, 1.5, 2];

/**
 * Hanya satu rekaman yang berbunyi pada satu waktu.
 *
 * Menekan putar pada pesan suara kedua sementara yang pertama masih bicara
 * menghasilkan dua suara yang saling menimpa — dan tidak ada orang yang
 * bermaksud begitu.
 */
let sedangBerbunyi: HTMLAudioElement | null = null;

/**
 * Pemutar rekaman suara di dalam gelembung.
 *
 * Bukan `<audio controls>`. Pemutar bawaan browser lebarnya tetap, warnanya
 * milik sistem operasi, dan — yang menentukan — dia menampilkan "0:00 / 0:00"
 * untuk rekaman MediaRecorder, yang tidak menyebut durasinya sendiri. Di sini
 * durasinya datang dari perekam (`durationMs`) sampai berkasnya bisa
 * menjawab sendiri.
 *
 * Penggeser posisinya `<input type="range">` biasa: bisa digeser dengan jari,
 * dengan panah papan ketik, dan dibacakan pembaca layar tanpa satu baris ARIA
 * tambahan.
 */
export default function VoicePlayer({ attachment, mine }: { attachment: Attachment; mine: boolean }) {
  const audio = useRef<HTMLAudioElement>(null);
  const [playing, setPlaying] = useState(false);
  const [position, setPosition] = useState(0);
  const [fileDuration, setFileDuration] = useState<number | null>(null);
  const [speed, setSpeed] = useState(1);
  const [failed, setFailed] = useState(false);

  // Durasi dari berkas lebih dipercaya — kalau dia punya. WebM dari
  // MediaRecorder melaporkan Infinity, dan itu bukan angka.
  const duration =
    fileDuration && Number.isFinite(fileDuration)
      ? fileDuration * 1000
      : (attachment.durationMs ?? 0);
  const voice = attachment.durationMs !== undefined;

  useEffect(() => {
    const el = audio.current;
    return () => {
      if (el && sedangBerbunyi === el) sedangBerbunyi = null;
    };
  }, []);

  const toggle = async () => {
    const el = audio.current;
    if (!el) return;
    if (!el.paused) {
      el.pause();
      return;
    }
    if (sedangBerbunyi && sedangBerbunyi !== el) sedangBerbunyi.pause();
    sedangBerbunyi = el;
    el.playbackRate = speed;
    try {
      await el.play();
    } catch {
      setFailed(true);
    }
  };

  const seek = (ms: number) => {
    const el = audio.current;
    if (!el) return;
    el.currentTime = ms / 1000;
    setPosition(ms);
  };

  const nextSpeed = () => {
    const next = SPEEDS[(SPEEDS.indexOf(speed) + 1) % SPEEDS.length]!;
    setSpeed(next);
    if (audio.current) audio.current.playbackRate = next;
  };

  const ink = mine ? 'text-accent-ink' : 'text-accent-text';
  const soft = mine ? 'text-accent-ink/80' : 'text-muted';

  return (
    <div className="flex w-[min(17rem,62vw)] items-center gap-2 py-0.5 pr-1 pl-0.5">
      <audio
        ref={audio}
        src={attachmentURL(attachment.url)}
        // Hanya header berkasnya, lewat permintaan sepotong. Sisanya baru
        // diambil saat tombol putar ditekan.
        preload="metadata"
        onLoadedMetadata={e => setFileDuration(e.currentTarget.duration)}
        onDurationChange={e => setFileDuration(e.currentTarget.duration)}
        onTimeUpdate={e => setPosition(e.currentTarget.currentTime * 1000)}
        onPlay={() => setPlaying(true)}
        onPause={() => setPlaying(false)}
        onEnded={e => {
          setPlaying(false);
          // Kembali ke awal: rekaman yang habis diputar dan ditekan lagi
          // seharusnya mulai dari depan, bukan diam di ujung.
          e.currentTarget.currentTime = 0;
          setPosition(0);
        }}
        onError={() => setFailed(true)}
      />

      <button
        type="button"
        onClick={() => void toggle()}
        disabled={failed}
        aria-label={playing ? 'Jeda' : 'Putar pesan suara'}
        className={`grid size-10 shrink-0 place-items-center rounded-full transition disabled:opacity-40 ${
          mine
            ? 'bg-accent-ink text-accent hover:brightness-95'
            : 'bg-accent text-accent-ink hover:brightness-110'
        }`}
      >
        {playing ? (
          <Icon name="jeda" size={18} className="stroke-[2.5]" />
        ) : (
          <Icon name="putar" size={18} className="translate-x-px fill-current" />
        )}
      </button>

      <div className="min-w-0 flex-1">
        {!voice && (
          <p className={`truncate text-[13px] font-semibold ${ink}`}>{attachment.name}</p>
        )}
        <input
          type="range"
          min={0}
          max={Math.max(1, Math.round(duration))}
          step={100}
          value={Math.min(position, duration)}
          onChange={e => seek(Number(e.target.value))}
          disabled={failed || duration === 0}
          aria-label="Posisi pemutaran"
          aria-valuetext={`${formatDuration(position)} dari ${formatDuration(duration)}`}
          className={`block h-5 w-full cursor-pointer ${mine ? 'accent-accent-ink' : 'accent-accent'}`}
        />
        <div className={`flex items-center justify-between text-[11.5px] tabular-nums ${soft}`}>
          <span className="flex items-center gap-1">
            {voice && <Icon name="mikrofon" size={12} />}
            {failed
              ? 'Tidak bisa diputar'
              : // Selama diputar: posisi. Selebihnya: panjang seluruhnya —
                // yang ingin diketahui orang sebelum memutuskan mendengarkan.
                formatDuration(playing || position > 0 ? position : duration)}
          </span>
        </div>
      </div>

      <button
        type="button"
        onClick={nextSpeed}
        aria-label={`Kecepatan ${speed}×, tekan untuk mengganti`}
        className={`h-6 shrink-0 self-center rounded-full px-1.5 text-[11.5px] font-bold tabular-nums transition ${
          mine ? 'bg-accent-ink/15 hover:bg-accent-ink/25' : 'bg-canvas text-muted hover:text-ink'
        }`}
      >
        {speed}×
      </button>
    </div>
  );
}
