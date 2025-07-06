package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"sync"
)

func main() {
	const (
		url   = "http://localhost:8080/add"
		total = 1000 // number of requests
		conc  = 10   // number of concurrent goroutines
	)

	var wg sync.WaitGroup
	sem := make(chan struct{}, conc)

	for i := 0; i < total; i++ {
		wg.Add(1)
		sem <- struct{}{} // limit concurrency
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()

			payload := map[string]string{
				"key":     "bench-key-" + strconv.Itoa(i),
				"content": "benchmark content " + strconv.Itoa(i),
			}
			body, _ := json.Marshal(payload)
			resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
			if err != nil {
				log.Printf("Error on request %d: %v\n", i, err)
				return
			}
			resp.Body.Close()
		}(i)
	}
	wg.Wait()
	log.Println("Benchmark complete!")
}
