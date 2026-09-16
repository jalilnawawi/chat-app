import { useEffect, useRef, useState } from 'react';
import { useStore } from '../store';
import AttachmentList from './AttachmentList';
import ReactionRow from './ReactionRow';
import type { Message, PendingMessage, ReplyPreview } from '../types';

type Props = {
  message?: Message;
  pending?: PendingMessage;
  mine: boolean;
  showAuthor: boolean;
  authorName: string;
  readByPeer?: boolean;
  /** Nama yang layak disorot di dalam teks; isMe menentukan sorotannya tegas. */
  highlights?: { name: string; isMe: boolean }[];
  /** Apakah pesan ini memanggil pembaca — menyalakan pengamat "sudah terlihat". */
  callsMe?: boolean;
  nameOf?: (userId: string) => string;
  onReply?: (m: Message) => void;
  onJump?: (messageId: string) => void;
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
  readByPeer,
  highlights = [],
  callsMe = false,
  nameOf,
  onReply,
  onJump,
  onSeen,
}: Props) {
  const editMessage = useStore(s => s.editMessage);
  const deleteMessage = useStore(s => s.deleteMessage);
  const retryMessage = useStore(s => s.retryMessage);

  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(message?.body ?? '');
  const rootRef = useRef<HTMLDivElement>(null);

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

  return (
    <div
      ref={rootRef}
      id={message ? `msg-${message.id}` : undefined}
      className={`group mb-2 flex ${mine ? 'justify-end' : 'justify-start'}`}
    >
      <div className={`flex max-w-[75%] flex-col ${mine ? 'items-end' : 'items-start'}`}>
        {showAuthor && !mine && (
          <p className="mb-0.5 px-1 text-xs font-medium text-muted">{authorName}</p>
        )}

        <div
          className={`rounded-2xl text-sm ${
            // Gelembung yang isinya cuma gambar dibuat rapat: padding tebal di
            // sekeliling foto membuatnya tampak seperti bingkai, bukan seperti
            // foto yang dikirim.
            attachments.length > 0 && body === '' && !deleted && !replyTo ? 'p-1.5' : 'px-3.5 py-2'
          } ${
            deleted
              ? 'border border-dashed border-line text-muted italic'
              : mine
                ? 'bg-accent text-white'
                : 'border border-line bg-surface'
          } ${
            // Pesan yang memanggil kita diberi pinggiran, bukan warna lain:
            // warnanya sudah dipakai untuk membedakan pesan sendiri dari pesan
            // orang, dan dua arti pada satu isyarat tidak bisa dibaca sekaligus.
            callsMe && !deleted ? 'ring-2 ring-amber-400' : ''
          } ${pending?.status === 'failed' ? 'opacity-60 ring-1 ring-red-500' : ''}`}
        >
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

        {message && !deleted && (
          <ReactionRow message={message} mine={mine} />
        )}

        <div
          className={`mt-0.5 flex items-center gap-2 px-1 text-[11px] text-muted ${
            mine ? 'justify-end' : 'justify-start'
          }`}
        >
          {createdAt && <span>{time(createdAt)}</span>}
          {message?.editedAt && !deleted && <span>diedit</span>}

          {pending?.status === 'sending' && <span>mengirim…</span>}
          {pending?.status === 'failed' && (
            <button
              onClick={() => void retryMessage(pending.conversationId, pending.id)}
              className="text-red-500 underline"
              // Sebab kegagalan ikut ditampilkan: pesan yang ditolak karena
              // kuota @semua terlihat persis sama dengan yang gagal karena
              // jaringan, dan keduanya menuntut tindakan yang berbeda.
              title={pending.error}
            >
              {pending.error ?? 'gagal'} — coba lagi
            </button>
          )}

          {mine && message && !deleted && readByPeer && <span>dibaca</span>}

          {message && !deleted && !editing && (
            // Dua syarat, bukan satu.
            //
            // `group-hover` di Tailwind v4 dibungkus @media (hover: hover), dan
            // perangkat tanpa penunjuk — setiap ponsel — melaporkan hover: none.
            // Dengan satu syarat saja, tombol balas/edit/hapus tidak pernah
            // muncul di layar sentuh: bukan sulit ditemukan, melainkan tidak
            // bisa dijangkau sama sekali. Di sana tombolnya memang selalu
            // tampil, karena tidak ada isyarat lain untuk memunculkannya.
            <span className="hidden gap-2 group-hover:flex [@media(hover:none)]:flex">
              {onReply && (
                <button onClick={() => onReply(message)} className="hover:text-ink">
                  balas
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
                      className="hover:text-ink"
                    >
                      edit
                    </button>
                  )}
                  <button onClick={() => void deleteMessage(message.id)} className="hover:text-ink">
                    hapus
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
  onJump?: (messageId: string) => void;
}) {
  const label = reply.deleted
    ? 'Pesan ini dihapus'
    : reply.body || kindLabel[reply.kind ?? 'file'] || 'Lampiran';

  return (
    <button
      type="button"
      onClick={() => onJump?.(reply.id)}
      disabled={!onJump}
      className={`mb-1.5 flex w-full flex-col items-start gap-0.5 rounded-lg border-l-2 px-2 py-1 text-left text-xs transition ${
        mine
          ? 'border-white/60 bg-white/15 hover:bg-white/25'
          : 'border-accent bg-accent-soft/60 hover:bg-accent-soft'
      } ${onJump ? 'cursor-pointer' : ''}`}
    >
      <span className="font-medium opacity-90">{nameOf?.(reply.senderId) ?? 'Seseorang'}</span>
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
            className={`rounded px-0.5 font-medium ${
              part.isMe ? 'bg-amber-400/40' : 'bg-current/15'
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
