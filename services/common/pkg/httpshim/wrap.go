package httpshim

import (
	"net/http"

	authshim "github.com/jamesread/httpauthshim"
	"github.com/jamesread/httpauthshim/authpublic"
)

// Wrap attaches httpauthshim request authentication so upstream handlers run with the same behavior.
func Wrap(inner http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg := &authpublic.Config{}
		_ = authshim.AuthFromHttpReq(r, cfg)
		inner.ServeHTTP(w, r)
	})
}
