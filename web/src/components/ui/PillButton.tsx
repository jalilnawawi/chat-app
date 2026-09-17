import type { ButtonHTMLAttributes, ReactNode, Ref } from 'react';
import Icon, { type IconName } from '../Icon';

/**
 * Tombol berbentuk pil — satu tempat untuk bentuk yang sebelumnya disalin di
 * kolom tulis, bilah rekam, baris pesan gagal, kabar hapus, dan form sunting.
 *
 * Nada, bukan warna: `solid` adalah tindakan utama di kanvas, `outline` yang
 * tenang, `danger` untuk memperbaiki kegagalan, `ghost` untuk jalan keluar.
 * Varian `onTeal*` hidup di dalam gelembung sendiri, tempat teal sudah jadi
 * latarnya.
 *
 * Nonaktif tidak pernah dipudarkan: tombol utama yang nonaktif jadi bergaris
 * dan tenang, bukan bidang teal yang terbaca seperti tombol macet.
 */
export type PillTone =
  | 'solid'
  | 'outline'
  | 'quiet'
  | 'danger'
  | 'ghost'
  | 'onTealSolid'
  | 'onTealGhost';

const TONE: Record<PillTone, string> = {
  solid:
    'border-transparent bg-accent text-accent-ink hover:brightness-110 active:scale-95 disabled:border-line-strong disabled:bg-surface disabled:text-muted disabled:hover:brightness-100 disabled:active:scale-100',
  outline:
    'border-line-strong bg-surface text-accent-text hover:border-accent hover:bg-accent-soft active:scale-95',
  /** Bergaris dan redup — "Muat pesan lama", tindakan yang tidak mendesak. */
  quiet: 'border-line-strong bg-surface font-medium text-muted hover:border-accent hover:text-accent-text',
  danger: 'border-danger bg-surface text-danger hover:bg-danger-soft',
  ghost: 'border-transparent text-muted hover:bg-surface hover:text-ink',
  onTealSolid:
    'border-transparent bg-accent-ink text-accent hover:brightness-95 disabled:border-accent-ink disabled:bg-transparent disabled:text-accent-ink',
  onTealGhost: 'border-transparent text-accent-ink hover:bg-accent-deep',
};

const SIZE = {
  /** 44px — kolom tulis, bilah rekam, baris gagal. */
  md: 'h-11 px-4 text-[15px]',
  /** 40px — kabar di atas kolom tulis, form sunting. */
  sm: 'h-10 px-3.5 text-[14px]',
} as const;

export default function PillButton({
  ref,
  tone = 'solid',
  size = 'md',
  icon,
  iconSize,
  children,
  /** Label disembunyikan di layar sempit; ikonnya tetap. */
  compact = false,
  className = '',
  type = 'button',
  ...rest
}: ButtonHTMLAttributes<HTMLButtonElement> & {
  ref?: Ref<HTMLButtonElement>;
  tone?: PillTone;
  size?: keyof typeof SIZE;
  icon?: IconName;
  iconSize?: number;
  compact?: boolean;
  children?: ReactNode;
}) {
  return (
    <button
      ref={ref}
      type={type}
      className={`flex shrink-0 items-center justify-center gap-2 rounded-full border font-semibold transition ${SIZE[size]} ${
        compact ? 'px-3 sm:px-4' : ''
      } ${TONE[tone]} ${className}`}
      {...rest}
    >
      {icon && <Icon name={icon} size={iconSize ?? (size === 'md' ? 20 : 17)} />}
      {children !== undefined &&
        (compact ? <span className="hidden sm:inline">{children}</span> : children)}
    </button>
  );
}
