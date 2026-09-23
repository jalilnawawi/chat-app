import {
  useCallback,
  useEffect,
  useImperativeHandle,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  type ClipboardEvent,
  type KeyboardEvent,
  type Ref,
} from 'react';
import { useStore } from '../store';
import EmojiPicker from './EmojiPicker';
import Icon from './Icon';
import MentionList, {
  MENTION_LIST_ID,
  initialMentionIndex,
  mentionCandidates,
  mentionOptionId,
  type MentionCandidate,
} from './MentionList';
import { excerpt } from '../format';
import { T } from '../teks';
import RecordingBar from './RecordingBar';
import IconButton from './ui/IconButton';
import PillButton from './ui/PillButton';
import UploadStrip from './UploadStrip';
import { formatDuration } from '../format';
import { canRecord, MAX_RECORDING_MS, useRecorder, type Recording } from '../useRecorder';
import { useCoarsePointer, useMediaQuery } from '../useMediaQuery';
import type { Member, Upload } from '../types';

/** Satu array kosong yang dipakai bersama — lihat catatan di UploadStrip. */
const KOSONG: Upload[] = [];

export type ComposerHandle = {
  focus: () => void;
  /** Menaruh teks di depan tulisan yang sedang disusun (pesan gagal yang ditarik kembali). */
  restoreDraft: (text: string) => void;
};

/**
 * Semua yang menyusun pesan berikutnya: laci lampiran, kutipan balasan,
 * daftar sebutan, papan emoji, perekam suara, dan kolom tulisnya sendiri.
 *
 * ChatPanel hanya berbicara dengannya lewat `ComposerHandle` dan satu
 * panggilan balik — `onEditLast` untuk ↑ di kolom kosong, karena riwayat dan
 * pesan mana yang boleh disunting adalah urusan ChatPanel.
 */
