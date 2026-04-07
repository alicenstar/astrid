package metrics

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

type ReplicatedReporter struct {
	sdkURL string
	client *http.Client
}

func NewReplicatedReporter(sdkURL string) *ReplicatedReporter {
	return &ReplicatedReporter{
		sdkURL: sdkURL,
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

type metricsPayload struct {
	Data map[string]interface{} `json:"data"`
}

func (r *ReplicatedReporter) SendCustomMetrics(metrics map[string]interface{}) error {
	payload := metricsPayload{Data: metrics}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal metrics: %w", err)
	}
	req, err := http.NewRequest("PATCH", r.sdkURL+"/api/v1/app/custom-metrics", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("send metrics: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("SDK returned status %d", resp.StatusCode)
	}
	return nil
}

func (r *ReplicatedReporter) ReportAppMetrics(userCount, planCount, mealCount, workoutCount int) {
	metrics := map[string]interface{}{
		"user_count":    userCount,
		"plan_count":    planCount,
		"meal_count":    mealCount,
		"workout_count": workoutCount,
	}
	if err := r.SendCustomMetrics(metrics); err != nil {
		log.Printf("WARN: failed to send custom metrics to Replicated SDK: %v", err)
	}
}
