package helpers

import (
	"bytes"
	_ "embed" // This import is required for //go:embed to work. The blank identifier is used to avoid an "unused import" error.
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

//go:embed template/dashboard-resources-approve.json.j2
var dashboardTemplate []byte

// CreateDashboard creates a new dashboard in Sysdig for a given team, based on an embedded template.
func CreateDashboard(dashboardApiEndpoint, token string, teamID int64, targetNamespace string) error {
	// Prepare the payload by replacing placeholders in the template.
	replacer := strings.NewReplacer(
		"__TEAM_ID__", strconv.FormatInt(teamID, 10),
		"__TARGET_NAMESPACE__", targetNamespace,
	)
	payload := replacer.Replace(string(dashboardTemplate))

	// Prepare the API request.
	url := fmt.Sprintf("%s/api/v3/dashboards", dashboardApiEndpoint)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte(payload)))
	if err != nil {
		return fmt.Errorf("failed to create dashboard request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	// Execute the request.
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute dashboard request: %w", err)
	}
	defer resp.Body.Close()

	// Check the response status code.
	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("dashboard creation failed: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	fmt.Printf("Successfully created dashboard for team ID %d\n", teamID)
	return nil
}
