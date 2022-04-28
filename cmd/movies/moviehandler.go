package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/dannyrandall/movies/internal/movies"
	"github.com/dannyrandall/movies/internal/otel"
	"go.opentelemetry.io/otel/trace"
)

type MovieHandler struct {
	Dynamo      *dynamodb.Client
	MoviesTable string
}

func (m *MovieHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	span := trace.SpanFromContext(r.Context())

	log := log.New(os.Stderr, fmt.Sprintf("AWS-XRAY-TRACE-ID: %s - ", otel.XRayTraceID(span)), log.LstdFlags|log.Lmsgprefix)
	log.Printf("Handling request: %s %s", r.Method, r.URL.String())

	switch r.Method {
	case http.MethodGet:
		m.getMovie(log, w, r)
	case http.MethodPost:
		m.createMovie(log, w, r)
	default:
		http.NotFound(w, r)
	}
}

func (m *MovieHandler) getMovie(log *log.Logger, w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	id := r.URL.Query().Get("id")
	log.Printf("Getting movie %q", id)

	result, err := m.Dynamo.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(m.MoviesTable),
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberS{Value: id},
		},
	})
	switch {
	case err != nil:
		httpError(w, http.StatusInternalServerError, log, "get item: %s", err)
		return
	case result.Item == nil:
		httpError(w, http.StatusNotFound, log, "no movie found with id %q", id)
		return
	}

	var movie movies.Movie
	if err := attributevalue.UnmarshalMap(result.Item, &movie); err != nil {
		httpError(w, http.StatusInternalServerError, log, "unmarshal result: %s", err)
		return
	}

	log.Printf("Got movie %+v", movie)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(movie); err != nil {
		log.Printf("error encoding movie %q: %s", movie.ID, err)
	}
}

func (m *MovieHandler) createMovie(log *log.Logger, w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	dec := json.NewDecoder(r.Body)

	var movie movies.Movie
	if err := dec.Decode(&movie); err != nil {
		httpError(w, http.StatusBadRequest, log, "decode movie %s", err)
		return
	}

	movie.ID = movies.NewID()
	log.Printf("Creating movie %+v", movie)

	av, err := attributevalue.MarshalMap(movie)
	if err != nil {
		httpError(w, http.StatusBadRequest, log, "marshal movie: %s", err)
		return
	}

	input := &dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(m.MoviesTable),
	}

	_, err = m.Dynamo.PutItem(ctx, input)
	if err != nil {
		httpError(w, http.StatusBadRequest, log, "put item: %s", err)
		return
	}

	log.Printf("Created movie %q", movie.ID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(movie); err != nil {
		log.Printf("error encoding movie %q: %s", movie.ID, err)
	}
}

func httpError(w http.ResponseWriter, code int, log *log.Logger, format string, a ...any) {
	str := fmt.Sprintf(format, a...)
	http.Error(w, str, code)
	log.Printf("returning error: %s", str)
}
