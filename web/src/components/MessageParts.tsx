import { useLayoutEffect, useRef } from 'react';
import Icon from './Icon';
import PillButton from './ui/PillButton';
import { jam, replyKindText } from '../format';
import { T, systemText } from '../teks';
import type { Message, PendingMessage, ReplyPreview } from '../types';

/**
 * Bagian-bagian gelembung pesan yang punya bentuk dan aturannya sendiri.
 *
 * MessageBubble memutuskan KAPAN bagian ini tampil; berkas ini memutuskan
 * seperti apa. Semua teks yang duduk di atas gelembung teal milik sendiri
 * memakai putih PENUH — putih di atas teal hanya 4,6:1, jadi transparansi
 * sekecil apa pun menjatuhkannya di bawah AA. Pembeda di sana adalah bobot,
 * ukuran, dan bidang `accent-deep`, bukan opacity.
 */

/**
 * Catatan sistem.
 *
 * Bukan ucapan siapa pun, jadi dia tidak berbentuk gelembung: tidak berpihak
 * kiri atau kanan, tidak punya tombol. Catatan sematan menunjuk sebuah pesan,
 * dan menekannya membawa ke sana — satu-satunya catatan sistem yang punya
 * tujuan; bentuknya tetap baris tengah.
 */
export function SystemNote({
  message,
  meId,
  onJump,
}: {
  message: Message;
  meId?: string;
  onJump?: (messageId: string, seq?: number) => void;
}) {
  const ev = message.systemEvent!;
  const target = ev.type === 'message.pinned' && ev.messageId && onJump ? ev : null;
  // Melepas sematan tidak menunjuk ke mana pun dan bukan kejadian yang perlu
  // dicari lagi: kalimatnya ditulis tanpa pil, supaya tidak berbobot sama
  // dengan catatan yang punya tujuan.
  const quiet = ev.type === 'message.unpinned';
  const cls = quiet
    ? 'sasaran-fokus px-3 py-1 text-center text-[13px] text-muted'
    : 'sasaran-fokus min-h-8 rounded-full border border-line bg-surface px-3 py-1.5 text-center text-[13px] text-muted';
  return (
    <div className={`flex justify-center px-6 ${quiet ? 'my-1.5' : 'my-3'}`}>
      {target ? (
        <button
          type="button"
          onClick={() => onJump?.(target.messageId!, target.messageSeq)}
          className={`${cls} inline-flex items-center gap-1.5 transition hover:border-accent hover:text-accent-text`}
        >
          <Icon name="sematan" size={13} />
          {systemText(ev, meId)}
        </button>
      ) : (
        <p className={cls}>{systemText(ev, meId)}</p>
      )}
    </div>
  );
}

/**
 * Kutipan balasan.
 *
 * Isinya datang dari server setiap kali riwayat dimuat, BUKAN disalin saat
 * pesannya dikirim — jadi yang tampil selalu keadaan terbaru: pesan yang sudah
 * diedit menampilkan versi barunya, dan yang sudah dihapus tetap ada sebagai
 * "pesan dihapus" alih-alih menghilang.
 */
