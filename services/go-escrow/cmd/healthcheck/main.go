package main

import (
	"net/http"
	"os"
)

func main() {
	resp, err := http.Get("http://localhost:8081/health")
	if err != nil {
		os.Exit(1)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		os.Exit(1)
	}
}
