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

func HandleMentorStream(provider llm.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		message := r.URL.Query().Get("message")
		if strings.TrimSpace(message) == "" {
			http.Error(w, "message required", http.StatusBadRequest)
			return
		}

		memory, err := db.GetMentorMemory()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		goals, _ := db.ListGoals()
		yesterday, _ := db.YesterdayReport()
		todayPlan, _ := db.GetDailyPlan(time.Now().Format("2006-01-02"))
		profile, _ := db.GetAllSettings(llm.ProfileKeys)

		var yPlan, tPlan string
		if yesterday != nil { yPlan = yesterday.Plan }
		if todayPlan != nil { tPlan = todayPlan.Tasks }

		history, err := db.RecentConversations(30)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var messages []llm.Message
		for _, c := range history {
			messages = append(messages, llm.Message{Role: c.Role, Content: c.Content})
		}
		messages = append(messages, llm.Message{Role: "user", Content: message})

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		systemPrompt := llm.MentorSystemPrompt(memory, goals, yPlan, tPlan, profile)
		var fullReply strings.Builder

		err = provider.StreamChat(r.Context(), systemPrompt, messages, func(chunk string) {
			fullReply.WriteString(chunk)
			fmt.Fprintf(w, "data: %s\n\n", sseData(chunk))
			flusher.Flush()
		})

		if err != nil {
			fmt.Fprintf(w, "data: [ERROR] %s\n\n", err.Error())
			flusher.Flush()
			return
		}

		// persist conversation
		_ = db.AddConversation("user", message)
		_ = db.AddConversation("assistant", fullReply.String())

		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	}
}

func HandleMentorMemory() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		memory, err := db.GetMentorMemory()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(memory)
	}
}

func HandleMentorWeeklySummary(provider llm.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		reports, err := db.RecentDailyReports(7)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		goals, err := db.ListGoals()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		memory, err := db.GetMentorMemory()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		userMsg := llm.MentorWeeklySummaryUser(reports, goals, memory)
		var fullReply strings.Builder

		err = provider.StreamChat(r.Context(), llm.MentorWeeklySummarySystem(), []llm.Message{
			{Role: "user", Content: userMsg},
		}, func(chunk string) {
			fullReply.WriteString(chunk)
			fmt.Fprintf(w, "data: %s\n\n", sseData(chunk))
			flusher.Flush()
		})
		if err != nil {
			fmt.Fprintf(w, "data: [ERROR] %s\n\n", err.Error())
			flusher.Flush()
			return
		}
		_ = db.AddConversation("assistant", "【本周总结】\n\n"+fullReply.String())
		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	}
}

func HandleMentorConversations() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodDelete:
			if err := db.ClearConversations(); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"status": "cleared"})
		case http.MethodGet:
			convs, err := db.RecentConversations(50)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if convs == nil {
				convs = []*db.Conversation{}
			}
			json.NewEncoder(w).Encode(convs)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}
