package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/gorilla/mux"
	loggly "github.com/jamespearly/loggly"
)

// get the number of records
func status(w http.ResponseWriter, r *http.Request, db *dynamodb.Client) {
	table := "bchauder_CoinbasePrices"
	count := 0
	input := &dynamodb.ScanInput{
		TableName: &table,
		Select: types.SelectCount,
	}
	for {
		result, err := db.Scan(r.Context(), input)
		if err != nil {
			http.Error(w, "Database error", 500)
			return
		}
		count += int(result.Count)
		if len(result.LastEvaluatedKey) == 0 {
			break
		}
		input.ExclusiveStartKey = result.LastEvaluatedKey
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"table": table,
		"recordCount": count,
	})
}

// get all the records
func all(w http.ResponseWriter, r *http.Request, db *dynamodb.Client) {
	table := "bchauder_CoinbasePrices"
	records := []map[string]interface{}{}
	input := &dynamodb.ScanInput{
		TableName: &table,
	}
	for {
		result, err := db.Scan(r.Context(), input)
		if err != nil {
			http.Error(w, "Database error", 500)
			return
		}
		for _, item := range result.Items {
			record := map[string]interface{}{}
			err := attributevalue.UnmarshalMap(item, &record)
			if err != nil {
				http.Error(w, "Error reading record", 500)
				return
			}
			records = append(records, record)
		}
		if len(result.LastEvaluatedKey) == 0 {
			break
		}
		input.ExclusiveStartKey = result.LastEvaluatedKey
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(records)
}

// find one record using pollId
func search(w http.ResponseWriter, r *http.Request, db *dynamodb.Client) {
	id := r.URL.Query().Get("pollId")
	// check the input
	if id == "" {
		http.Error(w, "Missing pollId", 400)
		return
	}
	for _, c := range id {
		if c < '0' || c > '9' {
			http.Error(w, "Invalid pollId", 400)
			return
		}
	}
	table := "bchauder_CoinbasePrices"
	result, err := db.GetItem(r.Context(), &dynamodb.GetItemInput{
		TableName: &table,
		Key: map[string]types.AttributeValue{
			"pollId": &types.AttributeValueMemberS{Value: id},
		},
	})
	if err != nil {
		http.Error(w, "Database error", 500)
		return
	}
	if len(result.Item) == 0 {
		http.Error(w, "Record not found", 404)
		return
	}
	record := map[string]interface{}{}
	err = attributevalue.UnmarshalMap(result.Item, &record)
	if err != nil {
		http.Error(w, "Error reading record", 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(record)
}

// handle requests and log them
func handleRequest(w http.ResponseWriter, r *http.Request, db *dynamodb.Client, client *loggly.ClientType) {
	code := 200
	if r.Method != "GET" {
		code = 405
		w.Header().Set("Allow", "GET")
		http.Error(w, "Method not allowed", code)
	} else if r.URL.Path == "/bchauder/status" {
		status(w, r, db)
	} else if r.URL.Path == "/bchauder/all" {
		all(w, r, db)
	} else if r.URL.Path == "/bchauder/search" {
		search(w, r, db)
	} else {
		code = 404
		http.Error(w, "Not found", code)
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	message := fmt.Sprintf("Method: %s IP: %s Path: %s Status: %d",
		r.Method, ip, r.URL.Path, code)
	fmt.Println(message)
	client.Send("info", message)
}

func main() {
	// connect to AWS
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion("us-east-2"))
	if err != nil {
		fmt.Println("AWS error:", err)
		os.Exit(1)
	}
	db := dynamodb.NewFromConfig(cfg)
	// set up Loggly
	client := loggly.New("GoServer")
	// set up the router
	router := mux.NewRouter()
	router.PathPrefix("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleRequest(w, r, db, client)
	})
	fmt.Println("Server running on port 8080")
	http.ListenAndServe(":8080", router)
}
