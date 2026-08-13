package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/iamfurkann/osint-engine/internal/ai"
	"github.com/iamfurkann/osint-engine/internal/config"
	"github.com/iamfurkann/osint-engine/internal/domain"
	"github.com/iamfurkann/osint-engine/internal/engine/orchestrator"
	"github.com/iamfurkann/osint-engine/internal/engine/watch"
	"github.com/rs/zerolog/log"
)

type Server struct {
	addr         string
	server       *http.Server
	config       *config.Config
	orchestrator *orchestrator.Orchestrator
	watcher      *watch.Watcher
	geminiClient *ai.GeminiClient
}

func NewServer(addr string, appCfg *config.Config, orch *orchestrator.Orchestrator, watcher *watch.Watcher) *Server {
	return &Server{
		addr:         addr,
		config:       appCfg,
		orchestrator: orch,
		watcher:      watcher,
		geminiClient: ai.NewGeminiClient(),
	}
}

// allowedOrigins, tarayıcıdan API'ye erişebilecek origin'lerin kapalı listesidir.
//
// Daha önce burada "Access-Control-Allow-Origin: *" vardı. Bu, kullanıcının
// ziyaret ettiği HERHANGİ bir web sayfasının bu daemon'a istek atabilmesi
// demekti: geçmiş araştırmaları listelemek, toplanmış tüm kişisel veriyi
// okumak, yeni tarama başlatmak ve API kotasını harcamak. Kimlik doğrulama
// olmadığı için tarayıcı hiçbir kimlik bilgisi göndermeden bunu yapabiliyordu.
var allowedOrigins = map[string]bool{
	"http://localhost:5173": true, // Vite dev server
	"http://127.0.0.1:5173": true,
	"http://localhost:4173": true, // Vite preview
	"http://127.0.0.1:4173": true,
	"http://localhost:8080": true, // aynı origin (gömülü UI)
	"http://127.0.0.1:8080": true,
}

// allowedHosts, DNS rebinding saldırılarına karşı Host başlığı denetimidir.
// Loopback'e bağlanmak tek başına yetmez: saldırganın kontrolündeki bir alan adı
// 127.0.0.1'e çözümlenirse tarayıcı isteği yine buraya getirir.
var allowedHosts = map[string]bool{
	"localhost:8080": true,
	"127.0.0.1:8080": true,
	"[::1]:8080":     true,
}

func (s *Server) Start() error {
	mux := http.NewServeMux()

	securityMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !allowedHosts[r.Host] {
				log.Warn().Str("host", r.Host).Str("remote", r.RemoteAddr).
					Msg("Request rejected — unexpected Host header (possible DNS rebinding)")
				http.Error(w, "invalid host", http.StatusForbidden)
				return
			}

			// Origin yalnızca izin listesindeyse yansıtılır; aksi hâlde
			// CORS başlığı hiç gönderilmez ve tarayıcı yanıtı bloklar.
			if origin := r.Header.Get("Origin"); origin != "" && allowedOrigins[origin] {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, DELETE")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			}
			w.Header().Add("Vary", "Origin")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}
			next.ServeHTTP(w, r)
		})
	}

	mux.HandleFunc("GET /api/investigations", s.handleListInvestigations)
	mux.HandleFunc("POST /api/investigations", s.handleStartInvestigation)
	mux.HandleFunc("GET /api/investigations/", s.handleGetInvestigation)  // Handle both graph and report
	mux.HandleFunc("POST /api/investigations/", s.handleGetInvestigation) // Handle analyze requests

	mux.HandleFunc("GET /api/watch", s.handleListWatchlist)
	mux.HandleFunc("POST /api/watch", s.handleAddWatch)
	mux.HandleFunc("DELETE /api/watch", s.handleRemoveWatch)

	s.server = &http.Server{
		Addr:              s.addr,
		Handler:           securityMiddleware(mux),
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Info().Str("addr", s.addr).Msg("HTTP API Server starting")

	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("HTTP API Server failed")
		}
	}()

	return nil
}

func (s *Server) Stop() {
	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.server.Shutdown(ctx); err != nil {
			log.Error().Err(err).Msg("HTTP API Server shutdown failed")
		}
		log.Info().Msg("HTTP API Server stopped")
	}
}

