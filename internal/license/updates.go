package license

import (
	"encoding/json"
	"fmt"
)

type UpdateInfo struct {
	VersionLabel string `json:"versionLabel"`
	CreatedAt    string `json:"createdAt"`
	ReleaseNotes string `json:"releaseNotes"`
}

func (c *Client) CheckForUpdates() (*UpdateInfo, error) {
	resp, err := c.http.Get(c.sdkURL + "/api/v1/app/updates")
	if err != nil {
		return nil, fmt.Errorf("check updates: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("SDK returned status %d", resp.StatusCode)
	}
	var updates []UpdateInfo
	if err := json.NewDecoder(resp.Body).Decode(&updates); err != nil {
		return nil, fmt.Errorf("decode updates: %w", err)
	}
	if len(updates) == 0 {
		return nil, nil
	}
	return &updates[0], nil
}
