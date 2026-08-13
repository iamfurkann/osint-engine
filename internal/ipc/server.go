package ipc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"

	"github.com/rs/zerolog/log"
)

// Server, Unix Domain Socket üzerinden IPC isteklerini dinleyen sunucudur.
type Server struct {
	socketPath string
	listener   net.Listener
	handlers   map[string]HandlerFunc
	mu         sync.RWMutex
	done       chan struct{}
	wg         sync.WaitGroup
}

// NewServer, yeni bir IPC sunucusu oluşturur.
func NewServer(socketPath string) *Server {
	return &Server{
		socketPath: socketPath,
		handlers:   make(map[string]HandlerFunc),
		done:       make(chan struct{}),
	}
}

// RegisterHandler, bir metot için handler kaydeder.
func (s *Server) RegisterHandler(method string, handler HandlerFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[method] = handler
}

// Listen, Unix Domain Socket'i oluşturup dinlemeye başlar.
func (s *Server) Listen() error {
	// Eski socket dosyasını temizle
	os.Remove(s.socketPath)

	listener, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("ipc: failed to listen on %s: %w", s.socketPath, err)
	}
	s.listener = listener

	// Socket dosyası izinleri (sadece owner)
	if err := os.Chmod(s.socketPath, 0600); err != nil {
		listener.Close()
		return fmt.Errorf("ipc: failed to set socket permissions: %w", err)
	}

	log.Info().Str("socket", s.socketPath).Msg("IPC server listening")

	s.wg.Add(1)
	go s.acceptLoop()

	return nil
}

// Shutdown, sunucuyu güvenle kapatır.
func (s *Server) Shutdown() {
	close(s.done)
	if s.listener != nil {
		s.listener.Close()
	}
	s.wg.Wait()
	os.Remove(s.socketPath)
	log.Info().Msg("IPC server stopped")
}

// SocketPath, sunucunun dinlediği socket yolunu döndürür.
func (s *Server) SocketPath() string {
	return s.socketPath
}

// acceptLoop, gelen bağlantıları kabul eder.
func (s *Server) acceptLoop() {
	defer s.wg.Done()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.done:
				return // Graceful shutdown
			default:
				log.Error().Err(err).Msg("IPC accept error")
				continue
			}
		}

		s.wg.Add(1)
		go s.handleConnection(conn)
	}
}

// handleConnection, tek bir istemci bağlantısını işler.
func (s *Server) handleConnection(conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // Max 1MB mesaj

	for scanner.Scan() {
		line := scanner.Bytes()

		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			resp := NewErrorResponse("", fmt.Errorf("invalid request: %w", err))
			s.writeResponse(conn, resp)
			continue
		}

		resp := s.dispatch(req)
		s.writeResponse(conn, resp)
	}
}

// dispatch, isteği uygun handler'a yönlendirir.
func (s *Server) dispatch(req Request) Response {
	s.mu.RLock()
	handler, ok := s.handlers[req.Method]
	s.mu.RUnlock()

	if !ok {
		return NewErrorResponse(req.ID, fmt.Errorf("unknown method: %q", req.Method))
	}

	result, err := handler(req.Params)
	if err != nil {
		return NewErrorResponse(req.ID, err)
	}

	resp, err := NewSuccessResponse(req.ID, result)
	if err != nil {
		return NewErrorResponse(req.ID, err)
	}

	return resp
}

// writeResponse, yanıtı bağlantıya yazar (JSON + newline).
func (s *Server) writeResponse(conn net.Conn, resp Response) {
	data, err := json.Marshal(resp)
	if err != nil {
		log.Error().Err(err).Msg("IPC: failed to marshal response")
		return
	}
	data = append(data, '\n')
	if _, err := conn.Write(data); err != nil {
		log.Error().Err(err).Msg("IPC: failed to write response")
	}
}
