package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/gin-gonic/gin"
	"github.com/segmentio/ksuid"
)

type Movie struct {
	ID    string `json:"id"`
	Title string `json:"title" binding:"required"`
	Year  int    `json:"year" binding:"required"`
}

func main() {
	table, ok := os.LookupEnv("MOVIES_NAME")
	if !ok {
		log.Fatalf("MOVIES_NAME is not set")
	}

	ddbTable := aws.String(table)
	log.Printf("Using %q as the movies table", table)

	r := gin.Default()

	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))

	// Create DynamoDB client
	ddb := dynamodb.New(sess)

	r.GET("/healthz", func(c *gin.Context) {
		c.Status(200)
	})

	r.POST("/api/v0/movie", func(c *gin.Context) {
		var movie Movie
		if err := c.ShouldBindJSON(&movie); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("bind: %s", err),
			})
			return
		}

		movie.ID = ksuid.New().String()
		log.Printf("Creating movie %+v", movie)

		av, err := dynamodbattribute.MarshalMap(movie)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("marshal movie: %s", err),
			})
			return
		}

		input := &dynamodb.PutItemInput{
			Item:      av,
			TableName: ddbTable,
		}

		_, err = ddb.PutItem(input)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("put item: %s", err),
			})
			return
		}

		log.Printf("Created movie %q", movie.ID)
		c.JSON(200, movie)
	})

	r.GET("/api/v0/movie", func(c *gin.Context) {
		id := c.Query("id")
		log.Printf("Getting movie %q", id)

		result, err := ddb.GetItem(&dynamodb.GetItemInput{
			TableName: ddbTable,
			Key: map[string]*dynamodb.AttributeValue{
				"id": {
					S: aws.String(id),
				},
			},
		})
		switch {
		case err != nil:
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("get item: %s", err),
			})
			return
		case result.Item == nil:
			c.Status(http.StatusNotFound)
			return
		}

		var movie Movie
		if err := dynamodbattribute.UnmarshalMap(result.Item, &movie); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("unmarshal result: %s", err),
			})
			return
		}

		log.Printf("Got movie %+v", movie)
		c.JSON(200, movie)
	})

	r.Run(":8080")
}
