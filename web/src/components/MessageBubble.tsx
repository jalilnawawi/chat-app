import { useEffect, useRef, useState } from 'react';
import { useStore } from '../store';
import AttachmentList from './AttachmentList';
import Avatar from './Avatar';
import Icon from './Icon';
import ReactionRow, { ReactionButton } from './ReactionRow';
import type { Message, PendingMessage, ReplyPreview, SystemEvent } from '../types';

type Props = {
  message?: Message;
  pending?: PendingMessage;
  mine: boolean;
  showAuthor: boolean;
  authorName: string;
  /** Foto pengirim, hanya dipakai pada pesan pembuka sebuah rentetan di grup. */
  authorAvatar?: string;
  /**
   * Pesan pertama dari sebuah rentetan — pengirim yang sama, berdekatan waktu.
   *
   * Yang menentukan bentuk sudutnya, jarak ke pesan sebelumnya, dan apakah nama
   * serta fotonya ditulis ulang. Sepuluh pesan berturut-turut dari satu orang
   * yang masing-masing mengulang nama dan foto adalah sepuluh kali menjawab
   * pertanyaan yang hanya ditanyakan sekali.
   */
  firstOfRun?: boolean;
  /** Ruang kosong selebar foto, supaya gelembung satu rentetan tetap sejajar. */
  gutter?: boolean;
  readByPeer?: boolean;
  /** Nama yang layak disorot di dalam teks; isMe menentukan sorotannya tegas. */
  highlights?: { name: string; isMe: boolean }[];
  /** Apakah pesan ini memanggil pembaca — menyalakan pengamat "sudah terlihat". */
  callsMe?: boolean;
  nameOf?: (userId: string) => string;
  onReply?: (m: Message) => void;
  /** `seq` ikut supaya pesan yang belum termuat tetap bisa dituju. */
  onJump?: (messageId: string, seq?: number) => void;
  onForward?: (m: Message) => void;
  /** Ada hanya bila pembaca BOLEH menyematkan — tombolnya tidak ditampilkan kepada yang tidak boleh. */
  onTogglePin?: (m: Message, pinned: boolean) => void;
  pinned?: boolean;
  /** Dipanggil sekali saat pesan yang memanggil kita benar-benar masuk layar. */
  onSeen?: (seq: number) => void;
};

const time = (iso: string) =>
  new Date(iso).toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit' });

/** Satu kata untuk pesan yang isinya hanya lampiran; kutipan kosong terbaca
 *  seperti pesan kosong, bukan seperti foto yang sedang dibalas. */
const kindLabel: Record<string, string> = {
  image: '📷 Gambar',
  video: '🎬 Video',
  audio: '🎵 Rekaman suara',
  file: '📎 Lampiran',
};

