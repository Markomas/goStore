package handler

import (
	"encoding/json"
	"net/http"
	"store/pkg"
	"time"
)

func (h *Handler) AddDefault(writer http.ResponseWriter, request *http.Request) {
	h.Add(writer, request)
}

func (h *Handler) Add(w http.ResponseWriter, r *http.Request) {
	// check if request is POST
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Only POST requests are allowed"))
		return
	}

	path := r.URL.Path
	prefix := "/add/"
	topic := "default"
	if len(path) > len(prefix) && path[:len(prefix)] == prefix {
		topic = path[len(prefix):]
	}

	// parse request body
	var item pkg.Record
	err := json.NewDecoder(r.Body).Decode(&item)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Invalid request body"))
		return
	}

	if item.UpdatedAt == 0 {
		item.UpdatedAt = time.Now().UnixNano() / 1e6
	}
	if item.CreatedAt == 0 {
		item.CreatedAt = time.Now().UnixNano() / 1e6
	}

	item.Topic = topic
	err = h.DB.Save(item)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Error saving item to database"))
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte("OK"))
}
