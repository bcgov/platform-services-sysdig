package helpers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// TeamMembership represents one user’s membership on a team
type TeamMembership struct {
	UserID int64  `json:"userId"`           // the user’s ID
	Role   string `json:"standardTeamRole"` // the role field
}

// FetchTeamMemberships fetches current user memberships for the given team.
func FetchTeamMemberships(apiEndpoint, token string, teamID int64) ([]TeamMembership, error) {
	url := fmt.Sprintf("%s/platform/v1/teams/%d/users", apiEndpoint, teamID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("FetchTeamMemberships: status %d, body %s", resp.StatusCode, string(b))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// The API returns { "page": {...}, "data": [ {userId, standardTeamRole, ...}, ... ] }
	var wrapper struct {
		Data []TeamMembership `json:"data"`
	}
	if err := json.Unmarshal(body, &wrapper); err != nil {
		return nil, fmt.Errorf("unmarshal memberships: %w", err)
	}
	return wrapper.Data, nil
}

// DeleteMembership removes a user from a Sysdig team.
func DeleteMembership(apiEndpoint, token string, teamID, userID int64) error {
	url := fmt.Sprintf("%s/platform/v1/teams/%d/users/%d",
		apiEndpoint, teamID, userID)
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("DeleteMembership: status %d, body %s",
			resp.StatusCode, string(body))
	}

	return nil
}

// SaveMembership adds or updates xxa user's role in a Sysdig team.
// PUT /platform/v1/teams/{teamId}/users/{userId}
// https://app.sysdigcloud.com/apidocs/monitor?_product=SDC#tag/Teams/operation/saveTeamUserV1
func SaveMembership(apiEndpoint, token string, teamID, userID int64, role string) (string, error) {
	url := fmt.Sprintf("%s/platform/v1/teams/%d/users/%d", apiEndpoint, teamID, userID)
	payload := map[string]string{"standardTeamRole": role}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("SaveMembership: status %d, body %s", resp.StatusCode, string(b))
	}
	// Read response body for success path
	respBody, _ := io.ReadAll(resp.Body)

	return string(respBody), nil
}
