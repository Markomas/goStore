package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/blevesearch/bleve/v2"
	"github.com/dgraph-io/badger/v4"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	index   bleve.Index
	apiKey  string
	logfile *os.File
	db      *badger.DB
)

type Record struct {
	Key       string `json:"key"`
	Topic     string `json:"topic"`
	Content   string `json:"content"`
	UpdatedAt int64  `json:"updated_at"`
	CreatedAt int64  `json:"created_at"`
}

func main() {
	storagePathFlag := flag.String("storage", "storage", "Storage folder for index, db and log")
	indexPathFlag := flag.String("indexpath", "index.bleve", "Path to Bleve index directory")
	badgerFolder := flag.String("dbpath", "badger", "DB folder path")
	logPath := flag.String("logfile", "store.log", "Path to log file")
	importLog := flag.Bool("import-log", false, "Import and decompress log file on startup")
	apikeyFlag := flag.String("apikey", "demo", "API key required for all requests")
	flag.Parse()

	if *storagePathFlag != "" {
		err := os.MkdirAll(*storagePathFlag, 0755)
		if err != nil {
			log.Fatal(err)
			return
		}
		*indexPathFlag = filepath.Join(*storagePathFlag, "bleve.index")
		*badgerFolder = filepath.Join(*storagePathFlag, "badger.db")
		*logPath = filepath.Join(*storagePathFlag, "store.log")
	}

	apiKey = *apikeyFlag
	if apiKey == "" {
		log.Fatal("API key must be provided using -apikey")
	}
	log.Printf("API key: %s\n", apiKey)

	var err error
	logfile, err = os.OpenFile(*logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatal(err)
	}
	defer logfile.Close()

	// Open or create Bleve index
	index, err = bleve.Open(*indexPathFlag)
	if errors.Is(err, bleve.ErrorIndexPathDoesNotExist) {
		mapping := bleve.NewIndexMapping()

		docMapping := bleve.NewDocumentMapping()

		textFieldMapping := bleve.NewTextFieldMapping()
		docMapping.AddFieldMappingsAt("Topic", textFieldMapping)
		docMapping.AddFieldMappingsAt("Content", textFieldMapping)

		mapping.AddDocumentMapping("record", docMapping)

		index, err = bleve.New(*indexPathFlag, mapping)
	}
	if err != nil {
		log.Fatal(err)
	}
	defer index.Close()

	db, err = badger.Open(badger.DefaultOptions(*badgerFolder))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if *importLog {
		fmt.Printf("Importing from log file: %s\n", *logPath)
		if err := readAndDecompressLogFile(*logPath); err != nil {
			log.Printf("Error importing log file: %v\n", err)
		}
	}

	http.HandleFunc("/add", withAPIKeyAuth(addHandler))
	http.HandleFunc("/get", withAPIKeyAuth(getHandler))
	http.HandleFunc("/search", withAPIKeyAuth(searchHandler))

	fmt.Println("Listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func readAndDecompressLogFile(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	const workerCount = 8
	dataChan := make(chan []byte, 100)
	errChan := make(chan error, workerCount)
	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for data := range dataChan {
				if err := saveToDB(data); err != nil {
					fmt.Println("Save to DB error:", err)
					errChan <- err
				}
			}
		}()
	}

	scanner := bufio.NewScanner(file)
	count := 0
	for scanner.Scan() {
		count++
		if count%100 == 0 {
			fmt.Printf("Processed %d records\n", count)
		}
		b64line := scanner.Text()
		if len(b64line) == 0 {
			continue
		}
		gzData, err := base64.StdEncoding.DecodeString(b64line)
		if err != nil {
			fmt.Println("Base64 decode error:", err)
			continue
		}
		zr, err := gzip.NewReader(bytes.NewReader(gzData))
		if err != nil {
			fmt.Println("Gzip decompress error:", err)
			continue
		}
		var out bytes.Buffer
		_, err = out.ReadFrom(zr)
		zr.Close()
		if err != nil {
			fmt.Println("Read decompressed data error:", err)
			continue
		}

		dataChan <- out.Bytes()
	}

	close(dataChan)
	wg.Wait()

	if err := scanner.Err(); err != nil {
		return err
	}

	close(errChan)
	if len(errChan) > 0 {
		return fmt.Errorf("some records failed to save")
	}

	return nil
}