// Handlers
func (s *Server) handleListInvestigations(w http.ResponseWriter, r *http.Request) {
	invs, err := s.orchestrator.ListInvestigations(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(invs); err != nil {
		log.Error().Err(err).Msg("Failed to encode investigations response")
	}
}

func (s *Server) handleStartInvestigation(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Target    string `json:"target"`
		Type      string `json:"type"` // e.g. domain, username
		Recursive bool   `json:"recursive"`
		Hints     struct {
			KnownUsernames []string `json:"known_usernames"`
			Email          string   `json:"email"`
			Phone          string   `json:"phone"`
			Location       string   `json:"location"`
			BirthYear      int      `json:"birth_year"`
		} `json:"hints"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	cleanTarget := req.Target
	invID := fmt.Sprintf("inv-%s-%d", cleanTarget, time.Now().Unix())

	if err := s.orchestrator.StartInvestigation(r.Context(), invID, req.Target, req.Type, req.Recursive); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// İpuçlarından türetilen yan araştırmalar. Arka planda koştukları için
	// hataları sessizce kaybolmasın diye açıkça log'lanıyor.
	startSideInvestigation := func(id, target, inputType string) {
		go func() {
			if err := s.orchestrator.StartInvestigation(
				context.Background(), id, target, inputType, req.Recursive,
			); err != nil {
				log.Error().Err(err).
					Str("investigation_id", id).
					Str("type", inputType).
					Msg("Hint-derived investigation failed to start")
			}
		}()
	}

	if req.Hints.Email != "" {
		emailInvID := fmt.Sprintf("inv-%s-%d", req.Hints.Email, time.Now().Unix())
		startSideInvestigation(emailInvID, req.Hints.Email, "email")
	}
	for _, username := range req.Hints.KnownUsernames {
		uInvID := fmt.Sprintf("inv-%s-%d", username, time.Now().Unix())
		startSideInvestigation(uInvID, username, "username")
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"id": invID}); err != nil {
		log.Error().Err(err).Msg("Failed to encode start-investigation response")
	}
}

func (s *Server) handleGetInvestigation(w http.ResponseWriter, r *http.Request) {
	// Parse URL: /api/investigations/{id}/graph OR /api/investigations/{id}/report
	path := r.URL.Path
	if len(path) <= 20 {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	rest := path[20:]
	isReport := false
	id := rest

	if len(id) > 6 && id[len(id)-6:] == "/graph" {
		id = id[:len(id)-6]
	} else if len(id) > 7 && id[len(id)-7:] == "/report" {
		id = id[:len(id)-7]
		isReport = true
	}

	progress, err := s.orchestrator.GetProgress(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	g, entities, correlations, err := s.orchestrator.BuildGraph(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if isReport {
		apiKey := s.config.Keys.Get("gemini")
		if apiKey == "" {
			http.Error(w, "Gemini API key is not set", http.StatusBadRequest)
			return
		}

		entitiesJSON, _ := json.MarshalIndent(entities, "", "  ")

		report, err := s.geminiClient.AnalyzeFindings(r.Context(), apiKey, id, string(entitiesJSON))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		resp := map[string]string{"report": report}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Error().Err(err).Msg("Failed to encode report response")
		}
		return
	}

	resp := map[string]interface{}{
		"InvestigationID": id,
		"Progress":        progress.Percent,
		"GraphStats":      map[string]interface{}{"NodeCount": len(g.Nodes()), "EdgeCount": g.EdgeCount()},
		"Entities":        entities,
		"Correlations":    correlations,
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Error().Err(err).Msg("Failed to encode graph response")
	}
}

func (s *Server) handleListWatchlist(w http.ResponseWriter, r *http.Request) {
	items, err := s.watcher.List(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(items); err != nil {
		log.Error().Err(err).Msg("Failed to encode watchlist response")
	}
}

func (s *Server) handleAddWatch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Target   string `json:"target"`
		Type     string `json:"type"`
		Interval int64  `json:"interval"` // in seconds
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	id := fmt.Sprintf("%s:%s", req.Type, req.Target)
	item := &domain.WatchItem{
		ID:        id,
		Target:    req.Target,
		Type:      req.Type,
		Interval:  time.Duration(req.Interval) * time.Second,
		LastRun:   time.Time{},
		CreatedAt: time.Now().UTC(),
	}

	if err := s.watcher.Add(r.Context(), item); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleRemoveWatch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if err := s.watcher.Remove(r.Context(), req.ID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
