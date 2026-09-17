import { useEffect, useLayoutEffect, useRef, useState, type KeyboardEvent } from 'react';
import { createPortal } from 'react-dom';
import EmojiPicker from './EmojiPicker';
import Icon, { type IconName } from './Icon';
import QuickReactions from './QuickReactions';

export type MenuItem = {
  key: string;
  label: string;
  icon: IconName;
  onSelect: () => void;
  /** Tindakan yang merusak: diberi warna bahaya dan dipisah garis dari sisanya. */
  danger?: boolean;
};

/** Reaksi yang bisa diberikan dari menu; `given` adalah yang sudah diberikan pembaca. */
export type MenuReactions = { given: string[]; onReact: (emoji: string) => void };

/** Di bawah lebar ini menunya naik dari bawah layar, bukan menempel di tombol. */
const SHEET_QUERY = '(max-width: 39.99rem)';

/**
 * Satu panah kecil di pojok kanan atas gelembung untuk semua tindakan atas
 * sebuah pesan.
 *
 * Panahnya menumpang DI DALAM gelembung, bukan di sampingnya: ruang di sisi
 * gelembung sudah dipakai tombol reaksi, dan dua tombol berjajar di setiap
 * pesan membuat riwayat terbaca seperti tabel. Di perangkat bertetikus dia
 * baru muncul saat gelembungnya disentuh kursor — atau saat dia sendiri
 * mendapat fokus papan ketik, karena yang tak terlihat tetap harus bisa
 * dijangkau. Di layar sentuh tidak ada kursor, jadi dia selalu tampil.
 *
 * Panahnya tidak pernah menimpa kalimat: gelembung memesan sudutnya lewat
 * `MenuCornerSpacer`, sehingga hanya baris pertama yang membungkus di
 * sampingnya. Area tekannya diperluas ke 40 px ke arah luar dan ke kiri, bukan
 * ke bawah, supaya jari yang memilih teks tidak membuka menu.
 *
 * Di layar lebar daftarnya menempel di tombol; di layar sempit dia naik dari
 * bawah sebagai lembar, tempat ibu jari memang berada. Keduanya daftar yang
 * sama dengan urutan yang sama — yang merusak selalu paling akhir.
 */
