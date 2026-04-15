package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/kokinedo/pipestream/internal/store"
	"github.com/kokinedo/pipestream/pkg/models"
)

// Server is the HTTP/WebSocket server for the pipeline API.
type Server struct {
	store *store.Store
	port  int
	bus   *EventBus
	mux   *http.ServeMux
}

// NewServer creates a Server.
func NewServer(st *store.Store, port int, bus *EventBus) *Server {
	s := &Server{store: st, port: port, bus: bus, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/events", s.handleGetEvents)
	s.mux.HandleFunc("GET /api/events/{id}", s.handleGetEvent)
	s.mux.HandleFunc("GET /api/stats", s.handleGetStats)
	s.mux.HandleFunc("GET /api/ws", s.handleWS)
}

// Start runs the HTTP server and blocks until ctx is cancelled.
func (s *Server) Start(ctx context.Context) error {
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: corsMiddleware(s.mux),
	}

	go func() {
		<-ctx.Done()
		srv.Close()
	}()

	log.Printf("[server] listening on :%d", s.port)
	err := srv.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleGetEvents(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	events, err := s.store.GetEvents(r.Context(), limit, offset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Filter by category if provided.
	cat := r.URL.Query().Get("category")
	if cat != "" {
		var filtered []models.ClassifiedEvent
		for _, e := range events {
			if string(e.Category) == cat {
				filtered = append(filtered, e)
			}
		}
		events = filtered
	}

	writeJSON(w, http.StatusOK, events)
}

func (s *Server) handleGetEvent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	events, err := s.store.GetEvents(r.Context(), 1000, 0)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	for _, e := range events {
		if e.ID == id {
			writeJSON(w, http.StatusOK, e)
			return
		}
	}
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "event not found"})
}

func (s *Server) handleGetStats(w http.ResponseWriter, r *http.Request) {
	catCounts, total, err := s.store.GetStats(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"total":      total,
		"categories": catCounts,
	})
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		log.Printf("[server] websocket accept error: %v", err)
		return
	}
	defer conn.CloseNow()

	ch, unsub := s.bus.Subscribe()
	defer unsub()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			conn.Close(websocket.StatusNormalClosure, "server shutting down")
			return
		case event, ok := <-ch:
			if !ok {
				return
			}
			if err := wsjson.Write(ctx, conn, event); err != nil {
				return
			}
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
