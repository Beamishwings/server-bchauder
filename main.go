package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	loggly "github.com/jamespearly/loggly"
)

// send the current time back as JSON
func status(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	data := map[string]string{
		"time": time.Now().Format(time.RFC3339),
	}
	json.NewEncoder(w).Encode(data)
}

// handle requests to the server
func handleRequest(w http.ResponseWriter, r *http.Request, client *loggly.ClientType) {
	code := 200
	if r.Method != "GET" {
		code = 405
		w.Header().Set("Allow", "GET")
		http.Error(w, "Method Not Allowed", code)
	} else if r.URL.Path != "/bchauder/status" {
		code = 404
		http.Error(w, "Not Found", code)
	} else {
		status(w, r)
	}
	// get the IP address of the request
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	// send request information to Loggly
	message := fmt.Sprintf("Method: %s IP: %s Path: %s Status: %d",
		r.Method, ip, r.URL.Path, code)
	fmt.Println(message)
	err = client.Send("info", message)
	if err != nil {
		fmt.Println("Loggly error:", err)
	}
}

func main() {
	// Loggly
	tag := "GoServer"
	client := loggly.New(tag)

	// HTTP server
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		handleRequest(w, r, client)
	})
	fmt.Println("Server running on port 8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Println("Server error:", err)
	}
}
