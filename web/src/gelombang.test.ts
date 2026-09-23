import { expect, test } from 'bun:test';

import { decodeWaveform, encodeWaveform, WAVEFORM_BARS } from './gelombang';

// Dijalankan dengan `bun test`. Bun sudah jadi syarat menjalankan frontend
// ini, jadi test ini tidak menambah satu pun ketergantungan.
//
// Yang diuji di sini adalah satu-satunya bagian client yang punya jawaban benar
// dan salah tanpa perlu layar. Salahnya tidak akan pernah dilaporkan compiler:
// bentuk gelombang yang meleset tetap 40 karakter, tetap lolos CHECK database,
// dan hanya terlihat sebagai gambar yang tidak cocok dengan suaranya.

test('rekaman kosong tidak punya bentuk', () => {
  expect(encodeWaveform([])).toBeNull();
});

test('selalu tepat 40 karakter, sepanjang apa pun rekamannya', () => {
  for (const n of [1, 3, 39, 40, 41, 3000]) {
    const out = encodeWaveform(Array.from({ length: n }, () => 0.5));
    expect(out).not.toBeNull();
    expect(out!.length).toBe(WAVEFORM_BARS);
  }
});

test('urutannya tidak terbalik', () => {
  // Separuh sunyi lalu separuh penuh: 20 batang terendah, lalu 20 tertinggi.
  const levels = Array.from({ length: 3000 }, (_, i) => (i < 1500 ? 0 : 1));
  expect(encodeWaveform(levels)).toBe('A'.repeat(20) + '_'.repeat(20));
});

test('yang diambil per petak adalah puncaknya, bukan rata-ratanya', () => {
  // Satu ledakan di antara keheningan. Rata-rata akan meratakannya jadi hampir
  // nol, dan justru itu yang paling ingin dilihat orang sebelum menggeser.
  const levels = Array.from({ length: 400 }, (_, i) => (i === 5 ? 1 : 0));
  expect(encodeWaveform(levels)![0]).toBe('_');
});

test('rekaman lebih pendek dari 40 petak tidak punya petak kosong', () => {
  // Tiga detik: 30 contoh untuk 40 batang. Tanpa penjagaan, sepuluh batang
  // terakhir jadi keheningan yang tidak pernah ada.
  const levels = Array.from({ length: 30 }, () => 0.8);
  const out = encodeWaveform(levels)!;
  expect(out.length).toBe(WAVEFORM_BARS);
  expect(out).toBe(out[0]!.repeat(WAVEFORM_BARS));
  expect(out[0]).not.toBe('A');
});

test('nilai di luar 0..1 tetap menghasilkan abjad yang sah', () => {
  const liar = encodeWaveform(Array.from({ length: 40 }, (_, i) => (i % 2 ? -3 : 9)))!;
  expect(liar).toMatch(/^[A-Za-z0-9_-]{40}$/);
});

test('pulang pergi', () => {
  const asal = Array.from({ length: 40 }, (_, i) => i / 39);
  const kembali = decodeWaveform(encodeWaveform(asal)!)!;
  expect(kembali.length).toBe(WAVEFORM_BARS);
  for (let i = 0; i < 40; i++) expect(Math.abs(kembali[i]! - asal[i]!)).toBeLessThan(0.01);
});

test('yang salah bentuk ditolak, bukan digambar separuh', () => {
  const salah = ['', 'A'.repeat(39), 'A'.repeat(41), 'A'.repeat(39) + '+', 'A'.repeat(39) + '/'];
  for (const s of salah) expect(decodeWaveform(s)).toBeNull();
  expect(decodeWaveform(undefined)).toBeNull();
});