export default function MessageBubble({
  message,
  pending,
  mine,
  showAuthor,
  authorName,
  authorAvatar,
  firstOfRun = true,
  gutter = false,
  readByPeer,
  highlights = [],
  callsMe = false,
  nameOf,
  onReply,
  onJump,
  onForward,
  onTogglePin,
  pinned = false,
  onSeen,
}: Props) {
  const editMessage = useStore(s => s.editMessage);
  const deleteMessage = useStore(s => s.deleteMessage);
  const retryMessage = useStore(s => s.retryMessage);

  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(message?.body ?? '');
  const rootRef = useRef<HTMLDivElement>(null);

  // Catatan sistem bukan ucapan siapa pun, jadi dia tidak berbentuk gelembung:
  // tidak berpihak kiri atau kanan, tidak punya tombol, tidak bisa disentuh.
  // Bentuknya sendiri yang mengatakan "ini bukan sesuatu yang dikatakan orang".
  if (message?.kind === 'system' && message.systemEvent) {
    const ev = message.systemEvent;
    // Catatan sematan menunjuk sebuah pesan, dan menekannya membawa ke sana —
    // satu-satunya catatan sistem yang punya tujuan. Bentuknya tetap baris
    // tengah; yang berubah cuma bahwa dia bisa ditekan.
    const target = ev.type === 'message.pinned' && ev.messageId && onJump ? ev : null;
    const cls =
      'rounded-full border border-line bg-surface px-3 py-1 text-center text-[12px] text-muted';
    return (
      <div className="my-3 flex justify-center px-6">
        {target ? (
          <button
            type="button"
            onClick={() => onJump?.(target.messageId!, target.messageSeq)}
            className={`${cls} inline-flex items-center gap-1.5 transition hover:border-accent hover:text-accent-text`}
          >
            <Icon name="sematan" size={13} />
            {systemText(ev)}
          </button>
        ) : (
          <p className={cls}>{systemText(ev)}</p>
        )}
      </div>
    );
  }

  const body = message?.body ?? pending?.body ?? '';
  const createdAt = message?.createdAt ?? pending?.createdAt ?? '';
  const deleted = Boolean(message?.deletedAt);
  const attachments = (message?.attachments ?? pending?.attachments ?? []).filter(() => !deleted);
  const replyTo = message?.replyTo ?? pending?.replyTo;

  // Penanda sebutan baru padam setelah pesannya BENAR-BENAR terlihat, bukan
  // saat percakapannya dibuka. Itu seluruh alasan mention_ack_seq terpisah dari
  // last_read_seq di server — dan di sinilah janji itu ditepati: seorang yang
  // membuka ruang lalu langsung pindah tidak pernah melewati ambang ini.
  useEffect(() => {
    if (!callsMe || !message || !onSeen) return;
    const el = rootRef.current;
    if (!el) return;

    const observer = new IntersectionObserver(
      entries => {
        if (entries.some(e => e.isIntersecting)) {
          onSeen(message.seq);
          observer.disconnect();
        }
      },
      { threshold: 0.6 },
    );
    observer.observe(el);
    return () => observer.disconnect();
  }, [callsMe, message, onSeen]);

  async function saveEdit() {
    if (!message) return;
    const next = draft.trim();
    setEditing(false);
    if (next && next !== message.body) await editMessage(message.id, next);
  }

  /**
   * Sudut gelembung.
   *
   * Tiga sudut selalu bulat penuh; yang keempat — di sisi pengirimnya — selalu
   * rapat. Itu yang membuat sederet pesan dari satu orang terbaca sebagai satu
   * blok dengan tulang punggung lurus, bukan sebagai lima benda terpisah yang
   * kebetulan berdekatan. Sudut atas ikut dirapatkan saat pesannya BUKAN
   * pembuka rentetan, sehingga sambungannya rata.
   */
  const sudut = mine
    ? `rounded-bubble rounded-br-[7px] ${firstOfRun ? '' : 'rounded-tr-[7px]'}`
    : `rounded-bubble rounded-bl-[7px] ${firstOfRun ? '' : 'rounded-tl-[7px]'}`;

  return (
    <div
      ref={rootRef}
      id={message ? `msg-${message.id}` : undefined}
      className={`group flex ${firstOfRun ? 'mt-3' : 'mt-0.5'} ${
        mine ? 'justify-end' : 'justify-start'
      }`}
    >
      {/* Foto pengirim hanya di pembuka rentetan; sisanya dapat ruang kosong
          selebar foto itu, supaya seluruh rentetan berdiri di garis yang sama.

          Disejajarkan ke ATAS, bukan ke bawah: yang di bawah blok ini adalah
          baris jam dan tombol, dan foto yang berdiri di sebelahnya terbaca
          seperti milik baris itu, bukan milik pesannya. */}
      {!mine && gutter && (
        <span className="mt-0.5 mr-2 w-8 shrink-0 self-start">
          {showAuthor && <Avatar name={authorName} url={authorAvatar} size={32} />}
        </span>
      )}

      <div
        className={`relative flex max-w-[min(80%,34rem)] min-w-0 flex-col ${mine ? 'items-end' : 'items-start'}`}
      >
        {showAuthor && !mine && (
          <p className="mb-1 px-1 text-[13px] font-bold text-accent-text">{authorName}</p>
        )}

        {/* Gelembung dan tombol reaksinya sebaris: tombolnya di sisi yang
            menghadap ke tengah layar — kiri untuk pesan sendiri, kanan untuk
            pesan orang. */}
        <div className={`flex max-w-full items-center gap-1 ${mine ? 'flex-row-reverse' : ''}`}>
          <div
            className={`min-w-0 text-[15px] leading-[1.45] ${sudut} ${
              // Gelembung yang isinya cuma gambar dibuat rapat: padding tebal di
              // sekeliling foto membuatnya tampak seperti bingkai, bukan seperti
              // foto yang dikirim.
              attachments.length > 0 && body === '' && !deleted && !replyTo ? 'p-1.5' : 'px-3.5 py-2.5'
            } ${
              deleted
                ? 'border border-dashed border-line-strong text-muted italic'
                : mine
                  ? 'bg-accent text-accent-ink'
                  : 'border border-line bg-surface'
            } ${
              // Pesan yang memanggil kita diberi PINGGIRAN mangga, bukan bidang
              // mangga. Latar sudah dipakai untuk membedakan pesan sendiri dari
              // pesan orang, dan dua arti pada satu isyarat tidak bisa dibaca
              // sekaligus. Pinggiran adalah isyarat ketiga yang masih kosong.
              callsMe && !deleted ? 'ring-2 ring-call' : ''
            } ${pending?.status === 'failed' ? 'opacity-70 ring-1 ring-danger' : ''}`}
          >
            {/* Penanda terusan berdiri di atas isinya, dan hanya itu: siapa
                penulis aslinya tidak ikut dibawa. Kalimat yang diteruskan
                tanpa penanda ini terbaca sebagai kalimat pengirimnya sendiri. */}
            {message?.forwarded && !deleted && (
              <p
                className={`mb-1 flex items-center gap-1 text-[12px] font-semibold italic ${
                  mine ? 'text-accent-ink/80' : 'text-muted'
                }`}
              >
                <Icon name="teruskan" size={13} />
                Diteruskan
              </p>
            )}

            {replyTo && !deleted && (
              <Quote reply={replyTo} mine={mine} nameOf={nameOf} onJump={onJump} />
            )}

            {editing ? (
              <textarea
                autoFocus
                value={draft}
                onChange={e => setDraft(e.target.value)}
                onBlur={() => void saveEdit()}
                onKeyDown={e => {
                  if (e.key === 'Enter' && !e.shiftKey) {
                    e.preventDefault();
                    void saveEdit();
                  }
                  if (e.key === 'Escape') setEditing(false);
                }}
                className="w-full resize-none bg-transparent text-inherit outline-none"
              />
            ) : (
              <div className="flex flex-col gap-1.5">
                <AttachmentList attachments={attachments} mine={mine} />
                {/* Pesan boleh hanya berisi lampiran — mengirim foto tanpa
                    keterangan adalah hal yang paling biasa dilakukan orang. */}
                {(body !== '' || deleted) && (
                  <p className="break-words whitespace-pre-wrap">
                    {deleted ? 'Pesan ini dihapus' : <Highlighted text={body} names={highlights} />}
                  </p>
                )}
              </div>
            )}
          </div>

          {message && !deleted && !editing ? (
            <ReactionButton message={message} mine={mine} />
          ) : (
            // Ruangnya tetap dipesan: pesan yang baru terkonfirmasi tidak boleh
            // menyempit dan membungkus ulang kalimatnya di depan mata.
            pending && <span aria-hidden className="w-8 shrink-0" />
          )}
        </div>

        {message && !deleted && <ReactionRow message={message} mine={mine} />}

        <div
          className={`mt-1 flex items-center gap-2 px-1 text-[11.5px] text-muted ${
            mine ? 'justify-end' : 'justify-start'
          }`}
        >
          {pinned && !deleted && (
            <span title="Disematkan" className="text-accent-text">
              <Icon name="sematan" size={13} />
            </span>
          )}
          {createdAt && <span className="tabular-nums">{time(createdAt)}</span>}
          {message?.editedAt && !deleted && <span>diedit</span>}

          {pending?.status === 'sending' && <span>mengirim…</span>}
          {pending?.status === 'failed' && (
            <button
              onClick={() => void retryMessage(pending.conversationId, pending.id)}
              className="font-semibold text-danger underline underline-offset-2"
              // Sebab kegagalan ikut ditampilkan: pesan yang ditolak karena
              // kuota @semua terlihat persis sama dengan yang gagal karena
              // jaringan, dan keduanya menuntut tindakan yang berbeda.
              title={pending.error}
            >
              {pending.error ?? 'gagal'} — coba lagi
            </button>
          )}

          {/* Dua centang, bukan kata "dibaca": dia duduk di baris yang sudah
              penuh angka dan kata, dan bentuk yang tidak perlu dibaca lebih
              cepat sampai daripada kata yang perlu. Judulnya tetap ada untuk
              yang memakai pembaca layar. */}
          {mine && message && !deleted && readByPeer && (
            <span title="Sudah dibaca" className="text-accent-text">
              <Icon name="terbaca" size={15} />
            </span>
          )}

          {message && !deleted && !editing && (
            // Dua syarat, bukan satu.
            //
            // `group-hover` di Tailwind v4 dibungkus @media (hover: hover), dan
            // perangkat tanpa penunjuk — setiap ponsel — melaporkan hover: none.
            // Dengan satu syarat saja, tombol balas/edit/hapus tidak pernah
            // muncul di layar sentuh: bukan sulit ditemukan, melainkan tidak
            // bisa dijangkau sama sekali. Di sana tombolnya memang selalu
            // tampil, karena tidak ada isyarat lain untuk memunculkannya.
            <span className="hidden items-center gap-1 group-hover:flex [@media(hover:none)]:flex">
              {onReply && (
                <button onClick={() => onReply(message)} className={aksi}>
                  Balas
                </button>
              )}
              {onForward && (
                <button onClick={() => onForward(message)} className={aksi}>
                  Teruskan
                </button>
              )}
              {onTogglePin && (
                <button onClick={() => onTogglePin(message, !pinned)} className={aksi}>
                  {pinned ? 'Lepas sematan' : 'Sematkan'}
                </button>
              )}
              {mine && (
                <>
                  {/* Pesan tanpa teks tidak punya apa pun untuk diedit;
                      lampiran tidak bisa diganti setelah terkirim. */}
                  {message.body !== '' && (
                    <button
                      onClick={() => {
                        setDraft(message.body);
                        setEditing(true);
                      }}
                      className={aksi}
                    >
                      Edit
                    </button>
                  )}
                  <button onClick={() => void deleteMessage(message.id)} className={aksi}>
                    Hapus
                  </button>
                </>
              )}
            </span>
          )}
        </div>
      </div>
    </div>
  );
}

