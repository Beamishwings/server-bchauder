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

		// add the records from this scan
        recordsFound := int(result.Count)
        count = count + recordsFound

		//check if there are anymore records
		if len(result.LastEvaluatedKey) > 0 {
			input.ExclusiveStartKey = result.LastEvaluatedKey
		} else {
			break
		}
	}

	//make the response
	response := make(map[string]interface{})
	response["table"] = table
	response["recordCount"] = count

	//now send the json response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}


// get all the records
func all(w http.ResponseWriter, r *http.Request, db *dynamodb.Client) {
	table := "bchauder_CoinbasePrices"
	//make a list to store the records in
	records := []map[string]interface{}{}

	input := &dynamodb.ScanInput{
		TableName: &table,
	}


	for { //fetch the records from dynamodb
		result, err := db.Scan(r.Context(), input)

		if err != nil {
			http.Error(w, "Database error", 500)
			return
		}

		//look through ebvery single record that we reciebved
		for _, item := range result.Items {
			record := make(map[string]interface{}) //map out each record
			err := attributevalue.UnmarshalMap(item, &record) //then convert the dynamo into a go map

			if err != nil {
				http.Error(w, "Error reading record", 500)
				return
			}
			//now just add the record to the list
			records = append(records, record)
		}
		//check for any last records
		 if len(result.LastEvaluatedKey) > 0 {
			input.ExclusiveStartKey = result.LastEvaluatedKey
		 } else {
			break
		}
	}
	//now send alkl the records back to JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(records)
}

// find one record using pollId
func search(w http.ResponseWriter, r *http.Request, db *dynamodb.Client) {
	//get pollID from the URL
	id := r.URL.Query().Get("pollId")
	// check the input
	if id == "" {
		http.Error(w, "Missing pollId", 400)
		return
	}

	for _, c := range id { //ensure that the id is all numbers
		if c < '0' || c > '9' {
			http.Error(w, "Invalid pollId", 400)
			return
		}
	}

	table := "bchauder_CoinbasePrices"

	//make the key we are seraching for
	key := make(map[string]types.AttributeValue)
	key["pollId"] = &types.AttributeValueMemberS{Value: id}

	//make the dynamo request
	input := &dynamodb.GetItemInput{
		TableName: &table,
		Key: key,
	}

	//now saerch for the record
	result, err := db.GetItem(r.Context(), input)

	if err != nil {
		http.Error(w, "Database error", 500)
		return
	}

	//check to see if we found anything
	if len(result.Item) == 0 {
		http.Error(w, "Record not found", 404)
		return
	}

	//now make a map for the record
	record := make(map[string]interface{})

	//convert the dynamo
	err = attributevalue.UnmarshalMap(result.Item, &record)

	if err != nil {
		http.Error(w, "Error reading record", 500)
		return
	}

	//send the record back as a JSON
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

	router.HandleFunc("/bchauder/status", func(w http.ResponseWriter, r *http.Request) {
    	status(w, r, db)
	}).Methods("GET")

	router.HandleFunc("/bchauder/all", func(w http.ResponseWriter, r *http.Request) {
    	all(w, r, db)
	}).Methods("GET")

	router.HandleFunc("/bchauder/search", func(w http.ResponseWriter, r *http.Request) {
    	search(w, r, db)
	}).Methods("GET")

	fmt.Println("Server running on port 8080")
	http.ListenAndServe(":8080", router)
}
