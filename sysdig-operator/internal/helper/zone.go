package helpers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var zoneClient = &http.Client{Timeout: 15 * time.Second}

// EnsureZone returns the project zone, creating it when it does not exist.
func EnsureZone(ctx context.Context, apiEndpoint, token, projectID string) (int64, error) {
	name := projectID + "-team-secure-zone"
	zonesURL := strings.TrimRight(apiEndpoint, "/") + "/platform/v2/zones"
	find := func() (int64, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, zonesURL+"?filter="+url.QueryEscape("name:"+name), nil)
		if err != nil {
			return 0, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := zoneClient.Do(req)
		if err != nil {
			return 0, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
			return 0, fmt.Errorf("list zones: status %d: %s", resp.StatusCode, body)
		}
		var result struct {
			Data []struct {
				ID   int64  `json:"id"`
				Name string `json:"name"`
			} `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return 0, fmt.Errorf("decode zones: %w", err)
		}
		for _, zone := range result.Data {
			if strings.EqualFold(zone.Name, name) {
				if zone.ID == 0 {
					return 0, fmt.Errorf("zone %q has no ID", name)
				}
				// ponytail: manually managed zones keep their existing scope.
				return zone.ID, nil
			}
		}
		return 0, nil
	}

	id, err := find()
	if id != 0 || err != nil {
		return id, err
	}

	payload, err := json.Marshal(map[string]any{
		"name": name,
		"scopes": []any{map[string]any{"filters": []any{map[string]any{
			"resourceType": "kubernetes",
			"expressions": []any{map[string]any{
				"field": "namespace", "operator": "contains", "value": []string{projectID},
			}},
		}}}},
	})
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, zonesURL, bytes.NewReader(payload))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := zoneClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusConflict {
		// Another reconciliation may have created the same zone after our lookup.
		id, err := find()
		if id != 0 || err != nil {
			return id, err
		}
		return 0, fmt.Errorf("zone %q conflicted but was not found", name)
	}
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return 0, fmt.Errorf("create zone %q: status %d: %s", name, resp.StatusCode, body)
	}
	var created struct {
		ID int64 `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		return 0, fmt.Errorf("decode created zone: %w", err)
	}
	if created.ID == 0 {
		return 0, fmt.Errorf("created zone %q has no ID", name)
	}
	return created.ID, nil
}
