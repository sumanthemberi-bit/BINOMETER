package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Event is something that happened, sent live to the dashboard.
// type can be: reading, lid, hand, status, alert, connected, disconnected
type Event struct {
	Type  string      `json:"type"`
	BinID string      `json:"bin_id"`
	Data  interface{} `json:"data,omitempty"`
	Time  time.Time   `json:"time"`
}

// Hub sends every event to everyone who is listening
// (browser tabs through /api/stream, and the USB reader).
type Hub struct {
	mu      sync.Mutex
	clients map[chan Event]bool
}

func NewHub() *Hub {
	return &Hub{clients: make(map[chan Event]bool)}
}

func (h *Hub) Subscribe() chan Event {
	ch := make(chan Event, 50)
	h.mu.Lock()
	h.clients[ch] = true
	h.mu.Unlock()
	return ch
}

func (h *Hub) Unsubscribe(ch chan Event) {
	h.mu.Lock()
	delete(h.clients, ch)
	h.mu.Unlock()
}

func (h *Hub) Publish(e Event) {
	if e.Time.IsZero() {
		e.Time = time.Now()
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.clients {
		// dont wait if a client is slow, just skip the event for them
		select {
		case ch <- e:
		default:
		}
	}
}

// GET /api/stream
// This uses server sent events. The browser keeps this connection open and gets each event as it happens.
// Easier than websockets, works with plain net/http.
func (s *Server) stream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := s.hub.Subscribe()
	defer s.hub.Unsubscribe(ch)

	fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()

	ping := time.NewTicker(20 * time.Second)
	defer ping.Stop()

	for {
		select {
		case <-r.Context().Done():
			return // browser tab closed
		case e := <-ch:
			data, _ := json.Marshal(e)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case <-ping.C:
			// keeps the connection from being closed by the browser
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}
