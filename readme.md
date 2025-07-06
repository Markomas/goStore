# goStore

[![Go](https://img.shields.io/badge/Go-1.17+-brightgreen)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

A lightweight key-value record store with HTTP API, backed by SQLite and append-only gzipped+base64 logs for durability and recovery.

## 📦 Features

- Simple JSON record storage (`key`, `content`, `created_at`, `updated_at`)
- Search by content with pagination
- Write-ahead logging (gzip + base64 encoded)
- Restore from log file on startup
- Configurable paths via CLI flags

---

## 🚀 Getting Started

### Clone & Build

```bash
git clone https://github.com/Markomas/goStore.git
cd goStore
go build -o goStore
```

---

## ⚙️ Usage

### Run with Defaults

```bash
./goStore
```

- Uses `./data.db` and `store.log`
- Serves on `http://localhost:8080`

### Command-Line Flags

| Flag            | Default       | Description                                      |
| --------------- | ------------- | ------------------------------------------------ |
| `--db`          | `./data.db`   | Path to SQLite database file                     |
| `--logfile`     | `store.log`   | Path to log file                                 |
| `--import-log`  | `false`       | Import & replay log file into database on start  |

### Examples

Custom DB and log file paths:

```bash
./goStore --db=mydata.db --logfile=mylog.log
```

Restore from log file:

```bash
./goStore --import-log
```

Combine all:

```bash
./goStore --db=prod.db --logfile=backup.log --import-log
```

---

## 📡 API Endpoints

### `POST /add`

Store a record.

**Request Body:**

```json
{
  "key": "example-key",
  "content": "some value"
}
```

---

### `GET /get-by-key?key=example-key`

Retrieve a record by key.

---

### `GET /search?q=term&limit=10&offset=0`

Search records by content.

- `q` (required): search term
- `limit`: max results (default: 20, max: 100)
- `offset`: result offset for pagination

---

## 🔄 Logging & Recovery

Each `POST /add` is logged to the file (`store.log`) as a gzip-compressed, base64-encoded JSON line.  
You can recover records from the log using:

```bash
./goStore --import-log
```

This performs an *upsert*, ensuring existing keys are updated.

---

## 📁 Project Structure

- `main.go`: Core server & database logic
- `store.log`: Append-only write-ahead log (auto-created)
- `data.db`: SQLite database (auto-created)

---

## 📄 License

[MIT](LICENSE)

---

## 🤝 Contributing

Issues and PRs welcome at [github.com/Markomas/goStore](https://github.com/Markomas/goStore)