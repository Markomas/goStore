package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	const (
		url    = "http://localhost:8080/add"
		apiKey = "demo"
		total  = 1000 // number of requests
		conc   = 10   // number of concurrent goroutines
	)

	var (
		wg      sync.WaitGroup
		sem     = make(chan struct{}, conc)
		success int64 // atomic counter for successful requests
	)
	client := &http.Client{}
	start := time.Now()

	for i := 0; i < total; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()

			payload := map[string]string{
				"key":     "bench-key-" + strconv.Itoa(i),
				"content": "benchmark content " + strconv.Itoa(i),
			}
			body, _ := json.Marshal(payload)

			req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
			if err != nil {
				log.Printf("Request creation failed (%d): %v\n", i, err)
				return
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-API-Key", apiKey)

			resp, err := client.Do(req)
			if err != nil {
				log.Printf("Error on request %d: %v\n", i, err)
				return
			}
			defer resp.Body.Close()

			bodyBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				log.Printf("Error reading response %d: %v\n", i, err)
				return
			}

			if resp.StatusCode != http.StatusOK {
				log.Printf("Non-200 response %d: %s\n", i, string(bodyBytes))
				return
			}

			atomic.AddInt64(&success, 1)
		}(i)
	}

	wg.Wait()
	duration := time.Since(start)
	rps := float64(success) / duration.Seconds()

	log.Printf("Benchmark complete!")
	log.Printf("Total duration: %s", duration)
	log.Printf("Successful requests: %d", success)
	log.Printf("Requests per second: %.2f", rps)
}
