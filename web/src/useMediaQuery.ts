import { useEffect, useState } from 'react';

/** Mengikuti media query selagi halaman terbuka. */
export function useMediaQuery(query: string): boolean {
  const [match, setMatch] = useState(() =>
    typeof window !== 'undefined' ? window.matchMedia(query).matches : false,
  );
  useEffect(() => {
    const mq = window.matchMedia(query);
    const onChange = () => setMatch(mq.matches);
    onChange();
    mq.addEventListener('change', onChange);
    return () => mq.removeEventListener('change', onChange);
  }, [query]);
  return match;
}

/**
 * Apakah penunjuk utamanya jari.
 *
 * Yang ditanyakan adalah `pointer: coarse`, bukan lebar layar: tablet dengan
 * papan ketik fisik tetap layar sentuh, dan jendela sempit di laptop tetap
 * punya tombol Shift.
 */
export const useCoarsePointer = () => useMediaQuery('(pointer: coarse)');
