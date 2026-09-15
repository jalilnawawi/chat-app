// Bentuk data ini harus sama persis dengan JSON dari server Go.
// Lihat server/internal/store/models.go dan server/internal/hub/hub.go.

export type User = {
  id: string;
  username: string;
  displayName: string;
  createdAt: string;
};

export type Message = {
  id: string;
  conversationId: string;
  seq: number;
  senderId: string;
  body: string;
  createdAt: string;
  editedAt: string | null;
  deletedAt: string | null;
};

export type Member = {
  userId: string;
  username: string;
  displayName: string;
  role: 'owner' | 'member';
  lastReadSeq: number;
};

export type Conversation = {
  id: string;
  type: 'direct' | 'group';
  title: string | null;
  lastSeq: number;
  lastReadSeq: number;
  unread: number;
  peer: User | null;
  lastMessage: Message | null;
  updatedAt: string;
};

/** Pesan yang sudah tampil di layar tapi belum dikonfirmasi server. */
export type PendingMessage = {
  id: string;
  conversationId: string;
  body: string;
  createdAt: string;
  status: 'sending' | 'failed';
};

/** Event yang datang dari server lewat WebSocket. */
export type ServerEvent =
  | { type: 'message.new'; payload: Message }
  | { type: 'message.updated'; payload: Message }
  | { type: 'conversation.new'; payload: Conversation }
  | { type: 'read.updated'; payload: { conversationId: string; userId: string; lastReadSeq: number } }
  | { type: 'typing'; payload: { conversationId: string; userId: string; displayName: string; typing: boolean } }
  | { type: 'presence'; payload: { userId: string; online: boolean } }
  | { type: 'presence.snapshot'; payload: { online: string[] } }
  /**
   * Pesan susulan setelah reconnect, dikirim berkelompok dalam satu frame.
   * Satu frame per pesan akan meluberkan antrean kirim koneksi saat banyak
   * orang menyusul bersamaan — lihat catatan di server/internal/ws/handler.go.
   */
  | { type: 'sync.batch'; payload: { conversationId: string; messages: Message[] } }
  | { type: 'sync.complete'; payload: Record<string, never> }
  /**
   * Server pamit terencana (rolling deploy). Bedanya dengan koneksi yang putus
   * begitu saja: instance pengganti SUDAH siap, jadi client boleh menyambung
   * lagi hampir seketika alih-alih mundur bertahap seperti menghadapi gangguan.
   */
  | { type: 'server.shutdown'; payload: { reason: string } }
  | { type: 'error'; payload: { message: string } };
