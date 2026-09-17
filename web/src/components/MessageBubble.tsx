import { useEffect, useRef, useState, type MouseEvent } from 'react';
import { useStore } from '../store';
import AttachmentList from './AttachmentList';
import Avatar from './Avatar';
import Icon from './Icon';
import MessageMenu, { MenuCornerSpacer, type MenuItem } from './MessageMenu';
import { EditForm, FailedRow, Highlighted, MetaRow, Quote, SystemNote } from './MessageParts';
import ReactionRow, { ReactionButton } from './ReactionRow';
import { excerpt } from '../format';
import { T } from '../teks';
import { useCoarsePointer } from '../useMediaQuery';
import type { Message, PendingMessage } from '../types';

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
  /** Id pembaca: catatan sistem tentang dirinya sendiri ditulis "Kamu". */
  meId?: string;
  /** Hapus sudah dijadwalkan; gelembungnya tampil sebagai dihapus sampai waktunya tiba. */
  deleting?: boolean;
  /** Menyalin teks pesan; ChatPanel yang mengabarkan hasilnya. */
  onCopy?: (m: Message) => void;
  /** Pesan gagal yang ditarik kembali ke kolom tulis. */
  onRewrite?: (p: PendingMessage) => void;
  /** Berubah setiap kali ChatPanel meminta pesan ini disunting (↑ di kolom kosong). */
  editSignal?: number;
  /** Penyuntingan selesai — disimpan atau dibatalkan. */
  onEditEnd?: () => void;
};

/**
 * Satu pesan: gelembungnya, tindakannya, reaksinya, dan baris di bawahnya.
 *
 * Yang diputuskan di sini adalah KAPAN setiap bagian tampil dan bagaimana
 * gelembung dibentuk (sudut, ruang untuk panah menu, warna). Bentuk tiap
 * bagiannya sendiri tinggal di MessageParts.
 */
