/**
 * Lambang aplikasi: satu gelembung dengan ekornya.
 *
 * Muncul di dua tempat saja — layar tunggu dan halaman masuk — dan keduanya
 * adalah saat orang belum melihat apa pun lagi.
 */
export default function Tanda({ size = 44 }: { size?: number }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 44 44"
      aria-hidden="true"
      className="text-accent"
    >
      <path
        d="M8 17a9 9 0 0 1 9-9h10a9 9 0 0 1 9 9v6a9 9 0 0 1-9 9h-9l-7.5 5.5a1 1 0 0 1-1.6-.8V30.6A9 9 0 0 1 8 23z"
        fill="currentColor"
      />
      <circle cx="16.5" cy="20" r="2.1" fill="var(--color-accent-ink)" />
      <circle cx="22" cy="20" r="2.1" fill="var(--color-accent-ink)" />
      <circle cx="27.5" cy="20" r="2.1" fill="var(--color-accent-ink)" />
    </svg>
  );
}
