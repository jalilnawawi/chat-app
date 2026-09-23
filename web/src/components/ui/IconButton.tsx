import type { ButtonHTMLAttributes, Ref } from 'react';
import Icon, { type IconName } from '../Icon';

/**
 * Tombol ikon persegi-membulat — kepala percakapan, penutup kabar, pembatal
 * balasan. Satu bentuk, supaya ukuran sasaran dan warna hover tidak berbeda
 * sedikit-sedikit di setiap tempat yang menyalinnya.
 *
 * `size`: `md` 40px (44px di layar sentuh), `sm` 40px rapat untuk bilah kabar
 * yang tipis (marginnya ditarik supaya bilah tidak ikut meninggi).
 * Label wajib: tombol ini tidak punya teks yang terlihat.
 */
export default function IconButton({
  ref,
  icon,
  label,
  iconSize,
  size = 'md',
  pressed,
  tone = 'muted',
  className = '',
  type = 'button',
  ...rest
}: Omit<ButtonHTMLAttributes<HTMLButtonElement>, 'aria-label'> & {
  ref?: Ref<HTMLButtonElement>;
  icon: IconName;
  label: string;
  iconSize?: number;
  size?: 'md' | 'sm';
  /** Tombol berkeadaan (panel terbuka): diberi `aria-pressed` dan nada aksen. */
  pressed?: boolean;
  /** `kepala`: untuk bidang gelap kepala percakapan, tempat nada redup biasa hilang. */
  tone?: 'muted' | 'danger' | 'kepala';
}) {
  const base =
    size === 'md'
      ? 'size-10 [@media(pointer:coarse)]:size-11'
      : '-my-1.5 size-10';
  const color =
    pressed === true
      ? 'bg-accent-soft text-accent-text'
      : tone === 'danger'
        ? 'text-danger hover:bg-surface'
        : tone === 'kepala'
          ? 'text-kepala-redup hover:bg-kepala-tekan hover:text-kepala-teks'
          : 'text-muted hover:bg-canvas hover:text-ink';
  return (
    <button
      ref={ref}
      type={type}
      aria-label={label}
      aria-pressed={pressed}
      title={rest.title ?? label}
      className={`grid shrink-0 place-items-center rounded-xl transition ${base} ${color} ${className}`}
      {...rest}
    >
      <Icon name={icon} size={iconSize ?? (size === 'md' ? 20 : 16)} />
    </button>
  );
}
