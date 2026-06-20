package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"worklog/db"
)

func HandleGoals() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			goals, err := db.ListGoals()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if goals == nil {
				goals = []*db.Goal{}
			}
			json.NewEncoder(w).Encode(goals)

		case http.MethodPost:
			var body struct {
				Title       string `json:"title"`
				Description string `json:"description"`
				Level       string `json:"level"`
				ParentID    *int64 `json:"parent_id"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, "invalid json", http.StatusBadRequest)
				return
			}
			if body.Title == "" || body.Level == "" {
				http.Error(w, "title and level required", http.StatusBadRequest)
				return
			}
			goal, err := db.CreateGoal(body.Title, body.Description, body.Level, body.ParentID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(goal)

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func HandleGoal() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// extract id from path: /api/goals/{id}
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
				Title       string `json:"title"`
				Description string `json:"description"`
				Status      string `json:"status"`
				Progress    *int   `json:"progress"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, "invalid json", http.StatusBadRequest)
				return
			}
			// Progress-only update
			if body.Progress != nil && body.Title == "" {
				if err := db.UpdateGoalProgress(id, *body.Progress); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
				return
			}
			if body.Status == "" {
				body.Status = "active"
			}
			if err := db.UpdateGoal(id, body.Title, body.Description, body.Status); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if body.Progress != nil {
				_ = db.UpdateGoalProgress(id, *body.Progress)
			}
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}
