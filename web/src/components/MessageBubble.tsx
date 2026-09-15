import { useState } from 'react';
import { useStore } from '../store';
import AttachmentList from './AttachmentList';
import type { Message, PendingMessage } from '../types';

type Props = {
  message?: Message;
  pending?: PendingMessage;
  mine: boolean;
  showAuthor: boolean;
  authorName: string;
  readByPeer?: boolean;
};

const time = (iso: string) =>
  new Date(iso).toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit' });

export default function MessageBubble({
  message,
  pending,
  mine,
  showAuthor,
  authorName,
  readByPeer,
}: Props) {
  const editMessage = useStore(s => s.editMessage);
  const deleteMessage = useStore(s => s.deleteMessage);
  const retryMessage = useStore(s => s.retryMessage);

  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(message?.body ?? '');

  const body = message?.body ?? pending?.body ?? '';
  const createdAt = message?.createdAt ?? pending?.createdAt ?? '';
  const deleted = Boolean(message?.deletedAt);
  const attachments = (message?.attachments ?? pending?.attachments ?? []).filter(() => !deleted);

  async function saveEdit() {
    if (!message) return;
    const next = draft.trim();
    setEditing(false);
    if (next && next !== message.body) await editMessage(message.id, next);
  }

  return (
    <div className={`group mb-2 flex ${mine ? 'justify-end' : 'justify-start'}`}>
      <div className={`max-w-[75%] ${mine ? 'items-end' : 'items-start'}`}>
        {showAuthor && !mine && (
          <p className="mb-0.5 px-1 text-xs font-medium text-muted">{authorName}</p>
        )}

        <div
          className={`rounded-2xl text-sm ${
            // Gelembung yang isinya cuma gambar dibuat rapat: padding tebal di
            // sekeliling foto membuatnya tampak seperti bingkai, bukan seperti
            // foto yang dikirim.
            attachments.length > 0 && body === '' && !deleted ? 'p-1.5' : 'px-3.5 py-2'
          } ${
            deleted
              ? 'border border-dashed border-line text-muted italic'
              : mine
                ? 'bg-accent text-white'
                : 'border border-line bg-surface'
          } ${pending?.status === 'failed' ? 'opacity-60 ring-1 ring-red-500' : ''}`}
        >
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
                  {deleted ? 'Pesan ini dihapus' : body}
                </p>
              )}
            </div>
          )}
        </div>

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
            >
              gagal — coba lagi
            </button>
          )}

          {mine && message && !deleted && readByPeer && <span>dibaca</span>}

          {mine && message && !deleted && !editing && (
            <span className="hidden gap-2 group-hover:flex">
              {/* Pesan tanpa teks tidak punya apa pun untuk diedit; lampiran
                  tidak bisa diganti setelah terkirim. */}
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
              <button
                onClick={() => void deleteMessage(message.id)}
                className="hover:text-ink"
              >
                hapus
              </button>
            </span>
          )}
        </div>
      </div>
    </div>
  );
}
