package handlers

import (
	"encoding/json"
	"net/http"
	"worklog/db"
	"worklog/llm"
)

var settingKeys = []string{
	llm.SettingProviderType,
	llm.SettingAPIKey,
	llm.SettingBaseURL,
	llm.SettingModel,
}

func HandleSettings(pm *llm.ProviderManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			cfg, err := db.GetAllSettings(settingKeys)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			// mask API key for frontend display
			if k := cfg[llm.SettingAPIKey]; len(k) > 8 {
				cfg[llm.SettingAPIKey] = k[:4] + "****" + k[len(k)-4:]
			}
			json.NewEncoder(w).Encode(cfg)

		case http.MethodPost:
			var body struct {
				ProviderType string `json:"provider_type"`
				APIKey       string `json:"api_key"`
				BaseURL      string `json:"base_url"`
				Model        string `json:"model"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, "invalid json", http.StatusBadRequest)
				return
			}
			if body.ProviderType == "" || body.APIKey == "" {
				http.Error(w, "provider_type and api_key required", http.StatusBadRequest)
				return
			}

			// If API key looks like a masked value from GET, keep old key
			if len(body.APIKey) > 4 && body.APIKey[4:8] == "****" {
				old, _ := db.GetSetting(llm.SettingAPIKey)
				if old != "" {
					body.APIKey = old
				}
			}

			saves := map[string]string{
				llm.SettingProviderType: body.ProviderType,
				llm.SettingAPIKey:       body.APIKey,
				llm.SettingBaseURL:      body.BaseURL,
				llm.SettingModel:        body.Model,
			}
			for k, v := range saves {
				if err := db.SetSetting(k, v); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			}

			// hot-reload provider
			if err := pm.Reload(); err != nil {
				http.Error(w, "saved but provider init failed: "+err.Error(), http.StatusUnprocessableEntity)
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}