export function Quote({
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
  const label = reply.deleted ? T.deletedMessage : reply.body || replyKindText(reply.kind);
  return (
    <button
      type="button"
      onClick={() => onJump?.(reply.id, reply.seq)}
      disabled={!onJump}
      className={`mb-2 flex w-full flex-col items-start gap-0.5 rounded-lg border-l-[3px] px-2.5 py-1.5 text-left text-[13px] transition ${
        mine
          ? 'border-accent-ink bg-accent-deep text-accent-ink hover:brightness-110'
          : 'border-accent bg-accent-soft text-ink hover:brightness-[0.97]'
      } ${onJump ? 'cursor-pointer' : ''}`}
    >
      <span className="font-bold">
        {nameOf?.(reply.senderId) ?? T.someone}
      </span>
      <span className={`line-clamp-2 ${reply.deleted ? 'italic' : ''}`}>{label}</span>
    </button>
  );
}

/**
 * Menyorot sebutan di dalam teks.
 *
 * Yang disorot adalah nama yang memang ada di percakapan ini, dan yang menyebut
 * PEMBACA disorot mangga. Penyorotan ini murni tampilan: siapa yang benar-benar
 * dipanggil sudah ditentukan server dari daftar id, bukan dari teks ini.
 */
export function Highlighted({
  text,
  names,
  mine,
}: {
  text: string;
  names: { name: string; isMe: boolean }[];
  mine: boolean;
}) {
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
    parts.push(sorted.find(n => n.name === match[1]) ?? match[0]);
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
              part.isMe ? 'bg-call text-call-ink' : mine ? 'bg-accent-deep' : 'bg-ink/8'
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
 * Form sunting di dalam gelembung sendiri.
 *
 * Menyimpan hanya atas perintah — Enter atau tombol Simpan. Menyimpan saat
 * kolomnya kehilangan fokus membuat klik ke tempat lain mengirim kalimat yang
 * belum selesai, kepada semua orang, tanpa jalan kembali. Kolomnya tumbuh
 * mengikuti isinya sampai kira-kira enam baris.
 */
export function EditForm({
  id,
  draft,
  touch,
  onChange,
  onSave,
  onCancel,
}: {
  id: string;
  draft: string;
  touch: boolean;
  onChange: (value: string) => void;
  onSave: () => void;
  onCancel: () => void;
}) {
  const box = useRef<HTMLTextAreaElement>(null);
  useLayoutEffect(() => {
    const el = box.current;
    if (!el) return;
    el.style.height = 'auto';
    el.style.height = `${el.scrollHeight}px`;
  }, [draft]);

  return (
    <div className="flex min-w-[min(16rem,60vw)] flex-col gap-2">
      <textarea
        ref={box}
        autoFocus
        value={draft}
        rows={1}
        aria-label="Sunting pesan"
        aria-describedby={`edit-hint-${id}`}
        onChange={e => onChange(e.target.value)}
        onFocus={e => e.currentTarget.setSelectionRange(draft.length, draft.length)}
        onKeyDown={e => {
          // Di layar sentuh Enter membuat baris baru, sama seperti kolom
          // tulis; yang menyimpan adalah tombolnya.
          if (e.key === 'Enter' && !e.shiftKey && !touch) {
            e.preventDefault();
            onSave();
          }
          if (e.key === 'Escape') {
            e.stopPropagation();
            onCancel();
          }
        }}
        className="cincin-sendiri block max-h-[9.5rem] w-full resize-none overflow-y-auto rounded-lg bg-accent-deep px-2 py-1 text-inherit outline-none focus-visible:ring-2 focus-visible:ring-accent-ink"
      />
      <div className="flex items-center justify-end gap-2">
        <span id={`edit-hint-${id}`} className="mr-auto text-[13px]">
          {touch ? 'Ketuk Simpan untuk menyimpan' : 'Enter simpan · Esc batal'}
        </span>
        <PillButton tone="onTealGhost" size="sm" onClick={onCancel}>
          Batal
        </PillButton>
        <PillButton tone="onTealSolid" size="sm" onClick={onSave} disabled={!draft.trim()}>
          Simpan
        </PillButton>
      </div>
    </div>
  );
}

/**
 * Baris pesan yang tidak terkirim: sebab, dan dua jalan keluar yang cukup
 * besar untuk ditekan. "Tulis ulang", bukan "buang" — teksnya kembali ke kolom
 * tulis, jadi tidak ada yang hilang dan tidak perlu tombol urungkan.
 */
export function FailedRow({
  pending,
  onRewrite,
  onRetry,
}: {
  pending: PendingMessage;
  onRewrite: () => void;
  onRetry: () => void;
}) {
  return (
    <div role="alert" className="mt-2 flex max-w-full flex-wrap items-center justify-end gap-x-2 gap-y-1.5">
      <p className="flex min-w-0 items-center gap-1.5 text-[13px] font-semibold text-danger">
        <Icon name="peringatan" size={16} />
        <span>{T.notSent(pending.error)}</span>
      </p>
      <span className="flex items-center gap-1.5">
        <PillButton tone="ghost" onClick={onRewrite} className="text-[14px]">
          {T.rewrite}
        </PillButton>
        <PillButton tone="danger" icon="ulang" iconSize={17} onClick={onRetry} className="text-[14px]">
          {T.resend}
        </PillButton>
      </span>
    </div>
  );
}

/**
 * Baris di bawah gelembung: sematan, jam, "diedit", status kirim, centang.
 *
 * Centang, bukan kata: baris ini sudah penuh angka, dan bentuk yang tidak
 * perlu dibaca lebih cepat sampai. Satu centang berarti sudah sampai di server
 * — satu-satunya kabar yang ada di grup — dan dua centang berarti sudah
 * dibaca, khusus DM. Kata lengkapnya tetap ada untuk pembaca layar.
 */
export function MetaRow({
  mine,
  createdAt,
  pinned,
  edited,
  sending,
  delivered,
  readByPeer,
}: {
  mine: boolean;
  createdAt: string;
  pinned: boolean;
  edited: boolean;
  sending: boolean;
  delivered: boolean;
  readByPeer: boolean;
}) {
  return (
    <div
      className={`mt-1 flex items-center gap-2 px-1 text-[11.5px] text-muted ${
        mine ? 'justify-end' : 'justify-start'
      }`}
    >
      {pinned && (
        <span title="Disematkan" className="text-accent-text">
          <Icon name="sematan" size={13} />
          <span className="sr-only">Disematkan</span>
        </span>
      )}
      {createdAt && <span className="tabular-nums">{jam(createdAt)}</span>}
      {edited && (
        <span className="flex items-center gap-0.5">
          <Icon name="tulis-ulang" size={12} />
          diedit
        </span>
      )}
      {sending && (
        <span className="flex items-center gap-1">
          <Icon name="ulang" size={12} />
          mengirim…
        </span>
      )}
      {mine &&
        delivered &&
        (readByPeer ? (
          <span title="Sudah dibaca" className="text-accent-text">
            <Icon name="terbaca" size={15} />
            <span className="sr-only">Sudah dibaca</span>
          </span>
        ) : (
          <span title="Terkirim">
            <Icon name="terkirim" size={14} />
            <span className="sr-only">Terkirim</span>
          </span>
        ))}
    </div>
  );
}
