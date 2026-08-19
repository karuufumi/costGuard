// @title costGuard API
// @version 1.0
// @description Private API for estimating cloud costs across AWS, Azure, and GCP.
// @description Estimates are informational and depend on the selected pricing catalog.
// @BasePath /
// @schemes http https
//
//go:generate go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -g main.go -d .,apis -o docs --outputTypes go,json,yaml

package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"costguard/apis"
	"costguard/internal/catalog"
	"costguard/internal/estimate"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           apis.NewHandler(apis.HandlerConfig{Estimator: estimate.NewCalculator(catalog.NewEmbedded())}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("costGuard API listening on http://localhost:%s", port)
	log.Fatal(server.ListenAndServe())
}
