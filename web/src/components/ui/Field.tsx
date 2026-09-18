import { useId, type InputHTMLAttributes, type Ref } from 'react';

/** Bentuk kolom isian di seluruh panel. Dipakai juga oleh kolom yang bukan `Field`. */
export const KOLOM =
  'w-full rounded-xl border border-line-strong bg-canvas px-3.5 py-2.5 text-[15px] outline-none transition focus:bg-surface disabled:border-line disabled:text-muted';

/**
 * Kolom isian berlabel.
 *
 * Label yang terlihat, bukan teks contoh. Teks contoh hilang pada ketukan
 * pertama: orang yang berhenti di tengah pengisian kehilangan satu-satunya
 * keterangan tentang kolom yang sedang dia isi, dan pembaca layar tidak pernah
 * mendapatkannya sama sekali. Untuk kolom password — tiga di antaranya
 * bersebelahan di panel akun — itu bedanya antara mengisi dan menebak.
 */
export default function Field({
  ref,
  label,
  hint,
  id,
  ...rest
}: InputHTMLAttributes<HTMLInputElement> & {
  ref?: Ref<HTMLInputElement>;
  label: string;
  /** Keterangan di bawah kolom — syarat, akibat, atau batas. */
  hint?: string;
}) {
  const auto = useId();
  const kolomId = id ?? auto;
  const hintId = hint ? `${kolomId}-ket` : undefined;

  return (
    <div className="mt-2 first:mt-0">
      <label htmlFor={kolomId} className="block text-[13px] font-semibold text-muted">
        {label}
      </label>
      <input
        ref={ref}
        id={kolomId}
        aria-describedby={hintId}
        className={`mt-1.5 ${KOLOM}`}
        {...rest}
      />
      {hint && (
        <p id={hintId} className="mt-1.5 text-[12px] leading-relaxed text-muted">
          {hint}
        </p>
      )}
    </div>
  );
}

/**
 * Kabar hasil — berhasil atau gagal.
 *
 * Wadahnya SELALU terpasang, juga saat belum ada kabarnya. Wadah yang baru
 * muncul bersama teksnya tidak dibacakan pembaca layar: yang diumumkan adalah
 * perubahan DI DALAM daerah hidup, dan daerah yang lahir bersama isinya tidak
 * pernah berubah.
 */
export function Kabar({ tone, text }: { tone: 'ok' | 'bahaya'; text: string | null }) {
  return (
    <p
      // alert menyela, status menunggu giliran. "Password saat ini salah"
      // pantas menyela; "Tautan verifikasi dikirim" tidak.
      role={tone === 'bahaya' ? 'alert' : 'status'}
      className={`text-[13px] ${tone === 'bahaya' ? 'text-danger' : 'text-ok'} ${text ? 'mt-2' : ''}`}
    >
      {text}
    </p>
  );
}