export default function MessageMenu({
  items,
  label,
  mine,
  summary,
  reactions,
}: {
  items: MenuItem[];
  label: string;
  /** Menentukan warna panah — harus sama dengan gelembungnya. */
  mine: boolean;
  /** Cuplikan pesan sasaran, ditulis di kepala lembar supaya jelas pesan mana yang diurus. */
  summary?: string;
  /**
   * Baris reaksi cepat di atas daftar. Di layar sentuh inilah satu-satunya
   * jalan memberi reaksi — tombol reaksi di samping gelembung tidak ada di
   * sana — jadi dia berada di tempat ibu jari sudah berada.
   */
  reactions?: MenuReactions;
}) {
  const [open, setOpen] = useState(false);
  const [sheet, setSheet] = useState(false);
  const [pos, setPos] = useState<{ top: number; left: number } | null>(null);
  // Dibuka dengan tetikus atau jari: fokus masuk ke wadah menunya, bukan ke
  // butir pertama — kalau tidak, butir itu tampil bercincin seolah dipilih.
  const [byPointer, setByPointer] = useState(false);
  // Papan emoji lengkap menggantikan daftar tindakan, di wadah yang sama.
  const [picking, setPicking] = useState(false);
  const button = useRef<HTMLButtonElement>(null);
  const menu = useRef<HTMLDivElement>(null);

  const close = (refocus: boolean) => {
    setOpen(false);
    setPos(null);
    setPicking(false);
    if (refocus) button.current?.focus();
  };

  const toggle = (e: { detail: number }) => {
    if (open) return close(false);
    setSheet(window.matchMedia(SHEET_QUERY).matches);
    setByPointer(e.detail > 0);
    setOpen(true);
  };

  // Letak popover dihitung dari tombolnya: di bawah bila muat, di atas bila
  // tidak. Ditambatkan ke `body` lewat portal, karena daftar riwayat
  // menggulir dan memotong apa pun yang mencoba keluar dari tepinya.
  useLayoutEffect(() => {
    if (!open || sheet) return;
    const b = button.current?.getBoundingClientRect();
    const m = menu.current?.getBoundingClientRect();
    if (!b || !m) return;
    const gap = 6;
    const below = b.bottom + gap + m.height <= window.innerHeight - 8;
    const top = below ? b.bottom + gap : Math.max(8, b.top - gap - m.height);
    const left = Math.min(Math.max(8, b.right - m.width), window.innerWidth - m.width - 8);
    setPos({ top, left });
  }, [open, sheet, picking]);

  // Fokus pindah ke butir pertama begitu daftarnya terbuka, supaya panah
  // langsung bekerja — dan pembaca layar langsung tahu dia ada di dalam menu.
  // Popover baru bisa difokus setelah letaknya dihitung: sebelum itu dia
  // masih `visibility: hidden`, dan browser menolak memfokus yang tak terlihat.
  const ready = open && (sheet || pos !== null);
  useEffect(() => {
    if (!ready) return;
    if (byPointer) menu.current?.focus();
    else menu.current?.querySelector<HTMLButtonElement>('[role="menuitem"]')?.focus();
  }, [ready]); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (!open) return;
    const onDown = (e: PointerEvent) => {
      const t = e.target as Node;
      if (menu.current?.contains(t) || button.current?.contains(t)) return;
      close(false);
    };
    // Menggulir riwayat membuat popover tertinggal di tempat yang salah.
    const onScroll = (e: Event) => {
      if (!sheet && !menu.current?.contains(e.target as Node)) close(false);
    };
    document.addEventListener('pointerdown', onDown);
    document.addEventListener('scroll', onScroll, true);
    window.addEventListener('resize', onScroll);
    return () => {
      document.removeEventListener('pointerdown', onDown);
      document.removeEventListener('scroll', onScroll, true);
      window.removeEventListener('resize', onScroll);
    };
  }, [open, sheet]);

  function onKeyDown(e: KeyboardEvent<HTMLDivElement>) {
    const all = Array.from(
      menu.current?.querySelectorAll<HTMLButtonElement>('[role="menuitem"]') ?? [],
    );
    const at = all.indexOf(document.activeElement as HTMLButtonElement);
    const go = (i: number) => all[(i + all.length) % all.length]?.focus();

    switch (e.key) {
      case 'ArrowDown':
        e.preventDefault();
        go(at + 1);
        break;
      case 'ArrowUp':
        e.preventDefault();
        go(at - 1);
        break;
      case 'Home':
        e.preventDefault();
        go(0);
        break;
      case 'End':
        e.preventDefault();
        go(all.length - 1);
        break;
      case 'Escape':
        e.preventDefault();
        e.stopPropagation();
        close(true);
        break;
      case 'Tab':
        if (!sheet) {
          close(false);
          break;
        }
        // Lembar menutupi seluruh layar: fokus berputar di dalamnya, tidak
        // lolos ke halaman yang sedang tertutup tirai.
        {
          const focusables = Array.from(
            menu.current?.querySelectorAll<HTMLButtonElement>('button') ?? [],
          );
          const i = focusables.indexOf(document.activeElement as HTMLButtonElement);
          const next = e.shiftKey ? i - 1 : i + 1;
          e.preventDefault();
          focusables[(next + focusables.length) % focusables.length]?.focus();
        }
        break;
    }
  }

  const select = (item: MenuItem) => {
    close(true);
    item.onSelect();
  };

  const firstDanger = items.findIndex(i => i.danger);

  const react = (emoji: string) => {
    close(true);
    reactions?.onReact(emoji);
  };

  const reactionRow = reactions && (
    <div className={sheet ? 'mb-1 px-1' : 'mb-1 border-b border-line pb-1.5'}>
      <QuickReactions
        given={reactions.given}
        onReact={react}
        onMore={() => setPicking(true)}
        variant={sheet ? 'sheet' : 'popover'}
      />
    </div>
  );

  const picker = reactions && (
    <EmojiPicker
      autoFocus={!byPointer}
      onPick={emoji => {
        // Dari papan lengkap: emoji yang SUDAH diberikan tidak dilepas. Orang
        // yang mencarinya di antara ratusan pilihan sedang ingin memberi.
        if (reactions.given.includes(emoji)) close(true);
        else react(emoji);
      }}
    />
  );

  const list = items.map((item, i) => (
    <div key={item.key}>
      {i === firstDanger && i > 0 && <div role="separator" className="mx-2 my-1 h-px bg-line" />}
      <button
        type="button"
        role="menuitem"
        onClick={() => select(item)}
        className={`flex w-full items-center gap-3 rounded-xl px-3 text-left font-medium transition ${
          sheet ? 'min-h-13 text-[16px]' : 'min-h-11 text-[15px]'
        } ${
          item.danger
            ? 'text-danger hover:bg-danger-soft focus-visible:bg-danger-soft'
            : 'hover:bg-canvas focus-visible:bg-canvas'
        }`}
      >
        <Icon name={item.icon} size={sheet ? 22 : 20} className={item.danger ? '' : 'text-muted'} />
        {item.label}
      </button>
    </div>
  ));

  return (
    <>
      <button
        ref={button}
        type="button"
        onClick={toggle}
        aria-label={label}
        aria-haspopup="menu"
        aria-expanded={open}
        data-menu-trigger=""
        title="Tindakan pesan (klik kanan juga bisa)"
        className={`absolute top-1 right-1 z-[1] grid size-7 place-items-center rounded-full transition before:absolute before:-top-1.5 before:-right-1.5 before:-bottom-0.5 before:-left-1.5 before:content-[''] [@media(pointer:coarse)]:before:-inset-2 ${
          mine ? 'bg-accent text-accent-ink hover:bg-accent-ink/15' : 'bg-surface text-muted hover:bg-canvas hover:text-ink'
          // Selalu terlihat, redup saat diam: tindakan yang harus dicari dengan
          // kursor dulu tidak pernah ditemukan sebagian penggunanya. Menyala
          // penuh saat BARIS pesannya disentuh kursor — cakupan yang sama
          // dengan tombol reaksi — atau saat difokus.
        } ${open ? 'opacity-100' : 'opacity-80 group-hover:opacity-100 focus-visible:opacity-100'}`}
      >
        <Icon name="buka-menu" size={18} className={`transition ${open ? 'rotate-180' : ''}`} />
      </button>

      {open &&
        createPortal(
          sheet ? (
            <div className="fixed inset-0 z-50 flex items-end bg-ink/40 backdrop-blur-[2px]">
              {/* Lembar adalah dialog kecil: kepala yang menyebut pesannya,
                  menu, lalu "Batal" — yang bukan tindakan atas pesan, jadi
                  tidak ikut dibacakan sebagai butir menu. */}
              <div
                ref={menu}
                role="dialog"
                aria-modal="true"
                aria-label={label}
                tabIndex={-1}
                onKeyDown={onKeyDown}
                className="cincin-sendiri w-full rounded-t-[24px] border border-line bg-surface px-2 pt-2 pb-[max(0.75rem,env(safe-area-inset-bottom))] shadow-pop outline-none"
              >
                <div aria-hidden className="mx-auto mb-2 h-1 w-10 rounded-full bg-line" />
                {summary && (
                  <p className="mx-3 mb-2 line-clamp-2 border-l-[3px] border-accent pl-2.5 text-[14px] text-muted">
                    {summary}
                  </p>
                )}
                {picking ? (
                  <div className="flex justify-center pb-1">{picker}</div>
                ) : (
                  <>
                    {reactionRow}
                    <div role="menu" aria-label={label}>
                      {list}
                    </div>
                  </>
                )}
                <div className="mx-2 my-1 h-px bg-line" />
                <button
                  type="button"
                  onClick={() => close(true)}
                  className="flex min-h-13 w-full items-center justify-center rounded-xl px-3 text-[16px] font-semibold text-muted transition hover:bg-canvas"
                >
                  Batal
                </button>
              </div>
            </div>
          ) : (
            // Wadahnya bukan menu: baris reaksi dan papan emoji bukan butir
            // menu. Menunya adalah daftar tindakan di dalamnya.
            <div
              ref={menu}
              role={picking ? 'dialog' : undefined}
              aria-label={picking ? 'Pilih reaksi' : undefined}
              tabIndex={-1}
              onKeyDown={onKeyDown}
              style={pos ? { top: pos.top, left: pos.left } : { top: 0, left: 0, visibility: 'hidden' }}
              className={`cincin-sendiri fixed z-50 rounded-2xl border border-line bg-surface shadow-pop outline-none ${
                picking ? 'overflow-hidden' : 'w-80 p-1.5'
              }`}
            >
              {picking ? (
                picker
              ) : (
                <>
                  {reactionRow}
                  <div role="menu" aria-label={label}>
                    {list}
                  </div>
                </>
              )}
            </div>
          ),
          document.body,
        )}
    </>
  );
}

/**
 * Ruang yang dipesan untuk panah menu di sudut kanan atas gelembung.
 *
 * Mengambang, bukan padding: yang terdorong hanya baris pertama, dan kalimat
 * panjang di bawahnya tetap memakai seluruh lebar gelembung. Ukurannya sama
 * persis dengan panahnya (28 px, 4 px dari tepi atas dan kanan gelembung).
 *
 * Dia harus berada di DALAM paragraf yang baris pertamanya dia dorong. Di
 * luar paragraf itu, lebar gelembung dihitung tanpa dia — gelembung "Ok"
 * tetap selebar dua huruf, dan kata itu terdesak ke bawah panah.
 */
export function MenuCornerSpacer() {
  return <span aria-hidden className="float-right -mt-1.5 -mr-1.5 ml-1.5 block size-7" />;
}
