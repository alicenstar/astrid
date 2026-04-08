package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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

	log.Println("Generating support bundle...")

	outDir, err := os.MkdirTemp("", "support-bundle-*")
	if err != nil {
		log.Printf("ERROR: failed to create temp dir: %v", err)
		http.Error(w, "Failed to create temp directory", http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(outDir)

	outPath := filepath.Join(outDir, "bundle")
	cmd := exec.CommandContext(r.Context(), "support-bundle",
		"--load-cluster-specs",
		"--interactive=false",
		"-o", outPath,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		log.Printf("ERROR: support bundle generation failed: %v", err)
		http.Error(w, "Failed to generate support bundle", http.StatusInternalServerError)
		return
	}

	bundlePath := outPath + ".tar.gz"
	f, err := os.Open(bundlePath)
	if err != nil {
		log.Printf("ERROR: failed to open bundle at %s: %v", bundlePath, err)
		http.Error(w, "Bundle generated but file not found", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		log.Printf("ERROR: failed to stat bundle: %v", err)
		http.Error(w, "Bundle generated but could not be read", http.StatusInternalServerError)
		return
	}

	uploadURL := h.sdkURL + "/api/v1/supportbundle"
	req, err := http.NewRequest("POST", uploadURL, f)
	if err != nil {
		log.Printf("ERROR: failed to create upload request: %v", err)
		http.Error(w, "Failed to prepare upload", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/gzip")
	req.ContentLength = stat.Size()

	resp, err := h.client.Do(req)
	if err != nil {
		log.Printf("ERROR: failed to upload bundle to SDK: %v", err)
		http.Error(w, "Bundle generated but upload failed", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 300 {
		log.Printf("ERROR: SDK upload returned %d: %s", resp.StatusCode, string(body))
		http.Error(w, fmt.Sprintf("Upload failed: %d", resp.StatusCode), http.StatusBadGateway)
		return
	}

	log.Println("Support bundle generated and uploaded successfully")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"message": "Support bundle generated and uploaded to Vendor Portal",
	})
}
