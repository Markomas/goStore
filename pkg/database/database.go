package database

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"os"
	"store/pkg"
)

type Database struct {
	file *os.File
}

func (d *Database) Save(item pkg.Record) error {
	// 1. Marshal to JSON
	data, err := json.Marshal(item)
	if err != nil {
		return err
	}

	// 2. Compress with gzip
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(data); err != nil {
		gz.Close()
		return err
	}
	gz.Close()

	// 3. Encode to base64
	encoded := base64.StdEncoding.EncodeToString(buf.Bytes())

	// 4. Write base64 to file + newline
	if _, err := d.file.WriteString(encoded + "\n"); err != nil {
		return err
	}
	return nil
}

func NewUserDatabase(path string) *Database {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}

	return &Database{
		file: file,
	}
}

func (d *Database) Close() error {
	return d.file.Close()
}
