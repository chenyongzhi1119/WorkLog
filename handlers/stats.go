package handlers

import (
	"encoding/json"
	"net/http"
	"time"
	"worklog/db"
)

func HandleStats() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		month := r.URL.Query().Get("month")
		if month == "" {
			month = time.Now().Format("2006-01")
		}
		// Compute Monday of current week
		now := time.Now()
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		weekStart := now.AddDate(0, 0, -(weekday - 1)).Format("2006-01-02")

		s, err := db.GetStats(month, weekStart)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Add goals
		goals, _ := db.ListGoals()
		type resp struct {
			*db.StatsData
			Goals []*db.Goal `json:"goals"`
		}
		json.NewEncoder(w).Encode(resp{s, goals})
	}
}
