package database

import (
	"encoding/json"
	"os"
	"store/pkg"
)

type Database struct {
	file *os.File
}

func (d Database) Save(item pkg.Record) error {
	enc := json.NewEncoder(d.file)
	if err := enc.Encode(item); err != nil {
		return err
	}
	// json.Encoder adds a newline after each Encode automatically!
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
