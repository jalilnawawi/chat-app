# Page Dependency Trees

There are no routes, so "pages" here means the top-level screens rendered by `web/src/App.tsx`. Line counts are given because the payload budget depends on them (all files are under the ~900-line threshold, so they can be passed whole).

## App shell
Entry: `web/src/App.tsx` (199)
Dependencies:
- web/src/components/AuthPage.tsx
- web/src/components/Sidebar.tsx (277)
  - web/src/components/Avatar.tsx (152)
  - web/src/components/Icon.tsx (129)
  - web/src/components/NewChatDialog.tsx (162)
  - web/src/components/StatusMenu.tsx (122)
  - web/src/components/ui/IconButton.tsx (57)
- web/src/components/ChatPanel.tsx (775) — see below
- web/src/components/ProfilePanel.tsx (300)
- web/src/components/AkunPanel.tsx
- web/src/components/PreferensiPanel.tsx (130)
- web/src/components/SearchPanel.tsx (326)
- web/src/components/Icon.tsx
- web/src/components/Tanda.tsx (25)
Non-UI: web/src/api.ts, web/src/store.ts, web/src/tema.ts, web/src/push.ts, web/src/useSocket.ts

## Chat (main design target)
Entry: `web/src/components/ChatPanel.tsx` (775)
Dependencies:
- web/src/components/ChatHeader.tsx (75)
  - web/src/components/Avatar.tsx (152)
  - web/src/components/ui/IconButton.tsx (57)
  - web/src/components/Sidebar.tsx (277) — for `conversationTitle`
- web/src/components/Composer.tsx (611)
  - web/src/components/EmojiPicker.tsx (201)
  - web/src/components/Icon.tsx (129)
  - web/src/components/MentionList.tsx (90)
  - web/src/components/RecordingBar.tsx (133)
  - web/src/components/UploadStrip.tsx (115)
    - web/src/components/AttachmentList.tsx (152)
  - web/src/components/ui/IconButton.tsx
  - web/src/components/ui/PillButton.tsx (81)
- web/src/components/MessageBubble.tsx (406)
  - web/src/components/AttachmentList.tsx (152)
    - web/src/components/VoicePlayer.tsx (258)
      - web/src/gelombang.ts (69)
  - web/src/components/Avatar.tsx
  - web/src/components/Icon.tsx
  - web/src/components/MessageMenu.tsx (358)
    - web/src/components/EmojiPicker.tsx (201)
    - web/src/components/QuickReactions.tsx (62)
  - web/src/components/MessageParts.tsx (326)
    - web/src/components/ui/PillButton.tsx
  - web/src/components/ReactionRow.tsx (184)
    - web/src/components/EmojiPicker.tsx
    - web/src/components/QuickReactions.tsx
- web/src/components/ConversationStates.tsx (174)
  - web/src/components/Avatar.tsx
  - web/src/components/Tanda.tsx (25)
- web/src/components/NoticeStack.tsx (105)
  - web/src/components/DeleteNotice.tsx (129)
- web/src/components/ForwardDialog.tsx (209)
- web/src/components/GroupPanel.tsx (241)
  - web/src/components/ui/PanelShell.tsx (154)
- web/src/components/PinBar.tsx (184)
- web/src/components/Icon.tsx
- web/src/components/ui/IconButton.tsx
- web/src/components/ui/PillButton.tsx
Hooks/helpers (non-visual, skip as context unless needed): web/src/store.ts (1697), web/src/chatHistory.ts, web/src/useHistoryScroll.ts, web/src/useRovingLog.ts, web/src/useMediaQuery.ts, web/src/format.ts, web/src/teks.ts, web/src/types.ts
Styling: web/src/index.css (318) — pass whole, it is under the threshold.

## Search
Entry: `web/src/components/SearchPanel.tsx` (326) → ui/PanelShell, Avatar, Icon

## Account / profile / preferences
Entries: `web/src/components/ProfilePanel.tsx` (300), `web/src/components/AkunPanel.tsx`, `web/src/components/PreferensiPanel.tsx` (130) → all built on `web/src/components/ui/PanelShell.tsx` (154) + `web/src/components/ui/Field.tsx` (72)
