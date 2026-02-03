package main

import (
	"net/http"
	"os"
	"time"
)

func main() {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://localhost:8080/healthz")
	if err != nil {
		os.Exit(1)
	}
	if resp.StatusCode != http.StatusOK {
		os.Exit(1)
	}
}
