/*
Copyright 2023 Avi Zimmerman <avi.zimmerman@gmail.com>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/user"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/webmeshproj/webmesh/pkg/cmd/ctlcmd/config"
	"github.com/webmeshproj/webmesh/pkg/mesh"
	"golang.org/x/exp/slog"
)

// Server is the daemon server.
type Server struct {
	*http.Server
	insecure bool
	log      *slog.Logger
	mesh     mesh.Mesh
	mu       sync.Mutex
}

// NewServer returns a new daemon server.
func NewServer(insecure bool) *Server {
	log := slog.Default().With("component", "daemon")
	s := &Server{
		Server: &http.Server{
			ReadTimeout:  time.Second * 5,
			WriteTimeout: time.Second * 5,
		},
		insecure: insecure,
		log:      log,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/connect", requirePOST(log, s.handleConnect))
	mux.HandleFunc("/disconnect", requirePOST(log, s.handleDisconnect))
	mux.HandleFunc("/interface-metrics", s.handleInterfaceMetrics)
	mux.HandleFunc("/query-store", requirePOST(log, s.handleQueryStore))
	s.Handler = logRequest(log, mux)
	return s
}

// ListenAndServe listens on the unix socket and serves requests.
func (s *Server) ListenAndServe() error {
	if s.insecure {
		// Mask the last 3 bits so the socket is accessible by everyone.
		syscall.Umask(0000)
	} else {
		// Mask the last bit so the socket is only accessible by the owner
		// and webmesh group.
		syscall.Umask(0007)
	}
	l, err := listen(s.insecure)
	if err != nil {
		return fmt.Errorf("listen unix socket: %w", err)
	}
	defer l.Close()
	if runtime.GOOS != "windows" && !s.insecure {
		// Change the socket ownership to the webmesh group if it exists.
		group, err := user.LookupGroup("webmesh")
		if err == nil {
			gid, err := strconv.Atoi(group.Gid)
			if err != nil {
				return fmt.Errorf("invalid gid: %w", err)
			}
			err = os.Chown(socketPath, -1, gid)
			if err != nil {
				return fmt.Errorf("chown unix socket: %w", err)
			}
		}
	}
	err = s.Server.Serve(l)
	if err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("serve: %w", err)
	}
	return nil
}

// Shutdown shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	if runtime.GOOS != "windows" {
		defer func() {
			err := os.Remove(socketPath)
			if err != nil && !os.IsNotExist(err) {
				s.log.Error("error removing unix socket", "error", err.Error())
			}
		}()
	}
	return s.Server.Shutdown(ctx)
}

// handleConnect handles a request to connect to the mesh.
func (s *Server) handleConnect(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer r.Body.Close()
	var req connectRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		s.returnError(w, fmt.Errorf("decode request: %w", err))
		return
	}
	cfg, err := config.FromFile(req.ConfigFile)
	if err != nil {
		s.returnError(w, fmt.Errorf("load config: %w", err))
		return
	}
	if s.mesh != nil {
		// Close the existing store.
		err = s.mesh.Close()
		if err != nil {
			s.returnError(w, fmt.Errorf("close existing store: %w", err))
			return
		}
	}
	s.mesh, err = newMeshConn(r.Context(), cfg, req.Options)
	if err != nil {
		s.returnError(w, err)
		return
	}
	s.returnOK(w)
}

// handleDisconnect handles a request to disconnect from the mesh.
func (s *Server) handleDisconnect(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer r.Body.Close()
	if s.mesh == nil {
		s.returnError(w, errNotConnected)
		return
	}
	err := s.mesh.Close()
	if err != nil {
		s.returnError(w, err)
		return
	}
	s.mesh = nil
	s.returnOK(w)
}

// handleInterfaceMetrics handles a request to get the interface metrics.
func (s *Server) handleInterfaceMetrics(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer r.Body.Close()
	if s.mesh == nil {
		s.returnError(w, errNotConnected)
		return
	}
	metrics, err := s.mesh.WireGuard().Metrics()
	if err != nil {
		s.returnError(w, fmt.Errorf("get interface metrics: %w", err))
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(metrics)
	if err != nil {
		s.log.Error("error encoding interface metrics", "error", err.Error())
	}
}

// handleQueryStore handles a request to query the mesh store.
func (s *Server) handleQueryStore(w http.ResponseWriter, r *http.Request) {}

// returnOK returns an OK response.
func (s *Server) returnOK(w http.ResponseWriter) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(daemonOKResponse{OK: true})
	if err != nil {
		s.log.Error("error encoding ok response", "error", err.Error())
	}
}

// returnError returns an error response.
func (s *Server) returnError(w http.ResponseWriter, err error) {
	s.log.Error("error handling daemon request", "error", err.Error())
	w.WriteHeader(http.StatusInternalServerError)
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(daemonError{Message: err.Error()})
	if err != nil {
		s.log.Error("error encoding error response", "error", err.Error())
	}
}

// logRequest logs the request.
func logRequest(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Info("handling daemon request", "method", r.Method, "path", r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

// requirePOST returns a handler that only allows POST requests.
func requirePOST(log *slog.Logger, next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			log.Error("invalid method", "method", r.Method)
			return
		}
		next(w, r)
	})
}
