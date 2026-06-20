package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
	"worklog/db"
	"worklog/llm"
)

func HandlePlans(provider llm.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		date := r.URL.Query().Get("date")
		if date == "" {
			date = time.Now().Format("2006-01-02")
		}

		switch r.Method {
		case http.MethodGet:
			plan, err := db.GetDailyPlan(date)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if plan == nil {
				json.NewEncoder(w).Encode(map[string]interface{}{"date": date, "tasks": nil, "confirmed": false})
				return
			}
			json.NewEncoder(w).Encode(plan)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func HandleGeneratePlan(provider llm.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		date := r.URL.Query().Get("date")
		if date == "" {
			date = time.Now().Format("2006-01-02")
		}

		goals, err := db.ListGoals()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		recentReports, err := db.RecentDailyReports(3)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		userPrompt := llm.PlanningUserPrompt(goals, recentReports, date)
		var fullContent strings.Builder

		err = provider.StreamChat(r.Context(), llm.PlanningSystemPrompt(), []llm.Message{
			{Role: "user", Content: userPrompt},
		}, func(chunk string) {
			fullContent.WriteString(chunk)
			fmt.Fprintf(w, "data: %s\n\n", sseData(chunk))
			flusher.Flush()
		})

		if err != nil {
			fmt.Fprintf(w, "data: [ERROR] %s\n\n", err.Error())
			flusher.Flush()
			return
		}

		// save plan
		_ = db.UpsertDailyPlan(date, fullContent.String())
		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	}
}

// HandleSavePlan saves plan text directly (e.g., generated via mentor chat).
func HandleSavePlan() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			Date  string `json:"date"`
			Tasks string `json:"tasks"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		if body.Date == "" {
			body.Date = time.Now().Format("2006-01-02")
		}
		if err := db.UpsertDailyPlan(body.Date, body.Tasks); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

func HandleConfirmPlan() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		date := r.URL.Query().Get("date")
		if date == "" {
			http.Error(w, "date required", http.StatusBadRequest)
			return
		}
		if err := db.ConfirmDailyPlan(date); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "confirmed"})
	}
}

// sseData encodes s as a JSON string (with quotes) so the frontend can
// JSON.parse it back, correctly restoring newlines and special characters.
func sseData(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
