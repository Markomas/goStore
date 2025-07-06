package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

type Record struct {
	Key     string `json:"key"`
	Topic   string `json:"topic"`
	Content string `json:"content"`
}

const (
	url     = "http://localhost:8080/add"
	apiKey  = "demo"
	numReqs = 10000 // total number of requests
	workers = 50    // concurrent workers
)

func randomWord(n int) string {
	chars := "abcdefghijklmnopqrstuvwxyz"
	result := make([]byte, n)
	for i := range result {
		result[i] = chars[rand.Intn(len(chars))]
	}
	return string(result)
}

func randomContent(size int) string {
	words := make([]string, size)
	for i := range words {
		words[i] = randomWord(5 + rand.Intn(5))
	}
	return fmt.Sprintf("This is about %s", words)
}

func sendRequest(id int, wg *sync.WaitGroup, successCount *int, mutex *sync.Mutex) {
	defer wg.Done()

	rec := Record{
		Key:     fmt.Sprintf("key_%d", id),
		Topic:   fmt.Sprintf("topic_%d", rand.Intn(1000)),
		Content: randomContent(500), // approx 10KB
	}

	body, _ := json.Marshal(rec)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err == nil && resp.StatusCode == http.StatusOK {
		mutex.Lock()
		*successCount++
		mutex.Unlock()
	}
	if resp != nil {
		resp.Body.Close()
	}
}

func main() {
	rand.Seed(time.Now().UnixNano())

	var wg sync.WaitGroup
	var successCount int
	var mutex sync.Mutex

	start := time.Now()

	sem := make(chan struct{}, workers) // concurrency limit

	for i := 0; i < numReqs; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(id int) {
			defer func() { <-sem }()
			sendRequest(id, &wg, &successCount, &mutex)
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)

	fmt.Printf("Sent %d requests in %s\n", numReqs, elapsed)
	fmt.Printf("Successful: %d\n", successCount)
	fmt.Printf("RPS: %.2f\n", float64(successCount)/elapsed.Seconds())
}