func getHandler(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "Missing key param", http.StatusBadRequest)
		return
	}

	topic := r.URL.Query().Get("topic")
	if topic == "" {
		http.Error(w, "Missing topic param", http.StatusBadRequest)
		return
	}

	keyParam := fmt.Sprintf("%s:%s", key, topic)

	var rec Record

	err := db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(keyParam))
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &rec)
		})
	})

	if err == badger.ErrKeyNotFound {
		http.Error(w, "Record not found", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, "Internal error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(rec); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func withAPIKeyAuth(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("X-API-Key")
		if key != apiKey {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		handler(w, r)
	}
}

func addHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Cannot read body", http.StatusBadRequest)
		return
	}

	logToFile(body)
	err = saveToDB(body)
	if err != nil {
		http.Error(w, "Internal error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func saveToDB(body []byte) error {
	var rec Record
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&rec); err != nil {
		return err
	}
	key := fmt.Sprintf("%s:%s", rec.Key, rec.Topic)

	now := time.Now().Unix()
	existing := false
	var oldRecord Record

	// Check if record exists
	err := db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err == badger.ErrKeyNotFound {
			return nil // Not an error — just no existing record
		}
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &oldRecord)
		})
	})
	if err != nil {
		return err
	}

	if oldRecord.Key != "" {
		existing = true
	}

	if existing {
		rec.CreatedAt = oldRecord.CreatedAt
	} else {
		rec.CreatedAt = now
	}
	rec.UpdatedAt = now

	// Save to DB
	data, err := json.Marshal(rec)
	if err != nil {
		return err
	}

	err = db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(key), data)
	})
	if err != nil {
		return err
	}

	// Index to Bleve
	err = index.Index(key, map[string]interface{}{
		"Key":     rec.Key,
		"Topic":   rec.Topic,
		"Content": rec.Content,
	})
	if err != nil {
		return err
	}
	return nil
}

func logToFile(body []byte) {
	go func(data []byte) {
		var buf bytes.Buffer
		zw := gzip.NewWriter(&buf)
		_, err := zw.Write(data)
		if err != nil {
			log.Println("gzip error:", err)
			return
		}
		zw.Close()

		encoded := base64.StdEncoding.EncodeToString(buf.Bytes())
		logfile.WriteString(encoded + "\n")
	}(body)
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	topic := r.URL.Query().Get("topic")
	queryText := r.URL.Query().Get("q")

	topicQuery := bleve.NewMatchQuery(topic)
	topicQuery.SetField("Topic")

	contentQuery := bleve.NewMatchQuery(queryText)
	contentQuery.SetField("Content")
	contentQuery.SetFuzziness(2)

	query := bleve.NewConjunctionQuery(topicQuery, contentQuery)

	searchReq := bleve.NewSearchRequest(query)
	searchReq.Size = 10

	searchRes, err := index.Search(searchReq)
	if err != nil {
		http.Error(w, "Search failed", http.StatusInternalServerError)
		return
	}

	results := []Record{}
	for _, hit := range searchRes.Hits {
		err := db.View(func(txn *badger.Txn) error {
			item, err := txn.Get([]byte(hit.ID))
			if err != nil {
				return nil
			}
			return item.Value(func(val []byte) error {
				var rec Record
				if err := json.Unmarshal(val, &rec); err == nil && strings.EqualFold(rec.Topic, topic) {
					results = append(results, rec)
				}
				return nil
			})
		})
		if err != nil {
			continue
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}
