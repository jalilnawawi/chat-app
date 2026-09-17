import { useStore } from '../store';
import Avatar, { dotFor, statusLabel } from './Avatar';
import IconButton from './ui/IconButton';
import { conversationTitle } from './Sidebar';
import type { Conversation } from '../types';

/** Kepala percakapan: siapa, keadaannya, cari, dan kelola grup. */
export default function ChatHeader({
  conversation,
  memberCount,
  panelOpen,
  onTogglePanel,
  onSearch,
}: {
  conversation: Conversation;
  memberCount: number;
  panelOpen: boolean;
  onTogglePanel: () => void;
  onSearch: () => void;
}) {
  const closeConversation = useStore(s => s.closeConversation);
  const online = useStore(s => s.online);
  const statuses = useStore(s => s.statuses);

  const peerOnline = conversation.peer ? online.has(conversation.peer.id) : false;
  // Status dibaca dari peta siaran, bukan dari salinan di dalam `peer`: yang
  // kedua membeku pada saat daftar percakapan diambil.
  const peerStatus = conversation.peer ? statuses[conversation.peer.id] : undefined;
  const title = conversationTitle(conversation);

  return (
    <header className="flex items-center gap-2 border-b border-line bg-surface px-2 py-2.5 md:px-4 md:py-3">
      {/* Hanya ada di layar sempit, tempat daftar percakapan benar-benar pergi
          saat sebuah percakapan dibuka. Di layar lebar keduanya bersebelahan,
          dan tombol kembali tidak mengembalikan apa pun. */}
      <IconButton
        icon="kembali"
        label="Kembali ke daftar percakapan"
        className="md:hidden"
        onClick={closeConversation}
      />

      <Avatar
        name={title}
        url={conversation.peer?.avatarUrl}
        size={40}
        grup={conversation.type === 'group'}
        dot={conversation.type === 'direct' ? dotFor(peerOnline, peerStatus?.status) : undefined}
      />
      <div className="min-w-0 flex-1">
        <h2 className="truncate text-[15px] font-bold">{title}</h2>
        <p className="truncate text-[13px] text-muted">
          {conversation.type === 'group'
            ? `${memberCount} anggota`
            : // Status yang dipasang orangnya menggantikan Online/Offline.
              // Yang pertama dinyatakan dengan sengaja; yang kedua cuma kabar
              // tentang apakah tabnya kebetulan terbuka — dan "Online" di
              // sebelah orang yang baru saja menulis "sedang rapat" adalah
              // undangan untuk mengganggunya.
              statusLabel(peerStatus?.status, peerStatus?.text, peerStatus?.expiresAt) ||
              (peerOnline ? 'Online' : 'Offline')}
        </p>
      </div>

      <IconButton icon="cari" label="Cari di percakapan ini" onClick={onSearch} />

      {/* Hanya grup yang punya pengelolaan. DM tidak bisa ditambahi orang —
          lihat catatan kebocoran di server/internal/store/group.go — jadi
          tombolnya memang tidak ada di sana, bukan ada tapi menolak. */}
      {conversation.type === 'group' && (
        <IconButton icon="anggota" label="Kelola grup" pressed={panelOpen} onClick={onTogglePanel} />
      )}
    </header>
  );
}
