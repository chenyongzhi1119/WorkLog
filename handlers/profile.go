package handlers

import (
	"encoding/json"
	"net/http"
	"worklog/db"
	"worklog/llm"
)

func HandleProfile() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			p, err := db.GetAllSettings(append(llm.ProfileKeys, "profile_reminder_time", "profile_notification"))
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode(p)

		case http.MethodPost:
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, "invalid json", http.StatusBadRequest)
				return
			}
			allowed := map[string]bool{
				"profile_name": true, "profile_role": true, "profile_team": true,
				"profile_stage": true, "profile_projects": true,
				"profile_reminder_time": true, "profile_notification": true,
			}
			for k, v := range body {
				if allowed[k] {
					if err := db.SetSetting(k, v); err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
				}
			}
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}
