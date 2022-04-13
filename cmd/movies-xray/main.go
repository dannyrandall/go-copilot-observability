package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-xray-sdk-go/instrumentation/awsv2"
	"github.com/aws/aws-xray-sdk-go/xray"
	"github.com/dannyrandall/movies/internal/copilot"
	"github.com/dannyrandall/movies/internal/handlers"
)

func main() {
	svcName := copilot.ServiceName()

	moviesTable, ok := os.LookupEnv("MOVIES_NAME")
	if !ok {
		log.Fatalf("MOVIES_NAME is not set")
	}
	log.Printf("Using %q as the movies table", moviesTable)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("unable to load aws config: %s", err)
	}

	awsv2.AWSV2Instrumentor(&cfg.APIOptions)
	ddb := dynamodb.NewFromConfig(cfg)

	movieHandler := &handlers.Movie{
		Dynamo:      ddb,
		MoviesTable: moviesTable,
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	mux.Handle("/api/xray/movie", xray.Handler(xray.NewFixedSegmentNamer(svcName), movieHandler))

	log.Printf("Starting server on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("error serving: %s", err)
	}
}
