package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"worklog/db"
)

func HandleDailyTasks() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		date := r.URL.Query().Get("date")
		if date == "" {
			date = time.Now().Format("2006-01-02")
		}
		switch r.Method {
		case http.MethodGet:
			tasks, err := db.ListDailyTasks(date)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if tasks == nil {
				tasks = []*db.DailyTask{}
			}
			json.NewEncoder(w).Encode(tasks)

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
			task, err := db.AddDailyTask(body.Date, body.Content)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(task)

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func HandleDailyTask() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
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

		switch r.Method {
		case http.MethodPut:
			var body struct {
				Done bool `json:"done"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, "invalid json", http.StatusBadRequest)
				return
			}
			if err := db.ToggleDailyTask(id, body.Done); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

		case http.MethodDelete:
			if err := db.DeleteDailyTask(id); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}
