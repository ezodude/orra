package main

import (
	"context"
	"net/http"
	"strings"

	"github.com/gilcrest/diygoapi/errs"
)

func (app *App) APIKeyMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			errs.HTTPErrorResponse(w, app.Logger, errs.E(errs.Unauthorized, "Authorization header is missing"))
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			errs.HTTPErrorResponse(w, app.Logger, errs.E(errs.Unauthorized, "Invalid Authorization header format"))
			return
		}

		apiKey := parts[1]

		// Store the API key in the request context
		ctx := context.WithValue(r.Context(), "api_key", apiKey)
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	}
}
