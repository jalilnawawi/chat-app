import { useCallback, useEffect, useRef, useState } from 'react';

/**
 * Batas panjang satu pesan suara.
 *
 * Lima menit Opus 32 kbps kira-kira 1,2 MB — jauh di bawah batas unggahan.
 * Batasnya bukan soal ukuran, melainkan soal pendengar: pesan suara sepuluh
 * menit adalah telepon yang tidak bisa dipotong.
 */
export const MAX_RECORDING_MS = 5 * 60_000;

/** Di bawah ini, hampir pasti tombolnya tersentuh tanpa sengaja. */
const MIN_RECORDING_MS = 700;

/** Berapa batang yang digambar di bilah perekam, dan seberapa sering. */
const LEVEL_BARS = 32;
const TICK_MS = 100;

/**
 * Format rekaman, urut dari yang paling disukai.
 *
 * Opus di dalam WebM adalah yang dihasilkan Chrome dan Firefox; Safari hanya
 * bisa MP4. Parameter `codecs` hanya untuk bertanya kepada browser — yang
 * dikirim ke server adalah tipe dasarnya, karena itu yang dicocokkan
 * narrowContainer dengan hasil sniffing.
 */
const FORMATS = [
  { mime: 'audio/webm;codecs=opus', base: 'audio/webm', ext: 'weba' },
  { mime: 'audio/ogg;codecs=opus', base: 'audio/ogg', ext: 'ogg' },
  { mime: 'audio/mp4', base: 'audio/mp4', ext: 'm4a' },
  { mime: 'audio/webm', base: 'audio/webm', ext: 'weba' },
];

function pickFormat() {
  if (typeof MediaRecorder === 'undefined') return null;
  return FORMATS.find(f => MediaRecorder.isTypeSupported(f.mime)) ?? null;
}

/**
 * Apakah perangkat ini bisa merekam sama sekali.
 *
 * getUserMedia hanya ada di konteks aman — HTTPS atau localhost. Membuka
 * aplikasi lewat IP LAN tanpa TLS membuatnya hilang, dan tombol yang pasti
 * gagal lebih baik tidak ditampilkan.
 */
export const canRecord = () =>
  typeof navigator !== 'undefined' &&
  Boolean(navigator.mediaDevices?.getUserMedia) &&
  pickFormat() !== null;

export type Recording = { file: File; durationMs: number };

type Phase = 'idle' | 'starting' | 'recording';

type Session = {
  stream: MediaStream;
  recorder: MediaRecorder;
  context: AudioContext | null;
  timer: number;
  startedAt: number;
  chunks: Blob[];
};

function describe(err: unknown): string {
  const name = err instanceof DOMException ? err.name : '';
  switch (name) {
    case 'NotAllowedError':
    case 'SecurityError':
      return 'Izin mikrofon ditolak. Izinkan lewat ikon gembok di bilah alamat.';
    case 'NotFoundError':
    case 'OverconstrainedError':
      return 'Tidak ada mikrofon yang tersambung.';
    case 'NotReadableError':
    case 'AbortError':
      return 'Mikrofon sedang dipakai aplikasi lain.';
    default:
      return 'Mikrofon tidak bisa dibuka.';
  }
}

/**
 * Merekam satu pesan suara.
 *
 * `stop(true)` menyerahkan rekamannya ke `onDone`; `stop(false)` membuangnya.
 * Rekaman yang mencapai batas panjang diserahkan sendiri, seperti kalau
 * tombol kirimnya ditekan — memotong diam-diam lalu membuang semuanya adalah
 * cara terburuk memberi tahu orang bahwa ada batas.
 *
 * Mikrofon dilepas di SETIAP jalan keluar, termasuk saat komponennya hilang.
 * Lampu merah di tab yang tetap menyala setelah orang selesai bicara adalah
 * hal pertama yang membuat orang berhenti memercayai sebuah aplikasi.
 */
