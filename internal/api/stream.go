package api

import (
	"bytes"
	"compress/flate"
	"encoding/json"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Stream keeps one websocket to Hyperliquid open, changing only the
// subscriptions that Watch changes and resubscribing after a reconnect.
type Stream struct {
	mu   sync.Mutex
	conn *websocket.Conn
	subs []req
}

// Watch streams coin's context, trades, grouped book and candles at iv.
func (s *Stream) Watch(coin string, iv Interval, g Grouping) {
	book := req{"type": "l2", "c": coin, "s": g.S}
	if g.M > 1 {
		book["m"] = g.M
	}
	subs := []req{{"type": "activeAssetCtx", "coin": coin}, {"type": "trades", "coin": coin}, book, {"type": "candle", "coin": coin, "interval": iv.Name}}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.send("unsubscribe", without(s.subs, subs))
	s.send("subscribe", without(subs, s.subs))
	s.subs = subs
}

// Run sends every update to send, reconnecting 3s after a drop
// (Hyperliquid allows 30 new connections a minute).
func (s *Stream) Run(send func(any)) {
	for {
		send(s.run(send))
		time.Sleep(3 * time.Second)
	}
}

func (s *Stream) run(send func(any)) error {
	c, _, err := websocket.DefaultDialer.Dial("wss://api.hyperliquid.xyz/ws", nil)
	if err != nil {
		return err
	}
	defer c.Close()
	s.mu.Lock()
	s.conn = c
	s.send("subscribe", s.subs)
	s.mu.Unlock()
	for synced := time.Now(); ; {
		var m struct {
			Channel string
			Data    json.RawMessage
		}
		c.SetReadDeadline(time.Now().Add(time.Minute))
		if err := c.ReadJSON(&m); err != nil {
			return err
		}
		if f := channels[m.Channel]; f != nil {
			v, err := f(m.Data)
			if err != nil {
				return err
			}
			send(v)
		}
		// l2 only sends changes, so resubscribe each minute for a fresh snapshot
		if time.Since(synced) > time.Minute {
			synced = time.Now()
			s.mu.Lock()
			book := slices.DeleteFunc(slices.Clone(s.subs), func(r req) bool { return r["type"] != "l2" })
			s.send("unsubscribe", book)
			s.send("subscribe", book)
			s.mu.Unlock()
		}
	}
}

// send writes subscription changes; a failed write surfaces as a read error in run.
func (s *Stream) send(method string, subs []req) {
	for _, r := range subs {
		if s.conn != nil {
			s.conn.WriteJSON(req{"method": method, "subscription": r})
		}
	}
}

// without returns the subscriptions in a that are not in b.
func without(a, b []req) []req {
	return slices.DeleteFunc(slices.Clone(a), func(r req) bool {
		return slices.ContainsFunc(b, func(o req) bool { return fmt.Sprint(o) == fmt.Sprint(r) })
	})
}

var channels = map[string]func([]byte) (any, error){
	"activeAssetCtx": decode[ActiveAssetCtx],
	"l2":             l2,
	"trades":         decode[[]Trade],
	"candle":         decode[Candle],
}

// l2 decodes the book feed: an L2Book snapshot, then L2Changes that arrive
// plain or deflate-compressed.
func l2(b []byte) (any, error) {
	var m struct {
		S *L2Book
		U L2Change
		C []byte
	}
	err := json.Unmarshal(b, &m)
	switch {
	case err != nil:
		return nil, err
	case m.S != nil:
		return *m.S, nil
	case m.C != nil:
		err = json.NewDecoder(flate.NewReader(bytes.NewReader(m.C))).Decode(&m.U)
	}
	return m.U, err
}

func decode[T any](b []byte) (any, error) {
	var v T
	err := json.Unmarshal(b, &v)
	return v, err
}
