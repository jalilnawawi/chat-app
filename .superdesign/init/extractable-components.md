# Extractable Components

Candidates for Superdesign `DraftComponent` extraction. Only components that appear across screens or define a shared pattern are listed; one-off dialogs and pure primitives are skipped.

## Layout Components

## Sidebar
- Source: `web/src/components/Sidebar.tsx`
- Category: layout
- Description: Left column — brand mark, own avatar + status menu, search/new-chat actions, panel buttons, and the conversation list
- Extractable props: hiddenOnMobile (boolean, default: false), panel (string, default: ""), searchOpen (boolean, default: false), activeConversationId (string, default: "")
- Hardcoded: all icon names, Indonesian labels, all CSS classes, avatar fallback logic

## ChatHeader
- Source: `web/src/components/ChatHeader.tsx`
- Category: layout
- Description: Top bar of the chat column — back button (mobile), avatar, title, presence/typing line, and header actions
- Extractable props: title (string, default: "Percakapan"), subtitle (string, default: ""), showBack (boolean, default: false)
- Hardcoded: icon names, button labels, all CSS

## PanelShell
- Source: `web/src/components/ui/PanelShell.tsx`
- Category: layout
- Description: Right-hand panel frame (heading, close button, scroll body) shared by profile, account, preferences, search, and group panels
- Extractable props: title (string, default: "Panel")
- Hardcoded: close icon, spacing, borders

## Composer
- Source: `web/src/components/Composer.tsx`
- Category: layout
- Description: Bottom write row — pill input, attach, emoji, voice record, send; also hosts mention list, upload strip, recording bar
- Extractable props: placeholder (string, default: "Tulis pesan"), replyingTo (string, default: ""), editing (boolean, default: false)
- Hardcoded: icon names, pill geometry, focus-ring opt-out class `cincin-sendiri`

## Basic Components

## MessageBubble
- Source: `web/src/components/MessageBubble.tsx`
- Category: basic
- Description: One message row — avatar, bubble (own = teal, other = surface + `line-bubble`), quoted reply, attachments, reactions, hover menu arrow, time + delivery ticks
- Extractable props: mine (boolean, default: false), runStart (boolean, default: true), runEnd (boolean, default: true), state (string, default: "sent")
- Hardcoded: bubble radii (`--radius-bubble`, `--radius-tail`), tick icons, all CSS

## PinBar
- Source: `web/src/components/PinBar.tsx`
- Category: basic
- Description: Pinned-message strip under the header, with cycle and unpin actions
- Extractable props: count (number, default: 1), index (number, default: 0)
- Hardcoded: icons, text, CSS

## ReactionRow
- Source: `web/src/components/ReactionRow.tsx`
- Category: basic
- Description: Reaction chips under a bubble plus the add-reaction affordance
- Extractable props: mine (boolean, default: false)
- Hardcoded: emoji picker trigger icon, chip styling

## Avatar
- Source: `web/src/components/Avatar.tsx`
- Category: basic
- Description: Round avatar with per-name hue fallback (uses `--avatar-*` tokens) and optional presence dot
- Extractable props: name (string, default: "A"), size (number, default: 40), presence (string, default: "")
- Hardcoded: hue computation, token names

## IconButton / PillButton
- Source: `web/src/components/ui/IconButton.tsx`, `web/src/components/ui/PillButton.tsx`
- Category: basic
- Description: The only two button primitives in the app (square icon button; pill text button)
- Extractable props: — (too simple to extract; inline them in drafts instead)
- Hardcoded: sizes, hover/active states, focus ring behaviour
