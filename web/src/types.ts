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
  | { type: 'sync.complete'; payload: Record<string, never> }
  | { type: 'error'; payload: { message: string } };
