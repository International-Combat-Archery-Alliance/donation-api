package api

import (
	"net/http"

	"github.com/International-Combat-Archery-Alliance/middleware"
	"github.com/rs/cors"
)

func (a *API) corsMiddleware() middleware.MiddlewareFunc {
	var serverCors *cors.Cors

	switch a.env {
	case LOCAL:
		serverCors = cors.New(cors.Options{
			AllowedOrigins: []string{"http://localhost:4173", "http://localhost:5173"},
			AllowedMethods: []string{
				http.MethodHead,
				http.MethodGet,
				http.MethodPost,
			},
			AllowedHeaders:   []string{"*"},
			AllowCredentials: true,
		})
	case PROD:
		serverCors = cors.New(cors.Options{
			AllowedOrigins: []string{"https://www.icaa.world", "https://icaa.world"},
			AllowedMethods: []string{
				http.MethodHead,
				http.MethodGet,
				http.MethodPost,
			},
			AllowedHeaders:   []string{"*"},
			MaxAge:           300,
			AllowCredentials: true,
		})
	}

	return serverCors.Handler
}
