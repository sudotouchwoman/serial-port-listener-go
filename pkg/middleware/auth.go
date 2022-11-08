package middleware

import (
	"context"
	"net/http"

	"github.com/sudotouchwoman/serial-port-listener-go/pkg/common"
)

const AccessToken = "AccessToken"

type Authorizer interface {
	Verify(token string) (string, error)
}

func Middleware(a Authorizer, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// check if credentials were passed
		token := r.Header.Get(AccessToken)
		if idKey, err := a.Verify(token); err == nil {
			// add id key to context as to be accessible
			// by underlying handlers (e.g. to check if user
			//  currently has several actvie sessions)
			ctx := r.Context()
			next.ServeHTTP(w, r.WithContext(context.WithValue(
				ctx, common.ClientIDKey, idKey,
			)))
			return
		}
		http.Error(w, "Access token missing", http.StatusForbidden)
	})
}
