package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-xray-sdk-go/xray"
	"github.com/segmentio/ksuid"
)

type Movie struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Year  int    `json:"year"`
}

func main() {
	ddbTable, ok := os.LookupEnv("MOVIES_NAME")
	if !ok {
		log.Fatalf("MOVIES_NAME is not set")
	}

	log.Printf("Using %q as the movies table", ddbTable)

	name := svcName()

	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))

	ddb := dynamodb.New(sess)
	xray.AWS(ddb.Client)

	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	mux.Handle("/api/v0/movie", xray.Handler(xray.NewFixedSegmentNamer(name), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handleMovieGet(w, r, ddb, ddbTable)
		case http.MethodPost:
			handleMovieCreate(w, r, ddb, ddbTable)
		default:
			http.NotFound(w, r)
			return
		}
	})))

	log.Printf("Starting server on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("error serving: %s", err)
	}
}

func svcName() string {
	app, ok := os.LookupEnv("COPILOT_APPLICATION_NAME")
	if !ok {
		return "movies"
	}

	env, ok := os.LookupEnv("COPILOT_ENVIRONMENT_NAME")
	if !ok {
		return "movies"
	}

	svc, ok := os.LookupEnv("COPILOT_SERVICE_NAME")
	if !ok {
		return "movies"
	}

	return fmt.Sprintf("%s-%s-%s", app, env, svc)
}

func handleMovieCreate(w http.ResponseWriter, r *http.Request, ddb *dynamodb.DynamoDB, ddbTable string) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	dec := json.NewDecoder(r.Body)

	var movie Movie
	if err := dec.Decode(&movie); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	movie.ID = ksuid.New().String()
	log.Printf("Creating movie %+v", movie)

	av, err := dynamodbattribute.MarshalMap(movie)
	if err != nil {
		http.Error(w, fmt.Sprintf("marshal movie: %s", err), http.StatusBadRequest)
		return
	}

	input := &dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(ddbTable),
	}

	_, err = ddb.PutItemWithContext(ctx, input)
	if err != nil {
		http.Error(w, fmt.Sprintf("put item: %s", err), http.StatusBadRequest)
		return
	}

	log.Printf("Created movie %q", movie.ID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(movie); err != nil {
		log.Printf("error encoding movie %q: %s", movie.ID, err)
	}
}

func handleMovieGet(w http.ResponseWriter, r *http.Request, ddb *dynamodb.DynamoDB, ddbTable string) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	id := r.URL.Query().Get("id")
	log.Printf("Getting movie %q", id)

	result, err := ddb.GetItemWithContext(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(ddbTable),
		Key: map[string]*dynamodb.AttributeValue{
			"id": {
				S: aws.String(id),
			},
		},
	})
	switch {
	case err != nil:
		http.Error(w, fmt.Sprintf("get item: %s", err), http.StatusInternalServerError)
		return
	case result.Item == nil:
		w.WriteHeader(http.StatusNotFound)
		return
	}

	var movie Movie
	if err := dynamodbattribute.UnmarshalMap(result.Item, &movie); err != nil {
		http.Error(w, fmt.Sprintf("unmarshal result: %s", err), http.StatusInternalServerError)
		return
	}

	log.Printf("Got movie %+v", movie)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(movie); err != nil {
		log.Printf("error encoding movie %q: %s", movie.ID, err)
	}
}