export default function Composer({
  ref,
  conversationId,
  isGroup,
  roster,
  meId,
  send,
  onEditLast,
  onLeaveToHistory,
}: {
  ref?: Ref<ComposerHandle>;
  conversationId: string;
  isGroup: boolean;
  roster: Member[];
  meId: string;
  send: (type: string, payload: unknown) => void;
  /** true bila ada pesan yang mulai disunting. */
  onEditLast: () => boolean;
  /** Esc di kolom kosong: pindah ke riwayat. true bila fokus berhasil dipindah. */
  onLeaveToHistory: () => boolean;
}) {
  // Server yang belum menjawab dianggap MATI: lebih baik tombolnya muncul
  // sedikit terlambat daripada muncul lalu ditarik kembali.
  const attachments = useStore(s => s.config?.attachments ?? false);
  const drafts = useStore(s => s.uploads[conversationId] ?? KOSONG);
  const replying = useStore(s => s.replyTo[conversationId] ?? null);
  const setReplyTo = useStore(s => s.setReplyTo);
  const sendMessage = useStore(s => s.sendMessage);
  const addFiles = useStore(s => s.addFiles);
  const noteMention = useStore(s => s.noteMention);
  const setMentionAll = useStore(s => s.setMentionAll);

  const [draft, setDraft] = useState('');
  const [mentionQuery, setMentionQuery] = useState<{ at: number; text: string } | null>(null);
  const [mentionIndex, setMentionIndex] = useState(-1);
  const [emojiOpen, setEmojiOpen] = useState(false);
  // Dibuka lewat papan ketik: fokus ikut pindah ke papan emoji. Lewat jari
  // tidak — fokus di sana tidak berguna dan hanya memunculkan cincin.
  const [emojiByKeyboard, setEmojiByKeyboard] = useState(false);
  const [focused, setFocused] = useState(false);
  // Kunci unggahan rekaman yang langsung dikirim begitu selesai terunggah.
  const [sendWhenReady, setSendWhenReady] = useState<string | null>(null);

  const touch = useCoarsePointer();
  // Petunjuk papan ketik hanya di layar lebar bertetikus; di tempat lain dia
  // terpotong, atau mengajarkan tombol yang tidak ada.
  const wide = useMediaQuery('(min-width: 64rem)') && !touch;

  const textarea = useRef<HTMLTextAreaElement>(null);
  const fileInput = useRef<HTMLInputElement>(null);
  const emojiBox = useRef<HTMLDivElement>(null);
  const emojiButton = useRef<HTMLButtonElement>(null);
  const typingSentAt = useRef(0);

  // Kolom tulis tumbuh mengikuti isinya — termasuk baris yang membungkus —
  // sampai batas `max-h`, lalu menggulir. Kolom yang tetap satu baris
  // menyembunyikan awal pesan panjang tepat saat orang sedang menulisnya.
  useLayoutEffect(() => {
    const el = textarea.current;
    if (!el) return;
    el.style.height = 'auto';
    el.style.height = `${el.scrollHeight}px`;
  }, [draft]);

  const uploading = drafts.some(u => u.status === 'uploading');
  const attachable = drafts.some(u => u.status === 'ready');

  useImperativeHandle(ref, () => ({
    focus: () => textarea.current?.focus(),
    restoreDraft: text => {
      setDraft(d => (d.trim() ? `${text}\n${d}` : text));
      requestAnimationFrame(() => textarea.current?.focus());
    },
  }));

  // ---- perekam ----

  // Dihitung sekali: kemampuan merekam tidak berubah selama halaman terbuka.
  const recordable = useMemo(() => attachments && canRecord(), [attachments]);

  /**
   * Untuk percakapan mana rekaman ini dibuat, dan apakah dia langsung dikirim.
   *
   * Dicatat saat tombol rekam ditekan, bukan dibaca saat rekamannya selesai:
   * perekam menyerahkan berkasnya SETELAH potongan terakhirnya tiba, dan pada
   * saat itu layar bisa saja sudah berpindah ke percakapan lain.
   */
  const recordingFor = useRef<{ conversationId: string; send: boolean } | null>(null);

  const onRecorded = useCallback(
    ({ file, durationMs, peaks }: Recording) => {
      const target = recordingFor.current;
      recordingFor.current = null;
      if (!target) return;
      // Rekaman masuk laci lampiran seperti berkas lain — unggahan, kemajuan,
      // dan coba-lagi-nya sama. Kalau unggahannya gagal, dia tetap di laci
      // dengan tombol coba lagi, dan tidak ada yang hilang.
      const [key] = addFiles(target.conversationId, [file], { durationMs, peaks });
      // Dikirim sendiri hanya kalau orangnya menekan kirim DAN masih berada di
      // percakapan yang sama. Rekaman yang terhenti karena berpindah ruang
      // menunggu di laci percakapan asalnya.
      if (key && target.send && target.conversationId === useStore.getState().activeId) {
        setSendWhenReady(key);
      }
    },
    [addFiles],
  );
  const recorder = useRecorder(onRecorded);
  const recording = recorder.phase !== 'idle';
  const stopRecording = useRef(recorder.stop);
  stopRecording.current = recorder.stop;

  useEffect(() => {
    if (!sendWhenReady) return;
    const u = drafts.find(d => d.key === sendWhenReady);
    if (!u || u.status === 'failed') {
      setSendWhenReady(null);
      return;
    }
    if (u.status !== 'ready') return;
    setSendWhenReady(null);
    // Teks yang sempat diketik selama rekamannya terunggah TIDAK ikut: dia
    // tetap di kolom tulis, menunggu dikirim dengan sengaja.
    void sendMessage(conversationId, '');
  }, [drafts, sendWhenReady, conversationId, sendMessage]);

  const startRecording = () => {
    recordingFor.current = { conversationId, send: false };
    void recorder.start();
  };

  const finishRecording = (keep: boolean) => {
    if (keep && recordingFor.current) recordingFor.current.send = true;
    if (!keep) recordingFor.current = null;
    recorder.stop(keep);
  };

  // Pindah ruang: pemilih ditutup, dan rekaman yang sedang berjalan dihentikan
  // lalu DISIMPAN di laci percakapan asalnya, tanpa dikirim. Suara yang
  // direkam untuk satu orang tidak boleh terkirim ke orang lain hanya karena
  // layarnya berganti — tapi lima menit bicara juga tidak boleh hilang tanpa
  // kabar karena satu ketukan di daftar.
  useLayoutEffect(() => {
    setMentionQuery(null);
    setEmojiOpen(false);
    if (recordingFor.current) recordingFor.current.send = false;
    stopRecording.current(true);
    setSendWhenReady(null);
  }, [conversationId]);

  // ---- papan emoji ----

  useEffect(() => {
    if (!emojiOpen) return;
    const onDown = (e: PointerEvent) => {
      const t = e.target as Node;
      if (emojiBox.current?.contains(t) || emojiButton.current?.contains(t)) return;
      setEmojiOpen(false);
    };
    const onKey = (e: globalThis.KeyboardEvent) => {
      if (e.key !== 'Escape') return;
      setEmojiOpen(false);
      // Fokus kembali ke tombolnya, bukan hilang ke badan halaman.
      if (emojiBox.current?.contains(document.activeElement)) emojiButton.current?.focus();
    };
    document.addEventListener('pointerdown', onDown);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('pointerdown', onDown);
      document.removeEventListener('keydown', onKey);
    };
  }, [emojiOpen]);

  /**
   * Menyisipkan emoji di posisi kursor, bukan di ujung teks.
   *
   * Papan emoji dibiarkan terbuka: orang jarang memilih satu saja. Di layar
   * sentuh kolom tulisnya TIDAK difokuskan kembali — fokus membuka papan
   * ketik, dan papan ketik menutupi papan emoji yang baru saja dipakai.
   */
  function insertEmoji(emoji: string) {
    const el = textarea.current;
    const start = el?.selectionStart ?? draft.length;
    const end = el?.selectionEnd ?? draft.length;
    setDraft(draft.slice(0, start) + emoji + draft.slice(end));
    notifyTyping();

    const pos = start + emoji.length;
    requestAnimationFrame(() => {
      if (!el) return;
      if (!touch && !emojiByKeyboard) el.focus();
      el.setSelectionRange(pos, pos);
    });
  }

  // ---- sebutan ----

  const candidates = useMemo(
    () => (mentionQuery ? mentionCandidates(mentionQuery.text, roster, meId, isGroup) : []),
    [mentionQuery, roster, meId, isGroup],
  );

  useEffect(() => {
    setMentionIndex(initialMentionIndex(candidates));
  }, [candidates]);

  /**
   * Mencari sebutan yang sedang diketik, dari posisi kursor ke belakang.
   *
   * Berhenti di "@" pertama yang berada di awal baris atau setelah spasi, dan
   * menyerah kalau sudah melewati satu baris penuh. Spasi TIDAK mengakhiri
   * pencarian — nama tampilan boleh mengandung spasi, dan "@Budi Santoso" harus
   * tetap bisa dipilih dari daftar.
   */
  function detectMention(value: string, caret: number) {
    const before = value.slice(0, caret);
    const at = before.lastIndexOf('@');
    if (at < 0) {
      setMentionQuery(null);
      return;
    }
    const prev = at > 0 ? before[at - 1] : ' ';
    const query = before.slice(at + 1);
    if (!/\s/.test(prev!) || query.includes('\n') || query.length > 32) {
      setMentionQuery(null);
      return;
    }
    setMentionQuery({ at, text: query });
  }

  function chooseMention(choice: MentionCandidate) {
    if (!mentionQuery) return;
    const el = textarea.current;
    const caret = el?.selectionStart ?? draft.length;
    setDraft(draft.slice(0, mentionQuery.at) + '@' + choice.name + ' ' + draft.slice(caret));
    setMentionQuery(null);

    // Yang dicatat adalah ID-nya, dan inilah yang dikirim ke server. Teksnya
    // hanya untuk dibaca manusia — server tidak pernah menguraikannya.
    if (choice.all) setMentionAll(conversationId, true);
    else noteMention(conversationId, choice.id, choice.name);

    const pos = mentionQuery.at + choice.name.length + 2;
    queueMicrotask(() => {
      el?.focus();
      el?.setSelectionRange(pos, pos);
    });
  }

  // ---- menulis dan mengirim ----

  function notifyTyping() {
    // Dikirim paling sering sekali per 2 detik; sisanya cuma menghabiskan bandwidth.
    const now = Date.now();
    if (now - typingSentAt.current < 2000) return;
    typingSentAt.current = now;
    send('typing', { conversationId, typing: true });
  }

  async function submit() {
    const body = draft.trim();
    // Pesan boleh tanpa teks asalkan ada lampiran yang sudah selesai diunggah.
    if (!body && !attachable) return;
    setDraft('');
    setMentionQuery(null);
    typingSentAt.current = 0;
    send('typing', { conversationId, typing: false });
    await sendMessage(conversationId, body);
  }

  function onKeyDown(e: KeyboardEvent<HTMLTextAreaElement>) {
    // Saat daftar sebutan terbuka, panah dan Enter miliknya — bukan milik
    // kolom tulis. Tanpa ini, Enter mengirim pesan yang masih berisi "@bud".
    if (mentionQuery && candidates.length > 0) {
      if (e.key === 'ArrowDown') {
        e.preventDefault();
        setMentionIndex(i => (i + 1) % candidates.length);
        return;
      }
      if (e.key === 'ArrowUp') {
        e.preventDefault();
        setMentionIndex(i => (i <= 0 ? candidates.length - 1 : i - 1));
        return;
      }
      if (e.key === 'Enter' || e.key === 'Tab') {
        e.preventDefault();
        // Tidak ada yang tersorot — hanya "@semua" yang cocok. Enter menutup
        // daftarnya saja; memilih "semua" butuh panah atau ketukan yang sengaja.
        if (mentionIndex < 0) setMentionQuery(null);
        else chooseMention(candidates[mentionIndex]!);
        return;
      }
      if (e.key === 'Escape') {
        e.preventDefault();
        setMentionQuery(null);
        return;
      }
    }

    // Escape membatalkan balasan — jalan keluar yang sama dengan membatalkan
    // penyuntingan, jadi tidak perlu dihafal terpisah.
    if (e.key === 'Escape' && replying) {
      e.preventDefault();
      setReplyTo(conversationId, null);
      return;
    }

    // Esc di kolom kosong: pindah ke riwayat, ke pesan terbaru. Jalan pintas
    // yang tidak bergantung pada berapa tombol berdiri di antaranya.
    if (e.key === 'Escape' && draft === '' && !replying && onLeaveToHistory()) {
      e.preventDefault();
      return;
    }

    // ↑ di kolom kosong: sunting pesan terakhir sendiri — jalan pintas yang
    // sudah dihafal orang dari aplikasi lain.
    if (e.key === 'ArrowUp' && draft === '' && !replying && onEditLast()) {
      e.preventDefault();
      return;
    }

    // Di layar sentuh Enter membuat baris baru. Papan ketik ponsel tidak punya
    // Shift+Enter, dan tanpa ini pesan beberapa baris mustahil ditulis —
    // yang mengirim di sana adalah tombolnya.
    if (e.key === 'Enter' && !e.shiftKey && !touch) {
      e.preventDefault();
      void submit();
    }
  }

  // Satu gerbang untuk KETIGA jalan masuk berkas — tombol, seret, dan tempel.
  // Seret ditangani ChatPanel, yang memanggil `addFiles` dengan syarat yang sama.
  function pick(files: FileList | File[] | null) {
    if (!attachments || !files) return;
    const daftar = Array.from(files);
    if (daftar.length > 0) addFiles(conversationId, daftar);
  }

  // Menempel gambar langsung dari papan klip — cara paling cepat mengirim
  // tangkapan layar, dan yang paling sering dicoba orang tanpa diberi tahu.
  function onPaste(e: ClipboardEvent<HTMLTextAreaElement>) {
    const files = Array.from(e.clipboardData.files);
    if (files.length === 0 || !attachments) return;
    e.preventDefault();
    pick(files);
  }

  const mentionOpen = mentionQuery !== null && candidates.length > 0;
  const inBoxButton =
    // `outline-none` wajib: `cincin-sendiri` hanya melepas cincin global, dan
    // tanpa ini browser menggambar garis bawaannya sendiri di atas lengkung pil.
    'cincin-sendiri grid size-11 shrink-0 place-items-center rounded-full outline-none transition focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-inset';

  return (
    <>
      <UploadStrip conversationId={conversationId} />

      {replying && (
        <div className="flex items-start gap-2.5 border-t border-line bg-surface px-3 py-2.5 md:px-4">
          <span className="mt-0.5 shrink-0 text-accent-text">
            <Icon name="balas" size={16} />
          </span>
          <div className="min-w-0 flex-1 border-l-[3px] border-accent pl-2.5">
            <p className="text-[13px] font-bold text-accent-text">
              {replying.senderId === meId
                ? T.replyingToSelf
                : T.replyingTo(
                    roster.find(m => m.userId === replying.senderId)?.displayName ?? T.someone,
                  )}
            </p>
            <p className="truncate text-[13px] text-muted">{excerpt(replying)}</p>
          </div>
          <IconButton
            icon="tutup"
            size="sm"
            label={T.cancelReply}
            title={`${T.cancelReply} (Esc)`}
            onClick={() => setReplyTo(conversationId, null)}
          />
        </div>
      )}

      <div className="relative border-t border-line bg-surface p-3">
        {mentionOpen && (
          <MentionList
            candidates={candidates}
            active={mentionIndex}
            onHover={setMentionIndex}
            onChoose={chooseMention}
          />
        )}

        {emojiOpen && !recording && (
          <div ref={emojiBox} className="absolute right-3 bottom-full z-20 mb-2">
            <EmojiPicker onPick={insertEmoji} autoFocus={emojiByKeyboard} />
          </div>
        )}

        {recorder.error && (
          <p
            role="alert"
            className="mb-2 flex items-center justify-between gap-2 rounded-xl bg-danger-soft px-3 py-2 text-[13px] text-danger"
          >
            {recorder.error}
            <button
              onClick={recorder.clearError}
              aria-label="Tutup pesan kesalahan"
              className="-my-1.5 grid size-10 shrink-0 place-items-center rounded-xl hover:bg-surface"
            >
              <Icon name="tutup" size={16} />
            </button>
          </p>
        )}

        {recording ? (
          <RecordingBar
            starting={recorder.phase === 'starting'}
            elapsed={recorder.elapsed}
            levels={recorder.levels}
            onCancel={() => finishRecording(false)}
            onSend={() => finishRecording(true)}
          />
        ) : (
          <div className="flex items-end gap-2">
            {/* Satu kotak berisi tiga hal: emoji di kiri, tulisan di tengah,
                lampiran di kanan. Keduanya tombol yang mengubah APA yang
                ditulis, jadi tempatnya di dalam kotak tulis — tombol di
                luarnya hanya satu, yang mengirim. */}
            <div className="flex min-w-0 flex-1 items-end rounded-[22px] border border-line-strong bg-canvas transition focus-within:border-accent focus-within:bg-surface focus-within:ring-1 focus-within:ring-accent">
              <textarea
                ref={textarea}
                value={draft}
                rows={1}
                onChange={e => {
                  setDraft(e.target.value);
                  detectMention(e.target.value, e.target.selectionStart);
                  notifyTyping();
                }}
                onClick={e => detectMention(e.currentTarget.value, e.currentTarget.selectionStart)}
                onFocus={() => setFocused(true)}
                onBlur={() => {
                  setFocused(false);
                  setMentionQuery(null);
                }}
                onPaste={onPaste}
                onKeyDown={onKeyDown}
                placeholder={T.writePlaceholder}
                aria-label="Tulis pesan"
                aria-describedby="petunjuk-kirim"
                // Daftar sebutan dikendalikan dari kolom ini: fokus tetap di
                // sini, dan pembaca layar mengikuti pilihan lewat
                // aria-activedescendant.
                aria-autocomplete="list"
                aria-controls={mentionOpen ? MENTION_LIST_ID : undefined}
                aria-activedescendant={
                  mentionOpen && mentionIndex >= 0 ? mentionOptionId(mentionIndex) : undefined
                }
                className="cincin-sendiri max-h-36 min-h-11 min-w-0 flex-1 resize-none overflow-y-auto bg-transparent py-2.5 pl-4 text-[15px] leading-6 outline-none placeholder:text-muted"
              />

              <button
                ref={emojiButton}
                onClick={e => {
                  setEmojiByKeyboard(e.detail === 0);
                  setEmojiOpen(v => !v);
                }}
                aria-label="Pilih emoji"
                aria-expanded={emojiOpen}
                title="Emoji"
                // Di KANAN kolom, bukan di kiri: urutan DOM mengikuti urutan
                // mata, dan Shift+Tab dari kolom tulis langsung sampai ke
                // riwayat, bukan berhenti di tombol emoji dulu. Cincinnya di
                // DALAM tombol: cincin luar yang bulat bertabrakan dengan
                // lengkung kotak tulis.
                className={`${inBoxButton} ${emojiOpen ? 'text-accent-text' : 'text-muted hover:text-ink'}`}
              >
                <Icon name="emoji" size={22} />
              </button>

              {/* Tombolnya hanya ada kalau server ini memang menerima lampiran —
                  aturan yang sama dengan tombol notifikasi di sidebar dan tombol
                  kelola grup: yang tidak boleh ditekan tidak ditampilkan, bukan
                  ditampilkan lalu ditolak. */}
              {attachments && (
                <>
                  <input
                    ref={fileInput}
                    type="file"
                    multiple
                    hidden
                    onChange={e => {
                      pick(e.target.files);
                      // Dikosongkan supaya memilih berkas yang SAMA dua kali
                      // berturut-turut tetap memicu change.
                      e.target.value = '';
                    }}
                  />
                  <button
                    onClick={() => fileInput.current?.click()}
                    aria-label="Lampirkan berkas"
                    title="Lampirkan berkas — bisa juga seret ke sini atau tempel gambar"
                    className={`${inBoxButton} text-muted hover:text-ink`}
                  >
                    <Icon name="klip" />
                  </button>
                </>
              )}
            </div>

            {/* Kotak kosong: tombolnya merekam. Ada yang bisa dikirim: tombolnya
                mengirim. Satu tempat, satu tombol — dua tombol bulat
                bersebelahan dengan warna yang sama akan tertukar oleh ibu jari.

                Tombol rekam bergaris, bukan berbidang: saat diam dia adalah
                benda paling mencolok di kolom tulis, dan bidang teal
                mengundang rekaman yang tidak disengaja. */}
            {recordable && !draft.trim() && !attachable && !uploading ? (
              <PillButton
                tone="outline"
                icon="mikrofon"
                onClick={startRecording}
                aria-label="Rekam pesan suara"
                title={`Rekam pesan suara — paling lama ${formatDuration(MAX_RECORDING_MS)}`}
              >
                {T.record}
              </PillButton>
            ) : (
              <PillButton
                tone="solid"
                icon="kirim"
                onClick={() => void submit()}
                // Unggahan yang belum selesai menahan tombol kirim. Melepasnya
                // lebih awal akan mengirim pesan TANPA lampiran yang jelas-jelas
                // terlihat di layar — kegagalan yang diam dan membingungkan.
                disabled={(!draft.trim() && !attachable) || uploading}
                aria-label="Kirim pesan"
                title={uploading ? 'Menunggu unggahan selesai' : touch ? 'Kirim' : 'Kirim (Enter)'}
              >
                {uploading ? T.uploading : T.send}
              </PillButton>
            )}
          </div>
        )}

        {/* Satu-satunya tempat pintasan tertulis, dan dia tetap ada selama
            kolomnya difokus — placeholder hilang begitu huruf pertama
            diketik, padahal saat itulah orang butuh tahu cara mengirimnya. */}
        <p
          id="petunjuk-kirim"
          className={
            wide && focused && !recording
              ? 'mt-1.5 truncate px-2 text-[13px] text-muted'
              : 'sr-only'
          }
        >
          {touch ? T.sendHintTouch : wide ? T.sendHint : T.sendHintSr}
        </p>
      </div>
    </>
  );
}
