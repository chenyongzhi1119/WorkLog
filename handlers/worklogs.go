package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"worklog/db"
)

func HandleWorkLogs() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		date := r.URL.Query().Get("date")
		if date == "" {
			date = time.Now().Format("2006-01-02")
		}
		switch r.Method {
		case http.MethodGet:
			logs, err := db.ListWorkLogs(date)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if logs == nil {
				logs = []*db.WorkLog{}
			}
			json.NewEncoder(w).Encode(logs)

		case http.MethodPost:
			var body struct {
				Content string `json:"content"`
				Date    string `json:"date"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, "invalid json", http.StatusBadRequest)
				return
			}
			if strings.TrimSpace(body.Content) == "" {
				http.Error(w, "content required", http.StatusBadRequest)
				return
			}
			if body.Date == "" {
				body.Date = date
			}
			log, err := db.AddWorkLog(body.Date, body.Content)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(log)

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func HandleWorkLog() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) < 3 {
			http.Error(w, "missing id", http.StatusBadRequest)
			return
		}
		id, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		if err := db.DeleteWorkLog(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
	}
}
