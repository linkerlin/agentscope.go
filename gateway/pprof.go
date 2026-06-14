package gateway

import (
	"net/http"
	"net/http/pprof"
)

// WithPProf registers Go pprof profiling endpoints on the server's mux.
// This enables runtime profiling via HTTP endpoints like /debug/pprof/heap,
// /debug/pprof/profile, /debug/pprof/goroutine, etc.
//
// In production, these endpoints should be protected by authentication or
// served on a separate internal port. Use WithPProfOnMux to register on
// a custom mux for isolation.
func (s *Server) WithPProf() *Server {
	registerPProf(s.mux)
	return s
}

// RegisterPProfOnMux registers pprof handlers on the given ServeMux.
// Use this to serve pprof on a separate internal listener.
func RegisterPProfOnMux(mux *http.ServeMux) {
	registerPProf(mux)
}

func registerPProf(mux *http.ServeMux) {
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
}
