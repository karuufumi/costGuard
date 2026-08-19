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
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"costguard/apis"
	cli "costguard/interface"
	"costguard/internal/catalog"
	"costguard/internal/domain"
	"costguard/internal/estimate"
)

func main() {
	estimator := estimate.NewCalculator(catalog.NewEmbedded())
	args := os.Args[1:]
	if len(args) > 0 && args[0] == "serve" {
		if err := serve(estimator); err != nil {
			log.Fatal(err)
		}
		return
	}

	app := cli.New(estimator, os.Stdin, os.Stdout, os.Stderr, stdinIsTerminal)
	if err := app.Run(context.Background(), args); err != nil {
		fmt.Fprintf(os.Stderr, "costguard: %v\n", err)
		if errors.Is(err, domain.ErrInvalidEstimate) || errors.Is(err, domain.ErrUnsupportedEstimate) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}

func serve(estimator domain.Estimator) error {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}

	server := &http.Server{
		Addr:              "127.0.0.1:" + port,
		Handler:           apis.NewHandler(apis.HandlerConfig{Estimator: estimator}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Printf("costGuard API listening on http://localhost:%s", port)
	return server.ListenAndServe()
}

func stdinIsTerminal() bool {
	info, err := os.Stdin.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
