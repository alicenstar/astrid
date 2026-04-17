package license

import (
	"encoding/json"
	"fmt"
	"strings"
)

type UpdateInfo struct {
	VersionLabel string `json:"versionLabel"`
	CreatedAt    string `json:"createdAt"`
	ReleaseNotes string `json:"releaseNotes"`
}

func (c *Client) CheckForUpdates(currentVersion string) (*UpdateInfo, error) {
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

	// The SDK returns updates newer than what it thinks is deployed,
	// sorted newest-first. If the SDK is stale, our own version may
	// appear in this list. Only return updates genuinely newer than
	// the deployed chart version.
	//
	// Strip build metadata (e.g. +sha) before comparing, since the
	// app version from Chart.yaml won't include the SHA suffix that
	// the release version label has.
	baseVersion := strings.SplitN(currentVersion, "+", 2)[0]
	for i, u := range updates {
		uBase := strings.SplitN(u.VersionLabel, "+", 2)[0]
		if uBase == baseVersion {
			if i > 0 {
				return &updates[0], nil
			}
			return nil, nil
		}
	}

	return &updates[0], nil
}
