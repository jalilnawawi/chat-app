# Routes

**There is no router.** No react-router, no file-based routing, no URL per conversation. The app deliberately avoids per-conversation routes so that opening a notification never reloads the page and drops the WebSocket (see the comments in `web/src/App.tsx`).

Entry: `web/src/main.tsx` → `web/src/App.tsx`.

## Top-level screens (state, not URL)

| Screen | Condition in `App.tsx` | Component |
| --- | --- | --- |
| Loading | `checking === true` | inline `<Tanda />` + "Memuat…" |
| Auth | `me === null` | `components/AuthPage.tsx` |
| App shell | `me !== null` | `Sidebar` + `ChatPanel` + optional right panel |

## Right-hand column (one at a time)

`App.tsx` keeps two pieces of state: `panel: 'profil' | 'akun' | 'preferensi' | undefined` and `search: string | null | undefined`. They share the same column, so opening one closes the other.

| Value | Component |
| --- | --- |
| `panel === 'profil'` | `components/ProfilePanel.tsx` |
| `panel === 'akun'` | `components/AkunPanel.tsx` |
| `panel === 'preferensi'` | `components/PreferensiPanel.tsx` |
| `search !== undefined` | `components/SearchPanel.tsx` (`null` = all conversations, string = one) |

## Query params consumed once, then stripped

- `?verify=<token>` — email verification link, handled before the session check.
- `?c=<conversationId>` — opened from a push notification when no tab was open.

## Main screen: chat

`ChatPanel` renders the active conversation chosen by `activeId` in the zustand store (`web/src/store.ts`). Its own sub-surfaces (group panel, forward dialog, search jump, pin bar) are local state inside `ChatPanel`, not routes.
