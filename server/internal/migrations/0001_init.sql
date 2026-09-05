-- Fondasi skema chat.
--
-- Keputusan desain penting:
--  1. "conversation" adalah satu abstraksi untuk DM dan grup, dibedakan kolom `type`.
--     DM = conversation 2 orang. Ini bikin semua logic pesan/read/typing seragam,
--     nggak perlu dua jalur kode.
--  2. `direct_key` memastikan cuma ada SATU DM antar sepasang user. Tanpa ini,
--     dua orang yang klik "chat" barengan bikin dua ruang terpisah.
--  3. `seq` monotonic per conversation jadi sumber kebenaran urutan pesan.
--     created_at nggak cukup: dua pesan bisa punya timestamp identik, dan jam
--     server bisa mundur. `seq` bikin pagination dan resume setelah reconnect
--     deterministik.
--  4. Primary key `messages.id` diisi UUIDv7 dari CLIENT. Itu yang bikin retry
--     aman (idempotent) dan optimistic UI bisa mencocokkan pesan lokal dengan
--     balasan server.

CREATE TABLE users (
    id            uuid        PRIMARY KEY,
    username      text        NOT NULL,
    display_name  text        NOT NULL,
    password_hash text        NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now()
);

-- Username case-insensitive tanpa perlu extension citext.
CREATE UNIQUE INDEX users_username_lower_idx ON users (lower(username));

-- Session disimpan sebagai HASH dari token. Kalau DB bocor, isinya nggak bisa
-- dipakai login (sama alasannya kayak nggak nyimpen password plaintext).
CREATE TABLE sessions (
    token_hash bytea       PRIMARY KEY,
    user_id    uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL
);

CREATE INDEX sessions_user_id_idx    ON sessions (user_id);
CREATE INDEX sessions_expires_at_idx ON sessions (expires_at);

CREATE TABLE conversations (
    id         uuid        PRIMARY KEY,
    type       text        NOT NULL CHECK (type IN ('direct', 'group')),
    title      text,
    created_by uuid        NOT NULL REFERENCES users(id),
    created_at timestamptz NOT NULL DEFAULT now(),

    -- Counter alokasi `seq`. Di-increment atomik saat kirim pesan
    -- (UPDATE ... RETURNING) sehingga urutan gapless & bebas race.
    last_seq   bigint      NOT NULL DEFAULT 0,

    -- Untuk 'direct': "<uuid-kecil>:<uuid-besar>" biar pasangan yang sama selalu
    -- menghasilkan kunci sama, ke arah mana pun chat dimulai. NULL untuk grup.
    direct_key text,

    CONSTRAINT conversations_direct_key_required CHECK (
        (type = 'direct' AND direct_key IS NOT NULL AND title IS NULL) OR
        (type = 'group'  AND direct_key IS NULL     AND title IS NOT NULL)
    )
);

CREATE UNIQUE INDEX conversations_direct_key_idx
    ON conversations (direct_key) WHERE direct_key IS NOT NULL;

CREATE TABLE conversation_members (
    conversation_id uuid        NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    user_id         uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role            text        NOT NULL DEFAULT 'member' CHECK (role IN ('owner', 'member')),
    joined_at       timestamptz NOT NULL DEFAULT now(),

    -- Dasar read receipt DAN unread badge sekaligus:
    -- unread = conversations.last_seq - last_read_seq.
    last_read_seq   bigint      NOT NULL DEFAULT 0,

    PRIMARY KEY (conversation_id, user_id)
);

-- "Daftar percakapan milik saya" — query paling sering dipanggil.
CREATE INDEX conversation_members_user_idx ON conversation_members (user_id);

CREATE TABLE messages (
    -- UUIDv7 dari client: idempotency key sekaligus PK. Kirim ulang pesan yang
    -- sama (retry setelah timeout) nggak bikin duplikat.
    id              uuid        PRIMARY KEY,
    conversation_id uuid        NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    seq             bigint      NOT NULL,
    sender_id       uuid        NOT NULL REFERENCES users(id),
    body            text        NOT NULL,

    -- Disiapkan sekarang, dipakai saat fitur upload masuk. Bentuknya:
    -- [{"url":"...","mime":"image/png","size":1234,"name":"foto.png"}]
    attachments     jsonb       NOT NULL DEFAULT '[]'::jsonb,

    created_at      timestamptz NOT NULL DEFAULT now(),
    edited_at       timestamptz,
    -- Soft delete: baris tetap ada supaya `seq` nggak bolong dan client yang
    -- lagi offline tetap bisa sinkron "pesan ini dihapus".
    deleted_at      timestamptz,

    UNIQUE (conversation_id, seq)
);

-- Cursor pagination: WHERE conversation_id = $1 AND seq < $2 ORDER BY seq DESC.
CREATE INDEX messages_conversation_seq_idx
    ON messages (conversation_id, seq DESC);
