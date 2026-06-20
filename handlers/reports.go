package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"worklog/db"
	"worklog/llm"
)

func HandleDailyReport(provider llm.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			month := r.URL.Query().Get("month")
			if month == "" {
				month = r.URL.Query().Get("date")
			}
			reports, err := db.ListDailyReports(month)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if reports == nil {
				reports = []*db.DailyReport{}
			}
			json.NewEncoder(w).Encode(reports)

		case http.MethodPost:
			var body struct {
				Date      string `json:"date"`
				Completed string `json:"completed"`
				Plan      string `json:"plan"`
				Issues    string `json:"issues"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, "invalid json", http.StatusBadRequest)
				return
			}
			if body.Date == "" {
				http.Error(w, "date required", http.StatusBadRequest)
				return
			}
			report, err := db.UpsertDailyReport(body.Date, body.Completed, body.Plan, body.Issues)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(report)

			// async memory update
			go func() {
				if err := updateMentorMemory(context.Background(), provider, report); err != nil {
					log.Printf("memory update error: %v", err)
				}
			}()

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func updateMentorMemory(ctx context.Context, provider llm.Provider, report *db.DailyReport) error {
	memory, err := db.GetMentorMemory()
	if err != nil {
		return err
	}
	prompt := llm.MemoryUpdatePrompt(report, memory)
	result, err := provider.Chat(ctx, "你是一个从工作日志中提取关键信息的分析器，请严格按要求输出 JSON。", []llm.Message{
		{Role: "user", Content: prompt},
	})
	if err != nil {
		return err
	}

	// extract JSON from result (strip markdown fences if present)
	result = strings.TrimSpace(result)
	if idx := strings.Index(result, "{"); idx > 0 {
		result = result[idx:]
	}
	if idx := strings.LastIndex(result, "}"); idx >= 0 {
		result = result[:idx+1]
	}

	var updated map[string]string
	if err := json.Unmarshal([]byte(result), &updated); err != nil {
		log.Printf("memory parse error: %v\nraw: %s", err, result)
		return nil
	}
	for k, v := range updated {
		if err := db.SetMentorMemoryKey(k, v); err != nil {
			log.Printf("set memory key %s: %v", k, err)
		}
	}
	log.Printf("mentor memory updated for report %s", report.Date)
	return nil
}

// HandleGenerateDailyReport converts raw work notes into structured 今日完成/明日计划/问题风险.
// HandleReportChat — SSE streaming chat for guided daily report writing.
func HandleReportChat(provider llm.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			Message string        `json:"message"`
			History []llm.Message `json:"history"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}

		// Load context
		goals, _ := db.ListGoals()
		yesterday, _ := db.YesterdayReport()
		todayPlan, _ := db.GetDailyPlan(r.URL.Query().Get("date"))

		var yPlan, tPlan string
		if yesterday != nil {
			yPlan = yesterday.Plan
		}
		if todayPlan != nil {
			tPlan = todayPlan.Tasks
		}

		system := llm.ReportChatSystem(goals, yPlan, tPlan)

		messages := append(body.History, llm.Message{Role: "user", Content: body.Message})

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		err := provider.StreamChat(r.Context(), system, messages, func(chunk string) {
			fmt.Fprintf(w, "data: %s\n\n", sseData(chunk))
			flusher.Flush()
		})
		if err != nil {
			fmt.Fprintf(w, "data: [ERROR] %s\n\n", err.Error())
			flusher.Flush()
			return
		}
		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	}
}

// HandleExtractReport — extracts structured report from conversation history.
func HandleExtractReport(provider llm.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			History []llm.Message `json:"history"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		if len(body.History) == 0 {
			http.Error(w, "history required", http.StatusBadRequest)
			return
		}

		result, err := provider.Chat(r.Context(), llm.ReportExtractSystem(), []llm.Message{
			{Role: "user", Content: llm.ReportExtractUser(body.History)},
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		result = strings.TrimSpace(result)
		if idx := strings.Index(result, "{"); idx > 0 {
			result = result[idx:]
		}
		if idx := strings.LastIndex(result, "}"); idx >= 0 {
			result = result[:idx+1]
		}
		var out struct {
			Completed string `json:"completed"`
			Plan      string `json:"plan"`
			Issues    string `json:"issues"`
		}
		if err := json.Unmarshal([]byte(result), &out); err != nil {
			http.Error(w, "parse error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(out)
	}
}

func HandleYesterdayReport() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		report, err := db.YesterdayReport()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if report == nil {
			json.NewEncoder(w).Encode(map[string]string{})
			return
		}
		json.NewEncoder(w).Encode(report)
	}
}

func HandleGenerateDailyReport(provider llm.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			Notes string `json:"notes"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Notes) == "" {
			http.Error(w, "notes required", http.StatusBadRequest)
			return
		}
		result, err := provider.Chat(r.Context(), llm.DailyReportGenerateSystem(), []llm.Message{
			{Role: "user", Content: llm.DailyReportGenerateUser(body.Notes)},
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// strip markdown fences if model wrapped output
		result = strings.TrimSpace(result)
		if idx := strings.Index(result, "{"); idx > 0 {
			result = result[idx:]
		}
		if idx := strings.LastIndex(result, "}"); idx >= 0 {
			result = result[:idx+1]
		}
		var out struct {
			Completed string `json:"completed"`
			Plan      string `json:"plan"`
			Issues    string `json:"issues"`
		}
		if err := json.Unmarshal([]byte(result), &out); err != nil {
			http.Error(w, "AI 返回格式错误: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(out)
	}
}

func HandleWeeklyReports(provider llm.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			reports, err := db.ListWeeklyReports(20)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if reports == nil {
				reports = []*db.WeeklyReport{}
			}
			json.NewEncoder(w).Encode(reports)

		case http.MethodPost:
			var body struct {
				WeekStart string `json:"week_start"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.WeekStart == "" {
				http.Error(w, "week_start required", http.StatusBadRequest)
				return
			}
			dailies, err := db.DailyReportsByWeek(body.WeekStart)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if len(dailies) == 0 {
				http.Error(w, "no daily reports found for this week", http.StatusBadRequest)
				return
			}
			content, err := provider.Chat(r.Context(), llm.WeeklyReportSystemPrompt(), []llm.Message{
				{Role: "user", Content: llm.WeeklyReportUserPrompt(dailies, body.WeekStart)},
			})
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if err := db.UpsertWeeklyReport(body.WeekStart, content, true); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"week_start": body.WeekStart, "content": content})

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func HandleMonthlyReports(provider llm.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			reports, err := db.ListMonthlyReports(12)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if reports == nil {
				reports = []*db.MonthlyReport{}
			}
			json.NewEncoder(w).Encode(reports)

		case http.MethodPost:
			var body struct {
				Month string `json:"month"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Month == "" {
				http.Error(w, "month required", http.StatusBadRequest)
				return
			}
			weeklies, err := db.WeeklyReportsByMonth(body.Month)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if len(weeklies) == 0 {
				http.Error(w, "no weekly reports found for this month", http.StatusBadRequest)
				return
			}
			content, err := provider.Chat(r.Context(), llm.MonthlyReportSystemPrompt(), []llm.Message{
				{Role: "user", Content: llm.MonthlyReportUserPrompt(weeklies, body.Month)},
			})
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if err := db.UpsertMonthlyReport(body.Month, content, true); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"month": body.Month, "content": content})

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}
