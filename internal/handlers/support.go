package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/alicenstar/astrid/internal/license"
)

type SupportHandler struct {
	sdkURL     string
	appVersion string
	tmpl       *Templates
	client     *http.Client
}

func NewSupportHandler(sdkURL, appVersion string, tmpl *Templates) *SupportHandler {
	return &SupportHandler{
		sdkURL:     sdkURL,
		appVersion: appVersion,
		tmpl:       tmpl,
		client:     &http.Client{Timeout: 5 * time.Minute},
	}
}

func (h *SupportHandler) Page(w http.ResponseWriter, r *http.Request) {
	ls := license.GetStatus(r)
	data := map[string]any{
		"Title":           "Support",
		"ActiveNav":       "support",
		"AppVersion":      h.appVersion,
		"LicenseExpired":  ls.Expired,
		"UpdateAvailable": ls.UpdateAvailable,
		"UpdateVersion":   ls.UpdateVersion,
	}
	h.tmpl.Render(w, "support", withUserEmail(r, data))
}

func (h *SupportHandler) GenerateBundle(w http.ResponseWriter, r *http.Request) {
	if h.sdkURL == "" {
		http.Error(w, "Replicated SDK not configured", http.StatusServiceUnavailable)
		return
	}

	log.Println("Triggering support bundle generation via Replicated SDK...")

	req, err := http.NewRequest("POST", h.sdkURL+"/api/v1/app/supportbundle/generate", nil)
	if err != nil {
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		log.Printf("ERROR: support bundle generation failed: %v", err)
		http.Error(w, "Failed to generate support bundle", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 300 {
		log.Printf("ERROR: SDK returned %d: %s", resp.StatusCode, string(body))
		http.Error(w, fmt.Sprintf("SDK error: %d", resp.StatusCode), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"message": "Support bundle generated and uploaded to Vendor Portal",
	})
}
