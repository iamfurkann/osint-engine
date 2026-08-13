package web

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"sync"

	"github.com/iamfurkann/osint-engine/internal/ipc"
	"github.com/rs/zerolog/log"
)

//go:embed static/*
var staticFiles embed.FS

// Server, gömülü web sunucusunu yönetir.
type Server struct {
	port      int
	ipcClient *ipc.Client
	server    *http.Server
	wg        sync.WaitGroup
}

// NewServer, yeni bir Web Sunucusu oluşturur.
func NewServer(port int, ipcSocketPath string) *Server {
	return &Server{
		port:      port,
		ipcClient: ipc.NewClient(ipcSocketPath),
	}
}

// Start, web sunucusunu başlatır.
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// Statik dosyaları sun (index.html, app.js, vb.)
	fsys, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return fmt.Errorf("statik dosyalar yüklenemedi: %w", err)
	}
	mux.Handle("/", http.FileServer(http.FS(fsys)))

	// API endpointleri
	mux.HandleFunc("/api/graph", s.handleGetGraph)

	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: mux,
	}

	log.Info().Int("port", s.port).Msg("Web Viewer started")

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("Web server error")
		}
	}()

	return nil
}

// Stop, sunucuyu durdurur.
func (s *Server) Stop() {
	if s.server != nil {
		s.server.Close()
	}
	s.wg.Wait()
	log.Info().Msg("Web Viewer stopped")
}

// handleGetGraph, IPC üzerinden graf verilerini (Cytoscape formatı) çeker.
func (s *Server) handleGetGraph(w http.ResponseWriter, r *http.Request) {
	invID := r.URL.Query().Get("id")
	if invID == "" {
		http.Error(w, "id parametresi gerekli", http.StatusBadRequest)
		return
	}

	if !s.ipcClient.IsRunning() {
		http.Error(w, "Daemon çalışmıyor", http.StatusServiceUnavailable)
		return
	}

	res, err := s.ipcClient.Call("investigation.graph", map[string]string{"id": invID})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(res); err != nil {
		log.Error().Err(err).Msg("web: failed to write graph response")
	}
}