export function useRecorder(onDone: (rec: Recording) => void) {
  const [phase, setPhase] = useState<Phase>('idle');
  const [elapsed, setElapsed] = useState(0);
  const [levels, setLevels] = useState<number[]>([]);
  const [error, setError] = useState<string | null>(null);

  const session = useRef<Session | null>(null);
  // Permintaan izin yang dibatalkan sebelum dijawab. getUserMedia tidak bisa
  // dibatalkan, jadi yang bisa dilakukan cuma menutup mikrofonnya begitu dia
  // akhirnya terbuka.
  const cancelled = useRef(false);
  const doneRef = useRef(onDone);
  doneRef.current = onDone;

  const release = useCallback((s: Session) => {
    clearInterval(s.timer);
    for (const track of s.stream.getTracks()) track.stop();
    void s.context?.close().catch(() => {});
  }, []);

  const stop = useCallback(
    (keep: boolean) => {
      if (phase === 'starting') {
        cancelled.current = true;
        setPhase('idle');
        return;
      }
      const s = session.current;
      if (!s) return;
      session.current = null;

      const durationMs = Math.min(performance.now() - s.startedAt, MAX_RECORDING_MS);
      const format = pickFormat()!;

      // Dijalankan SETELAH perekam menyerahkan potongan terakhirnya — stop()
      // memicu satu dataavailable lagi, dan berkas yang dirakit sebelum itu
      // kehilangan detik terakhirnya.
      const finish = () => {
        release(s);
        if (!keep) return;
        if (durationMs < MIN_RECORDING_MS) {
          setError('Rekaman terlalu pendek.');
          return;
        }
        const base = s.recorder.mimeType.split(';')[0] || format.base;
        const ext = FORMATS.find(f => f.base === base)?.ext ?? format.ext;
        const stamp = new Date().toISOString().slice(0, 19).replace(/[T:]/g, '-');
        const file = new File(s.chunks, `pesan-suara-${stamp}.${ext}`, { type: base });
        doneRef.current({ file, durationMs });
      };
      if (s.recorder.state !== 'inactive') {
        s.recorder.onstop = finish;
        s.recorder.stop();
      } else {
        finish();
      }

      setPhase('idle');
      setElapsed(0);
      setLevels([]);
    },
    [phase, release],
  );

  const stopRef = useRef(stop);
  stopRef.current = stop;

  const start = useCallback(async () => {
    if (phase !== 'idle') return;
    const format = pickFormat();
    if (!format) return;

    setError(null);
    setPhase('starting');
    cancelled.current = false;

    let stream: MediaStream;
    try {
      stream = await navigator.mediaDevices.getUserMedia({
        audio: { echoCancellation: true, noiseSuppression: true, autoGainControl: true },
      });
    } catch (err) {
      setPhase('idle');
      setError(describe(err));
      return;
    }
    if (cancelled.current) {
      for (const track of stream.getTracks()) track.stop();
      return;
    }

    let recorder: MediaRecorder;
    try {
      recorder = new MediaRecorder(stream, {
        mimeType: format.mime,
        // Suara bicara, bukan musik. 32 kbps Opus sudah jernih untuk itu.
        audioBitsPerSecond: 32_000,
      });
    } catch (err) {
      for (const track of stream.getTracks()) track.stop();
      setPhase('idle');
      setError(describe(err));
      return;
    }

    // Tingkat suara untuk bilah perekam: bukti yang terlihat bahwa mikrofonnya
    // benar-benar menangkap sesuatu. Tanpa ini, mikrofon yang dibisukan dari
    // perangkatnya menghasilkan rekaman lima menit berisi sunyi.
    let context: AudioContext | null = null;
    let analyser: AnalyserNode | null = null;
    try {
      context = new AudioContext();
      analyser = context.createAnalyser();
      analyser.fftSize = 512;
      context.createMediaStreamSource(stream).connect(analyser);
    } catch {
      context = null;
      analyser = null;
    }
    const samples = analyser ? new Uint8Array(analyser.fftSize) : null;

    const s: Session = {
      stream,
      recorder,
      context,
      timer: 0,
      startedAt: performance.now(),
      chunks: [],
    };
    recorder.ondataavailable = e => {
      if (e.data.size > 0) s.chunks.push(e.data);
    };

    s.timer = window.setInterval(() => {
      const now = performance.now() - s.startedAt;
      setElapsed(now);
      if (analyser && samples) {
        analyser.getByteTimeDomainData(samples);
        let sum = 0;
        for (const v of samples) sum += ((v - 128) / 128) ** 2;
        // Akar dari rata-rata kuadrat, lalu diperbesar: suara bicara biasa
        // jarang melewati 0,2, dan batang setinggi seperlima tidak terbaca
        // sebagai "sedang merekam".
        const level = Math.min(1, Math.sqrt(sum / samples.length) * 4);
        setLevels(prev => [...prev, level].slice(-LEVEL_BARS));
      }
      if (now >= MAX_RECORDING_MS) stopRef.current(true);
    }, TICK_MS);

    session.current = s;
    // Potongan tiap detik, bukan satu potongan di akhir: tab yang mendadak
    // ditutup setidaknya tidak menahan seluruh rekaman di satu buffer.
    recorder.start(1000);
    s.startedAt = performance.now();
    setElapsed(0);
    setLevels([]);
    setPhase('recording');
  }, [phase]);

  // Komponennya hilang di tengah rekaman — pindah percakapan, keluar akun:
  // rekaman dibuang, mikrofon dilepas.
  useEffect(
    () => () => {
      cancelled.current = true;
      const s = session.current;
      if (!s) return;
      session.current = null;
      s.recorder.onstop = null;
      if (s.recorder.state !== 'inactive') s.recorder.stop();
      release(s);
    },
    [release],
  );

  return { phase, elapsed, levels, error, clearError: () => setError(null), start, stop };
}
