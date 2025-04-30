package helpers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// SaveMembership adds or updates xxa user's role in a Sysdig team.
// PUT /platform/v1/teams/{teamId}/users/{userId}
// https://app.sysdigcloud.com/apidocs/monitor?_product=SDC#tag/Teams/operation/saveTeamUserV1
func SaveMembership(apiEndpoint, token string, teamID, userID int64, role string) error {
	url := fmt.Sprintf("%s/platform/v1/teams/%d/users/%d", apiEndpoint, teamID, userID)
	payload := map[string]string{"standardTeamRole": role}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("SaveMembership: status %d, body %s", resp.StatusCode, string(b))
	}
	// Read response body for success path
	respBody, _ := io.ReadAll(resp.Body)
	fmt.Printf("DEBUG: SaveMembership resp body:\n%s\n", string(respBody))

	return nil
}
