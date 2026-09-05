/**
 * UUIDv7: 48 bit pertama berisi timestamp milidetik, sisanya acak.
 *
 * Dipakai sebagai id pesan yang dibuat di client. Dua sifatnya penting di sini:
 * unik tanpa koordinasi dengan server (jadi UI optimistik punya id final sejak
 * detik pertama), dan urut menurut waktu (enak dijadikan primary key di
 * Postgres karena penyisipannya tidak mengacak B-tree).
 */
export function uuidv7(): string {
  const bytes = new Uint8Array(16);
  crypto.getRandomValues(bytes);

  const ts = BigInt(Date.now());
  for (let i = 0; i < 6; i++) {
    bytes[i] = Number((ts >> BigInt(8 * (5 - i))) & 0xffn);
  }

  bytes[6] = (bytes[6]! & 0x0f) | 0x70; // versi 7
  bytes[8] = (bytes[8]! & 0x3f) | 0x80; // varian RFC 4122

  const hex = Array.from(bytes, b => b.toString(16).padStart(2, '0')).join('');
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
}
