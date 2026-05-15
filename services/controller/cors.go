package main

import "net/http"

// withCORS allows browser clients (including Connect preflight) to call the controller from another origin.
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Echo requested headers so custom Connect headers pass preflight without maintaining a fixed list.
		if h := r.Header.Get("Access-Control-Request-Headers"); h != "" {
			w.Header().Set("Access-Control-Allow-Headers", h)
		} else {
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Connect-Protocol-Version, Connect-Timeout-Ms, Connect-Accept-Encoding, Connect-Content-Encoding, Grpc-Timeout, X-Grpc-Web, X-User-Agent")
		}
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
