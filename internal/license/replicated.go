package license

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Client struct {
	sdkURL string
	http   *http.Client
}

type LicenseField struct {
	Name      string      `json:"name"`
	Title     string      `json:"title"`
	Value     interface{} `json:"value"`
	ValueType string      `json:"valueType"`
}

type LicenseInfo struct {
	LicenseID    string `json:"licenseID"`
	CustomerName string `json:"customerName"`
	LicenseType  string `json:"licenseType"`
	ExpiresAt    string `json:"expirationPolicy"`
}

func NewClient(sdkURL string) *Client {
	return &Client{
		sdkURL: sdkURL,
		http:   &http.Client{Timeout: 5 * time.Second},
	}
}

func (c *Client) GetLicenseInfo() (*LicenseInfo, error) {
	resp, err := c.http.Get(c.sdkURL + "/api/v1/license/info")
	if err != nil {
		return nil, fmt.Errorf("fetch license: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("SDK returned status %d", resp.StatusCode)
	}
	var li LicenseInfo
	if err := json.NewDecoder(resp.Body).Decode(&li); err != nil {
		return nil, fmt.Errorf("decode license: %w", err)
	}
	return &li, nil
}

func (c *Client) GetFieldValue(fieldName string) (interface{}, error) {
	resp, err := c.http.Get(c.sdkURL + "/api/v1/license/fields/" + fieldName)
	if err != nil {
		return nil, fmt.Errorf("fetch field %s: %w", fieldName, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		return nil, nil
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("SDK returned status %d for field %s", resp.StatusCode, fieldName)
	}
	var field LicenseField
	if err := json.NewDecoder(resp.Body).Decode(&field); err != nil {
		return nil, fmt.Errorf("decode field: %w", err)
	}
	return field.Value, nil
}

func (c *Client) IsFeatureEnabled(fieldName string) bool {
	val, err := c.GetFieldValue(fieldName)
	if err != nil || val == nil {
		return false
	}
	b, ok := val.(bool)
	return ok && b
}

func (c *Client) IsExpired() bool {
	val, err := c.GetFieldValue("expires_at")
	if err != nil || val == nil {
		return false
	}
	s, ok := val.(string)
	if !ok || s == "" {
		return false
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return false
	}
	return time.Now().After(t)
}