/** Tombol kecil di bawah gelembung: tanpa bidang warna sampai disentuh. */
const aksi =
  'rounded-md px-1.5 py-0.5 font-medium transition hover:bg-accent-soft hover:text-accent-text';

/**
 * Gelembung kutipan.
 *
 * Isinya datang dari server setiap kali riwayat dimuat, BUKAN disalin saat
 * pesannya dikirim — jadi yang tampil selalu keadaan terbaru: pesan yang sudah
 * diedit menampilkan versi barunya, dan yang sudah dihapus tetap ada sebagai
 * "pesan dihapus" alih-alih menghilang dan menyisakan balasan tanpa konteks.
 */
function Quote({
  reply,
  mine,
  nameOf,
  onJump,
}: {
  reply: ReplyPreview;
  mine: boolean;
  nameOf?: (userId: string) => string;
  onJump?: (messageId: string, seq?: number) => void;
}) {
  const label = reply.deleted
    ? 'Pesan ini dihapus'
    : reply.body || kindLabel[reply.kind ?? 'file'] || 'Lampiran';

  return (
    <button
      type="button"
      onClick={() => onJump?.(reply.id, reply.seq)}
      disabled={!onJump}
      className={`mb-2 flex w-full flex-col items-start gap-0.5 rounded-lg border-l-[3px] px-2.5 py-1.5 text-left text-[13px] transition ${
        mine
          ? 'border-accent-ink/70 bg-accent-ink/15 hover:bg-accent-ink/25'
          : 'border-accent bg-accent-soft/70 hover:bg-accent-soft'
      } ${onJump ? 'cursor-pointer' : ''}`}
    >
      <span className="font-bold opacity-90">{nameOf?.(reply.senderId) ?? 'Seseorang'}</span>
      <span className={`line-clamp-2 opacity-80 ${reply.deleted ? 'italic' : ''}`}>{label}</span>
    </button>
  );
}

