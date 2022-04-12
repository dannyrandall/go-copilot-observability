package main

import (
	"log"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-xray-sdk-go/xray"
	"github.com/dannyrandall/movies/internal/copilot"
	"github.com/dannyrandall/movies/internal/handlers"
)

type Movie struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Year  int    `json:"year"`
}

func main() {
	svcName := copilot.ServiceName()

	moviesTable, ok := os.LookupEnv("MOVIES_NAME")
	if !ok {
		log.Fatalf("MOVIES_NAME is not set")
	}
	log.Printf("Using %q as the movies table", moviesTable)

	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	ddb := dynamodb.New(sess)
	xray.AWS(ddb.Client)

	movieHandler := &handlers.Movie{
		Dynamo:      ddb,
		MoviesTable: moviesTable,
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("No handler registered for path %q", r.URL.String())
		http.NotFound(w, r)
	})

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	mux.Handle("/api/xray/movie", xray.Handler(xray.NewFixedSegmentNamer(svcName), movieHandler))

	log.Printf("Starting server on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("error serving: %s", err)
	}
}
