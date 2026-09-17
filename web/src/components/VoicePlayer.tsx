import { useEffect, useRef, useState } from 'react';
import { attachmentURL } from '../api';
import Icon from './Icon';
import type { Attachment } from '../types';

import { formatDuration } from '../format';
import { T } from '../teks';

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
  // Sebab kegagalan, bukan sekadar "gagal": jaringan yang putus butuh
  // "coba lagi", berkas yang rusak tidak.
  const [failure, setFailure] = useState<string | null>(null);
  const failed = failure !== null;

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

  // Gagal memutar sering hanya sesaat — jaringan putus di tengah unduhan.
  // Mencoba lagi berarti meminta berkasnya dari awal.
  const retry = () => {
    setFailure(null);
    audio.current?.load();
  };

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
      setFailure('browser menolak memutarnya');
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
  // Putih penuh di gelembung sendiri — lihat catatan kontras di MessageParts.
  const soft = mine ? 'text-accent-ink' : 'text-muted';

  return (
    // Lebar tetap, tapi tidak pernah melebihi gelembungnya: gelembung pesan
    // suara memesan sisi kanannya untuk panah menu, dan pemutar yang dipatok
    // pada lebar layar meluber ke bawah panah itu di ponsel.
    <div className="flex w-68 max-w-full min-w-0 items-center gap-2 py-0.5 pr-1 pl-0.5">
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
        onError={e => setFailure(mediaErrorText(e.currentTarget.error))}
        // Elemen audio tanpa `controls` tidak untuk difokus; pemutarnya
        // adalah tombol dan penggeser di bawah.
        tabIndex={-1}
      />

      <button
        type="button"
        onClick={() => void toggle()}
        disabled={failed}
        aria-label={playing ? 'Jeda' : 'Putar pesan suara'}
        // Nonaktif = bergaris, bukan dipudarkan: lingkaran pudar terbaca
        // sebagai tombol yang macet, bukan rekaman yang tak bisa diputar.
        className={`grid size-10 shrink-0 place-items-center rounded-full border transition ${
          mine
            ? 'border-transparent bg-accent-ink text-accent hover:brightness-95 disabled:border-accent-ink disabled:bg-transparent disabled:text-accent-ink'
            : 'border-transparent bg-accent text-accent-ink hover:brightness-110 disabled:border-line-strong disabled:bg-surface disabled:text-muted'
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
          // Setinggi 32 px (40 px di layar sentuh) walau jalurnya tipis: yang
          // ditekan jari adalah kotaknya, bukan garisnya.
          className={`block h-8 w-full cursor-pointer [@media(pointer:coarse)]:h-10 ${mine ? 'accent-accent-ink' : 'accent-accent'}`}
        />
        <div className={`flex items-center justify-between text-[11.5px] tabular-nums ${soft}`}>
          <span className="flex items-center gap-1">
            {voice && <Icon name="mikrofon" size={12} />}
            {failed ? (
              <>
                <span role="alert">{T.voiceFailed(failure!)}</span> ·
                <button
                  type="button"
                  onClick={retry}
                  className="-my-3 min-h-10 px-1 font-semibold underline underline-offset-2"
                >
                  Coba lagi
                </button>
              </>
            ) : (
              // Selama diputar: posisi. Selebihnya: panjang seluruhnya —
              // yang ingin diketahui orang sebelum memutuskan mendengarkan.
              formatDuration(playing || position > 0 ? position : duration)
            )}
          </span>
        </div>
      </div>

      <button
        type="button"
        onClick={nextSpeed}
        aria-label={`Kecepatan putar ${speed}×, tekan untuk mengganti`}
        title="Kecepatan putar — tekan untuk mengganti"
        className={`h-9 min-w-10 shrink-0 self-center rounded-full px-2 text-[13px] font-bold tabular-nums transition ${
          mine ? 'bg-accent-deep hover:brightness-110' : 'bg-canvas text-muted hover:text-ink'
        }`}
      >
        {speed}×
      </button>
    </div>
  );
}

/** Kalimat untuk kode kegagalan media — yang bisa dilakukan orang berbeda untuk tiap sebab. */
function mediaErrorText(err: MediaError | null): string {
  switch (err?.code) {
    case MediaError.MEDIA_ERR_NETWORK:
      return 'koneksi terputus';
    case MediaError.MEDIA_ERR_DECODE:
      return 'berkasnya rusak';
    case MediaError.MEDIA_ERR_SRC_NOT_SUPPORTED:
      return 'formatnya tidak didukung browser ini';
    default:
      return 'berkas tidak bisa dibuka';
  }
}
