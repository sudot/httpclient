package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		bytes, _ := json.Marshal(map[string]string{
			"date": time.Now().String(),
		})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(bytes)
	})
	log.Fatal(http.ListenAndServe(":8080", nil))
}
