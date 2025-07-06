package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

type Record struct {
	Key       string `json:"key"`
	Topic     string `json:"topic"`
	Content   string `json:"content"`
	UpdatedAt int64  `json:"updated_at"`
	CreatedAt int64  `json:"created_at"`
}

var db *sql.DB
var logfile *os.File
var apiKey string

func main() {
	// Command-line flags
	dbPath := flag.String("db", "./data.db", "Path to SQLite database file")
	logPath := flag.String("logfile", "store.log", "Path to log file")
	importLog := flag.Bool("import-log", false, "Import and decompress log file on startup")
	apikeyFlag := flag.String("apikey", "demo", "API key required for all requests")
	flag.Parse()
	apiKey = *apikeyFlag
	if apiKey == "" {
		log.Fatal("API key must be provided using -apikey")
	}
	log.Printf("API key: %s\n", apiKey)

	flag.Parse()

	var err error
	db, err = sql.Open("sqlite3", *dbPath)
	if err != nil {
		log.Fatal(err)
	}

	// Initialize table if not exists
	_, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS "records"
		(
			id      INTEGER
			primary key autoincrement,
			key     TEXT,
			topic     TEXT,
			content TEXT,
			updated_at integer,
			created_at integer not null
		);
		
		CREATE UNIQUE INDEX IF NOT EXISTS key_index
			on records (key, topic);
       CREATE VIRTUAL TABLE IF NOT EXISTS records_fts USING fts5(
			    key,
				topic,
				content,
				content='records',
				content_rowid='id',
				prefix='2 3 4'
		);
		-- Insert trigger
		CREATE TRIGGER IF NOT EXISTS records_ai AFTER INSERT ON records BEGIN
		  INSERT INTO records_fts(rowid, key, topic, content)
		  VALUES (new.id, new.key, new.topic, new.content);
		END;
		
		-- Delete trigger
		CREATE TRIGGER IF NOT EXISTS records_ad AFTER DELETE ON records BEGIN
		  DELETE FROM records_fts WHERE rowid = old.id;
		END;
		
		-- Update trigger
		CREATE TRIGGER IF NOT EXISTS records_au AFTER UPDATE ON records BEGIN
		  UPDATE records_fts SET key = new.key, topic = new.topic, content = new.content
		  WHERE rowid = old.id;
		END;


	`)
	if err != nil {
		log.Fatal(err)
	}

	if *importLog {
		fmt.Printf("Importing from log file: %s\n", *logPath)
		if err := readAndDecompressLogFile(*logPath); err != nil {
			log.Printf("Error importing log file: %v\n", err)
		}
	}

	logfile, err = os.OpenFile(*logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatal(err)
	}
	defer logfile.Close()

	http.HandleFunc("/add", withAPIKeyAuth(addHandler))
	http.HandleFunc("/get-by-key", withAPIKeyAuth(getHandler))
	http.HandleFunc("/search", withAPIKeyAuth(searchHandler))

	fmt.Println("Listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
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
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	log.Println("Received add request:", string(body))
	var rec Record
	if err := json.NewDecoder(r.Body).Decode(&rec); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	err = saveJsonToDB(rec)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	logToFile(body)

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func saveJsonToDB(rec Record) error {
	_, err := db.Exec(`
        INSERT INTO records (key, topic, content, created_at, updated_at) VALUES (?,?,?,?,?)
        ON CONFLICT(key, topic) DO UPDATE SET content=excluded.content, updated_at=excluded.updated_at;
    `, rec.Key, rec.Topic, rec.Content, time.Now().Unix(), time.Now().Unix())
	return err
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

	var rec Record
	err := db.QueryRow("SELECT key, topic, content, updated_at, created_at FROM records WHERE key=? and topic=?", key, topic).Scan(&rec.Key, &rec.Topic, &rec.Content, &rec.UpdatedAt, &rec.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(rec)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		http.Error(w, "Missing 'q' param", http.StatusBadRequest)
		return
	}
	topic := r.URL.Query().Get("topic")
	if topic == "" {
		http.Error(w, "Missing topic param", http.StatusBadRequest)
		return
	}

	// Get pagination params
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := 20 // default
	offset := 0 // default

	if limitStr != "" {
		if val, err := strconv.Atoi(limitStr); err == nil && val > 0 && val <= 100 {
			limit = val
		}
	}
	if offsetStr != "" {
		if val, err := strconv.Atoi(offsetStr); err == nil && val >= 0 {
			offset = val
		}
	}

	query := `SELECT r.key, r.topic, r.content, r.updated_at, r.created_at
FROM records r
JOIN records_fts fts ON r.id = fts.rowid
WHERE fts.content MATCH ? and r.topic=? LIMIT? OFFSET?`
	rows, err := db.Query(query, q+"*", topic, limit, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	results := []Record{}
	for rows.Next() {
		var rec Record
		if err := rows.Scan(&rec.Key, &rec.Topic, &rec.Content, &rec.UpdatedAt, &rec.CreatedAt); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		results = append(results, rec)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(results); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func readAndDecompressLogFile(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
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

		var rec Record
		err = json.Unmarshal(out.Bytes(), &rec)
		if err != nil {
			fmt.Println("Unmarshal JSON error:", err)
			continue
		}

		err = saveJsonToDB(rec)
		if err != nil {
			fmt.Println("Save to DB error:", err)
			continue
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
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
