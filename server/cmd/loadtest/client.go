package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
)

// apiClient adalah satu user virtual: satu sesi, satu koneksi WebSocket.
type apiClient struct {
	base     string
	wsBase   string
	http     *http.Client
	username string
	password string

	id     uuid.UUID
	cookie string
	conn   *websocket.Conn
	convs  []uuid.UUID
}

type httpError struct {
	Status     int
	Body       string
	RetryAfter time.Duration
}

func (e *httpError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.Status, e.Body)
}

// do mengirim satu permintaan JSON dan menempelkan cookie sesi bila sudah ada.
func (c *apiClient) do(ctx context.Context, method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.base+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.cookie != "" {
		req.Header.Set("Cookie", c.cookie)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode >= 400 {
		e := &httpError{Status: resp.StatusCode, Body: string(bytes.TrimSpace(raw))}
		if secs, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil {
			e.RetryAfter = time.Duration(secs) * time.Second
		}
		return e
	}

	for _, ck := range resp.Cookies() {
		if ck.Name == "chat_session" && ck.Value != "" {
			c.cookie = ck.Name + "=" + ck.Value
		}
	}

	if out != nil && len(raw) > 0 {
		return json.Unmarshal(raw, out)
	}
	return nil
}

// ensureAccount memakai akun yang sudah ada bila mungkin.
//
// Login lebih murah dari register hanya di sisi database — argon2id tetap
// dihitung di keduanya. Yang benar-benar dihemat adalah tidak menumpuk ribuan
// user baru setiap kali load test dijalankan ulang.
func (c *apiClient) ensureAccount(ctx context.Context) error {
	var user struct {
		ID uuid.UUID `json:"id"`
	}

	err := c.do(ctx, "POST", "/api/auth/login", map[string]string{
		"username": c.username,
		"password": c.password,
	}, &user)
	if err == nil {
		c.id = user.ID
		return nil
	}

	var he *httpError
	if !asHTTPError(err, &he) || he.Status != http.StatusUnauthorized {
		return err
	}

	if err := c.do(ctx, "POST", "/api/auth/register", map[string]string{
		"username":    c.username,
		"displayName": c.username,
		"password":    c.password,
	}, &user); err != nil {
		return err
	}
	c.id = user.ID
	return nil
}

type conversationSummary struct {
	ID    uuid.UUID `json:"id"`
	Type  string    `json:"type"`
	Title *string   `json:"title"`
}

func (c *apiClient) listConversations(ctx context.Context) ([]conversationSummary, error) {
	var out []conversationSummary
	if err := c.do(ctx, "GET", "/api/conversations", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *apiClient) createGroup(ctx context.Context, title string, members []uuid.UUID) (uuid.UUID, error) {
	var out struct {
		ConversationID uuid.UUID `json:"conversationId"`
	}
	err := c.do(ctx, "POST", "/api/conversations/group", map[string]any{
		"title":     title,
		"memberIds": members,
	}, &out)
	return out.ConversationID, err
}

func (c *apiClient) sendMessage(ctx context.Context, convID uuid.UUID, body string) error {
	return c.do(ctx, "POST", "/api/conversations/"+convID.String()+"/messages", map[string]any{
		"id":   uuid.New(),
		"body": body,
	}, nil)
}

// connect membuka WebSocket dengan cookie sesi, persis seperti browser.
func (c *apiClient) connect(ctx context.Context) error {
	conn, _, err := websocket.Dial(ctx, c.wsBase+"/ws", &websocket.DialOptions{
		HTTPClient: c.http,
		HTTPHeader: http.Header{"Cookie": []string{c.cookie}},
	})
	if err != nil {
		return err
	}
	// Pesan masuk hanya dibaca dan dibuang isinya setelah dicatat, tapi batas
	// tetap dipasang supaya satu balasan raksasa tidak menghabiskan memori
	// proses uji dan mengaburkan hasil pengukuran.
	conn.SetReadLimit(1 << 20)
	c.conn = conn
	return nil
}

// sync meniru langkah pertama client sungguhan setelah tersambung, supaya jalur
// resume ikut terbebani — bukan cuma jalur siaran.
func (c *apiClient) sync(ctx context.Context) error {
	cursors := make(map[string]int64, len(c.convs))
	for _, id := range c.convs {
		cursors[id.String()] = 0
	}
	raw, err := json.Marshal(map[string]any{
		"type":    "sync",
		"payload": map[string]any{"cursors": cursors},
	})
	if err != nil {
		return err
	}
	return c.conn.Write(ctx, websocket.MessageText, raw)
}

func asHTTPError(err error, target **httpError) bool {
	he, ok := err.(*httpError)
	if ok {
		*target = he
	}
	return ok
}
