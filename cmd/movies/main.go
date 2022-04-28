package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/dannyrandall/movies/internal/copilot"
	"github.com/dannyrandall/movies/internal/otel"
	"go.opentelemetry.io/contrib/instrumentation/github.com/aws/aws-sdk-go-v2/otelaws"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func main() {
	// Get DynamoDB table name
	moviesTable, ok := os.LookupEnv("MOVIES_NAME")
	if !ok {
		log.Fatalf("MOVIES_NAME is not set")
	}
	log.Printf("Using %q as the DynamoDB movies table", moviesTable)

	// Timeout for setup functions
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	svcName := copilot.ServiceName("movies")
	if err := otel.SetupTracer(ctx, svcName); err != nil {
		log.Fatalf("unable to setup otel tracer: %s", err)
	}

	// Load AWS SDK config
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("unable to load aws config: %s", err)
	}

	otelaws.AppendMiddlewares(&cfg.APIOptions)

	// Setup HTTP server
	mux := http.NewServeMux()

	// Simple health check endpoint
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	// API Endpoints
	mux.Handle("/movies/api/movie", otelhttp.NewHandler(&MovieHandler{
		Dynamo:      dynamodb.NewFromConfig(cfg),
		MoviesTable: moviesTable,
	}, "movie"))

	// Run HTTP Server
	log.Printf("Starting server on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("error serving: %s", err)
	}
}