export default function MessageBubble({
  message,
  pending,
  mine,
  showAuthor,
  authorName,
  authorAvatar,
  firstOfRun = true,
  gutter = false,
  readByPeer = false,
  highlights = [],
  callsMe = false,
  nameOf,
  onReply,
  onJump,
  onForward,
  onTogglePin,
  pinned = false,
  onSeen,
  meId,
  deleting = false,
  onCopy,
  onRewrite,
  editSignal = 0,
  onEditEnd,
}: Props) {
  const editMessage = useStore(s => s.editMessage);
  const scheduleDelete = useStore(s => s.scheduleDelete);
  const retryMessage = useStore(s => s.retryMessage);
  const discardPending = useStore(s => s.discardPending);
  const toggleReaction = useStore(s => s.toggleReaction);

  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(message?.body ?? '');
  const rootRef = useRef<HTMLDivElement>(null);
  const touch = useCoarsePointer();

  const isSystem = message?.kind === 'system' && Boolean(message.systemEvent);
  const deleted = Boolean(message?.deletedAt) || deleting;

  // Permintaan menyunting dari luar gelembung: ↑ di kolom tulis yang kosong.
  useEffect(() => {
    if (!editSignal || !message || message.body === '') return;
    setDraft(message.body);
    setEditing(true);
  }, [editSignal]); // eslint-disable-line react-hooks/exhaustive-deps

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

  if (isSystem) return <SystemNote message={message!} meId={meId} onJump={onJump} />;

  const body = message?.body ?? pending?.body ?? '';
  const createdAt = message?.createdAt ?? pending?.createdAt ?? '';
  const failed = pending?.status === 'failed';
  const attachments = deleted ? [] : (message?.attachments ?? pending?.attachments ?? []);
  const replyTo = message?.replyTo ?? pending?.replyTo;

  async function saveEdit() {
    if (!message) return;
    const next = draft.trim();
    if (!next) return;
    setEditing(false);
    onEditEnd?.();
    if (next !== message.body) await editMessage(message.id, next);
  }

  const cancelEdit = () => {
    setEditing(false);
    onEditEnd?.();
  };

  const menuItems: MenuItem[] = [];
  if (message && !deleted && !editing) {
    if (onReply) menuItems.push({ key: 'balas', label: 'Balas', icon: 'balas', onSelect: () => onReply(message) });
    if (onCopy && message.body !== '')
      menuItems.push({ key: 'salin', label: 'Salin teks', icon: 'salin', onSelect: () => onCopy(message) });
    if (onForward)
      menuItems.push({ key: 'teruskan', label: 'Teruskan', icon: 'teruskan', onSelect: () => onForward(message) });
    if (onTogglePin)
      menuItems.push({
        key: 'sematan',
        label: pinned ? 'Lepas sematan' : 'Sematkan',
        icon: 'sematan',
        onSelect: () => onTogglePin(message, !pinned),
      });
    // Pesan tanpa teks tidak punya apa pun untuk diedit; lampiran tidak bisa
    // diganti setelah terkirim.
    if (mine && message.body !== '')
      menuItems.push({
        key: 'edit',
        label: 'Edit',
        icon: 'tulis',
        onSelect: () => {
          setDraft(message.body);
          setEditing(true);
        },
      });
    if (mine)
      menuItems.push({
        key: 'hapus',
        label: 'Hapus untuk semua',
        icon: 'hapus',
        danger: true,
        onSelect: () => scheduleDelete(message),
      });
  }
  const hasMenu = menuItems.length > 0;

  /**
   * Sudut gelembung.
   *
   * Tiga sudut selalu bulat penuh; yang keempat — di sisi pengirimnya — selalu
   * rapat. Itu yang membuat sederet pesan dari satu orang terbaca sebagai satu
   * blok dengan tulang punggung lurus. Sudut atas ikut dirapatkan saat pesannya
   * BUKAN pembuka rentetan, sehingga sambungannya rata.
   */
  const sudut = mine
    ? `rounded-bubble rounded-br-tail ${firstOfRun ? '' : 'rounded-tr-tail'}`
    : `rounded-bubble rounded-bl-tail ${firstOfRun ? '' : 'rounded-tl-tail'}`;

  // Gelembung yang isinya hanya lampiran dibuat rapat: padding tebal di
  // sekeliling foto membuatnya tampak seperti bingkai.
  const attachmentOnly = attachments.length > 0 && body === '' && !replyTo && !message?.forwarded;
  // Foto saja: panah menu boleh berdiri di atas fotonya. Pemutar suara dan
  // kartu berkas punya tombol dan nama di tepi kanannya, jadi untuk mereka
  // sisi kanan gelembung dipesan selebar panah.
  const imageOnly = attachmentOnly && attachments.every(a => a.mime.startsWith('image/'));
  const reserveRight = hasMenu && attachmentOnly && !imageOnly;
  // Teks adalah baris pertama gelembung: ruang sudut panah ditaruh di dalam
  // paragrafnya, supaya ikut dihitung saat gelembung menentukan lebarnya.
  const textFirst = hasMenu && body !== '' && !message?.forwarded && !replyTo && attachments.length === 0;
  // Di gelembung sendiri tidak ada yang memanggil pembacanya: "@semua" yang
  // ditulis sendiri tidak boleh menyala mangga — mangga hanya untuk panggilan.
  const names = mine ? highlights.map(h => ({ ...h, isMe: false })) : highlights;

  const who = mine ? T.youLower : `dari ${authorName || T.someone}`;
  const summary = message ? excerpt(message) : '';

  // Klik kanan di desktop dan tekan lama di Android membuka menu yang sama.
  // Kecuali saat ada teks yang sedang dipilih: di sana menu bawaan browser —
  // salin, cari — justru yang dicari orang.
  const onContextMenu = (e: MouseEvent<HTMLDivElement>) => {
    if (!hasMenu || window.getSelection()?.toString()) return;
    e.preventDefault();
    e.currentTarget.querySelector<HTMLButtonElement>('[data-menu-trigger]')?.click();
  };

  return (
    <div
      ref={rootRef}
      id={message ? `msg-${message.id}` : undefined}
      className={`group flex ${firstOfRun ? 'mt-3' : 'mt-0.5'} ${mine ? 'justify-end' : 'justify-start'}`}
    >
      {/* Foto pengirim hanya di pembuka rentetan; sisanya dapat ruang kosong
          selebar foto itu, supaya seluruh rentetan berdiri di garis yang sama.
          Disejajarkan ke ATAS: yang di bawah blok ini adalah baris jam, dan
          foto di sebelahnya terbaca seperti milik baris itu. */}
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
            menghadap ke tengah layar. */}
        <div className={`flex max-w-full items-center gap-1 ${mine ? 'flex-row-reverse' : ''}`}>
          <div
            onContextMenu={onContextMenu}
            className={`sasaran-fokus group/bubble relative min-w-0 text-[15px] leading-[1.45] ${sudut} ${
              // Di dalam gelembung teal, cincin fokus teal tidak terlihat —
              // lihat .gelembung-sendiri di index.css.
              mine && !deleted ? 'gelembung-sendiri' : ''
            } ${attachmentOnly ? `p-1.5 ${reserveRight ? 'pr-9' : ''}` : 'px-3.5 py-2.5'} ${
              deleted
                ? 'border border-dashed border-line-strong text-muted italic'
                : mine
                  ? 'bg-accent text-accent-ink'
                  : 'border border-line-bubble bg-surface'
            } ${
              // Pesan yang memanggil kita diberi PINGGIRAN mangga, bukan bidang:
              // latar sudah dipakai untuk membedakan pesan sendiri dari pesan
              // orang, dan dua arti pada satu isyarat tidak bisa dibaca sekaligus.
              callsMe && !deleted ? 'ring-2 ring-call' : ''
            } ${
              // Cincin bahaya dengan jarak dari gelembungnya: menempel langsung
              // di tepi teal, merah dan teal nyaris tidak bisa dibedakan.
              failed ? 'ring-2 ring-danger ring-offset-2 ring-offset-canvas' : ''
            }`}
          >
            {hasMenu && !attachmentOnly && !textFirst && <MenuCornerSpacer />}

            {/* Penanda terusan: siapa penulis aslinya tidak ikut dibawa, tapi
                kalimat yang diteruskan tanpa penanda terbaca sebagai kalimat
                pengirimnya sendiri. */}
            {message?.forwarded && !deleted && (
              <p
                className={`mb-1 flex items-center gap-1 text-[13px] font-semibold ${
                  mine ? 'text-accent-ink' : 'text-muted'
                }`}
              >
                <Icon name="teruskan" size={13} />
                Diteruskan
              </p>
            )}

            {replyTo && !deleted && (
              // flow-root: kotak ini menyempit di samping ruang sudut panah,
              // bukan turun ke bawahnya dan meninggalkan pita kosong.
              <div className="flow-root">
                <Quote reply={replyTo} mine={mine} nameOf={nameOf} onJump={onJump} />
              </div>
            )}

            {editing && message ? (
              <EditForm
                id={message.id}
                draft={draft}
                touch={touch}
                onChange={setDraft}
                onSave={() => void saveEdit()}
                onCancel={cancelEdit}
              />
            ) : (
              // Blok biasa, bukan flex: baris teks harus bisa membungkus di
              // samping ruang sudut panah.
              <div className="space-y-1.5">
                <AttachmentList
                  attachments={attachments}
                  mine={mine}
                  imageAlt={mine ? 'Gambar yang kamu kirim' : `Gambar dari ${authorName || T.someone}`}
                />
                {/* Pesan boleh hanya berisi lampiran — mengirim foto tanpa
                    keterangan adalah hal yang paling biasa dilakukan orang. */}
                {(body !== '' || deleted) && (
                  <p className="break-words whitespace-pre-wrap">
                    {textFirst && <MenuCornerSpacer />}
                    {/* Selama jeda urungkan pesannya BELUM hilang: kalimatnya
                        mengikuti kabar di bawah ("Menghapus…"), bukan bentuk
                        lampau yang menyatakan semuanya sudah selesai. */}
                    {deleted ? (
                      message?.deletedAt ? T.deletedMessage : T.deletingMessage
                    ) : (
                      <Highlighted text={body} names={names} mine={mine} />
                    )}
                  </p>
                )}
              </div>
            )}

            {/* Setelah isinya di DOM, walau tampil di sudut atas: pembaca
                layar membacakan pesannya dulu, baru tindakannya. */}
            {hasMenu && message && (
              <MessageMenu
                items={menuItems}
                label={`Tindakan untuk pesan ${who}: ${summary.slice(0, 60)}`}
                mine={mine}
                summary={summary}
                reactions={{
                  given: message.reactions.filter(r => r.mine).map(r => r.emoji),
                  onReact: emoji => void toggleReaction(message.conversationId, message.id, emoji),
                }}
              />
            )}
          </div>

          {message && !deleted && !editing ? (
            <ReactionButton message={message} mine={mine} label={`Beri reaksi untuk pesan ${who}`} />
          ) : (
            // Ruangnya tetap dipesan: pesan yang baru terkonfirmasi tidak boleh
            // menyempit dan membungkus ulang kalimatnya di depan mata.
            pending && <span aria-hidden className="w-9 shrink-0 [@media(hover:none)]:hidden" />
          )}
        </div>

        {failed && pending && (
          <FailedRow
            pending={pending}
            onRewrite={() =>
              onRewrite ? onRewrite(pending) : discardPending(pending.conversationId, pending.id)
            }
            onRetry={() => void retryMessage(pending.conversationId, pending.id)}
          />
        )}

        {message && !deleted && <ReactionRow message={message} mine={mine} />}

        {/* Pesan yang sudah dihapus tidak membawa baris jam: dia bukan
            kejadian yang perlu dicari waktunya, dan jamnya tetap dibacakan
            lewat label barisnya. */}
        {!message?.deletedAt && (
          <MetaRow
            mine={mine}
            createdAt={createdAt}
            pinned={pinned && !deleted}
            edited={Boolean(message?.editedAt) && !deleted}
            sending={pending?.status === 'sending'}
            delivered={Boolean(message) && !deleted}
            readByPeer={readByPeer}
          />
        )}
      </div>
    </div>
  );
}
