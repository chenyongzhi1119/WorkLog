package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"worklog/db"
	"worklog/handlers"
	"worklog/llm"
)

//go:embed frontend
var frontendFS embed.FS

func main() {
	if err := db.Init("worklog.db"); err != nil {
		log.Fatalf("db init: %v", err)
	}

	// Build fallback provider from env var (used when DB has no settings yet)
	var fallback llm.Provider
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		fallback = llm.NewAnthropicProvider(key)
		log.Println("fallback: Anthropic from ANTHROPIC_API_KEY")
	}

	pm := llm.NewManager(fallback)
	if pm.Get() == nil {
		log.Println("warning: no provider configured — set one via the Settings panel")
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/api/settings", handlers.HandleSettings(pm))
	mux.HandleFunc("/api/profile", handlers.HandleProfile())
	mux.HandleFunc("/api/stats", handlers.HandleStats())

	mux.HandleFunc("/api/reports/daily", handlers.HandleDailyReport(pm))
	mux.HandleFunc("/api/reports/daily/generate", handlers.HandleGenerateDailyReport(pm))
	mux.HandleFunc("/api/reports/daily/chat", handlers.HandleReportChat(pm))
	mux.HandleFunc("/api/reports/daily/extract", handlers.HandleExtractReport(pm))
	mux.HandleFunc("/api/reports/yesterday", handlers.HandleYesterdayReport())
	mux.HandleFunc("/api/reports/weekly", handlers.HandleWeeklyReports(pm))
	mux.HandleFunc("/api/reports/monthly", handlers.HandleMonthlyReports(pm))

	mux.HandleFunc("/api/goals", handlers.HandleGoals())
	mux.HandleFunc("/api/goals/", handlers.HandleGoal())

	mux.HandleFunc("/api/plans", handlers.HandlePlans(pm))
	mux.HandleFunc("/api/plans/generate", handlers.HandleGeneratePlan(pm))
	mux.HandleFunc("/api/plans/save", handlers.HandleSavePlan())
	mux.HandleFunc("/api/plans/confirm", handlers.HandleConfirmPlan())

	mux.HandleFunc("/api/mentor/stream", handlers.HandleMentorStream(pm))
	mux.HandleFunc("/api/mentor/weekly-summary", handlers.HandleMentorWeeklySummary(pm))
	mux.HandleFunc("/api/mentor/memory", handlers.HandleMentorMemory())
	mux.HandleFunc("/api/mentor/conversations", handlers.HandleMentorConversations())

	mux.HandleFunc("/api/tasks", handlers.HandleDailyTasks())
	mux.HandleFunc("/api/tasks/", handlers.HandleDailyTask())

	mux.HandleFunc("/api/worklogs", handlers.HandleWorkLogs())
	mux.HandleFunc("/api/worklogs/", handlers.HandleWorkLog())

	sub, err := fs.Sub(frontendFS, "frontend")
	if err != nil {
		log.Fatal(err)
	}
	mux.Handle("/", noCache(http.FileServer(http.FS(sub))))

	addr := ":8090"
	log.Printf("WorkLog running at http://localhost%s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func noCache(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		h.ServeHTTP(w, r)
	})
}