/**
 * Menyorot sebutan di dalam teks.
 *
 * Yang disorot adalah nama yang memang ada di percakapan ini, dan yang menyebut
 * PEMBACA disorot lebih tegas. Perhatikan bahwa penyorotan ini murni tampilan:
 * siapa yang benar-benar dipanggil sudah ditentukan server dari daftar id,
 * bukan dari teks ini. Kalau keduanya sampai berbeda — misalnya seseorang
 * mengetik "@Budi" tanpa memilihnya dari daftar — yang terjadi adalah tulisan
 * yang tidak tersorot, bukan orang yang salah dibangunkan.
 */
function Highlighted({ text, names }: { text: string; names: { name: string; isMe: boolean }[] }) {
  if (names.length === 0) return <>{text}</>;

  // Nama terpanjang lebih dulu: "Budi Santoso" harus menang atas "Budi",
  // kalau tidak sisanya tertinggal sebagai teks biasa di tengah sorotan.
  const sorted = [...names].sort((a, b) => b.name.length - a.name.length);
  const pattern = new RegExp(`@(${sorted.map(n => escapeRegExp(n.name)).join('|')})`, 'g');

  const parts: (string | { name: string; isMe: boolean })[] = [];
  let last = 0;
  for (const match of text.matchAll(pattern)) {
    const at = match.index;
    if (at > last) parts.push(text.slice(last, at));
    const found = sorted.find(n => n.name === match[1]);
    parts.push(found ?? match[0]);
    last = at + match[0].length;
  }
  if (last < text.length) parts.push(text.slice(last));

  return (
    <>
      {parts.map((part, i) =>
        typeof part === 'string' ? (
          part
        ) : (
          <span
            key={i}
            className={`rounded px-1 font-bold ${
              part.isMe ? 'bg-call text-call-ink' : 'bg-current/15'
            }`}
          >
            @{part.name}
          </span>
        ),
      )}
    </>
  );
}

function escapeRegExp(s: string) {
  return s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

/**
 * Menyusun kalimat untuk sebuah catatan sistem.
 *
 * Kalimatnya dirakit DI SINI, bukan disimpan di server. Yang tersimpan cuma
 * kejadiannya — siapa melakukan apa kepada siapa — sehingga bahasanya bisa
 * berubah, atau diterjemahkan, tanpa menulis ulang riwayat siapa pun.
 *
 * Namanya diambil dari catatan itu sendiri, bukan dari daftar anggota: orang
 * yang dikeluarkan sudah tidak ada di sana, dan "Budi mengeluarkan (tidak
 * dikenal)" gagal justru pada satu hal yang ingin diketahui orang.
 */
function systemText(ev: SystemEvent): string {
  const actor = ev.actor.name;
  const targets = (ev.targets ?? []).map(t => t.name).join(', ');

  switch (ev.type) {
    case 'member.added':
      return `${actor} menambahkan ${targets}`;
    case 'member.removed':
      return `${actor} mengeluarkan ${targets}`;
    case 'member.left':
      return `${actor} keluar dari grup`;
    case 'title.changed':
      return `${actor} mengganti judul grup jadi "${ev.title}"`;
    case 'owner.changed':
      return `${actor} menjadikan ${targets} pemilik grup`;
    case 'message.pinned':
      return `${actor} menyematkan sebuah pesan`;
    case 'message.unpinned':
      return `${actor} melepas sematan sebuah pesan`;
    default:
      return 'Grup diperbarui';
  }
}
