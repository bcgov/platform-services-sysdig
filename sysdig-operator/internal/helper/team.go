package helpers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// SysdigTeam represents a team object in Sysdig
type SysdigTeam struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// Scope defines a resource filtering scope for a team.
type Scope struct {
	Type       string `json:"type"`
	Expression string `json:"expression"`
}

// UISettings holds the team theme.
type UISettings struct {
	Theme string `json:"theme"`
}

// CreateTeamRequest is the payload for creating a team
type CreateTeamRequest struct {
	Name                      string          `json:"name"`
	Description               string          `json:"description,omitempty"`
	Product                   string          `json:"product"`
	IsDefaultTeam             bool            `json:"isDefaultTeam,omitempty"`
	CanUseAwsMetrics          bool            `json:"canUseAwsMetrics,omitempty"`
	CanUseCustomEvents        bool            `json:"canUseCustomEvents,omitempty"`
	CanUseSysdigCapture       bool            `json:"canUseSysdigCapture,omitempty"`
	Scopes                    []Scope         `json:"scopes,omitempty"`
	ZoneIDs                   []int64         `json:"zoneIds,omitempty"`
	IsAllZones                *bool           `json:"isAllZones,omitempty"`
	UISettings                UISettings      `json:"uiSettings,omitempty"`
	AdditionalTeamPermissions map[string]bool `json:"additionalTeamPermissions,omitempty"`
}

// buildFilterExpression constructs a filter expression from a list of namespaces.
func BuildFilterExpression(namespaces []string) string {
	quoted := make([]string, len(namespaces))
	for i, ns := range namespaces {
		quoted[i] = fmt.Sprintf("\"%s\"", ns)
	}
	return fmt.Sprintf("kubernetes.namespace.name in (%s)", strings.Join(quoted, ","))
}

// FetchTeams fetches teams from Sysdig API, optionally filtered by name
func FetchTeams(apiEndpoint, token, filterName string) ([]SysdigTeam, error) {
	endpoint := fmt.Sprintf("%s/platform/v1/teams", apiEndpoint)
	if filterName != "" {
		endpoint = fmt.Sprintf("%s?filter=name:%s", endpoint, filterName)
	}

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("building FetchTeams request: %w", err)
	}
	// Set your Sysdig token and content type
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	// Execute
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling FetchTeams: %w", err)
	}
	defer resp.Body.Close()

	// Handle non-200
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("FetchTeams: status %d, body %s",
			resp.StatusCode, string(body))
	}

	// Read the whole body once
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading FetchTeams body: %w", err)
	}

	// Unmarshal under `data`
	var wrapper struct {
		Data []SysdigTeam `json:"data"`
	}
	if len(bodyBytes) == 0 {
		return []SysdigTeam{}, nil
	}
	if err := json.Unmarshal(bodyBytes, &wrapper); err != nil {
		return nil, fmt.Errorf("decoding FetchTeams JSON: %w", err)
	}

	return wrapper.Data, nil
}

// CreateTeam creates a new team in Sysdig without user assignments.
// Secure selects the project zone; Sysdig still requires a HOST_CONTAINER scope.
func CreateTeam(apiEndpoint, token, name, description, product string, namespaces []string, zoneID int64) (int64, error) {
	url := fmt.Sprintf("%s/platform/v1/teams", apiEndpoint)
	scopes := []Scope{{Type: "HOST_CONTAINER", Expression: "container"}}
	var zoneIDs []int64
	var isAllZones *bool
	if product == "monitor" {
		scopes = append(scopes, Scope{Type: "AGENT", Expression: BuildFilterExpression(namespaces)})
	} else if product == "secure" {
		if zoneID <= 0 {
			return 0, fmt.Errorf("secure team %q requires a zone ID", name)
		}
		zoneIDs = []int64{zoneID}
		allZones := false
		isAllZones = &allZones
	}

	// TODO: {"type":"unprocessable_entity","message":"Teamless custom events not available in Secure","details":[]}
	perms := map[string]bool{}
	if product == "monitor" {
		perms = map[string]bool{
			"hasSysdigCaptures":       false,
			"hasInfrastructureEvents": true,
			"hasAwsData":              false,
			"hasRapidResponse":        false,
			"hasAgentCli":             true,
			"hasBeaconMetrics":        true,
		}
	} else {
		perms = map[string]bool{
			"hasSysdigCaptures":       true,
			"hasInfrastructureEvents": false,
			"hasAwsData":              false,
			"hasRapidResponse":        false,
			"hasAgentCli":             false,
			"hasBeaconMetrics":        false,
		}
	}
	reqBody := CreateTeamRequest{
		Name:                      name,
		Description:               description,
		Product:                   product,
		IsDefaultTeam:             false,
		CanUseAwsMetrics:          false,
		CanUseCustomEvents:        true,
		CanUseSysdigCapture:       false,
		Scopes:                    scopes,
		ZoneIDs:                   zoneIDs,
		IsAllZones:                isAllZones,
		UISettings:                UISettings{Theme: "#EE635E"},
		AdditionalTeamPermissions: perms,
	}
	return postTeam(url, token, reqBody)
}

// shared function to POST a team
func postTeam(url, token string, body interface{}) (int64, error) {
	payload, _ := json.Marshal(body)
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	bodyInfo, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return 0, fmt.Errorf("CreateTeam: status %d, body %s", resp.StatusCode, string(bodyInfo))
	}

	var created SysdigTeam
	if err := json.Unmarshal(bodyInfo, &created); err != nil {
		return 0, fmt.Errorf("parsing CreateTeam response JSON: %w", err)
	}
	if created.ID == 0 {
		return 0, fmt.Errorf("CreateTeam response has no team ID")
	}

	fmt.Printf("Successfully created a team with ID %d and name %s", created.ID, created.Name)
	return created.ID, nil
}

// DeleteTeam deletes a team by its ID from Sysdig.
func DeleteTeam(apiEndpoint, token string, teamID int64) error {
	url := fmt.Sprintf("%s/platform/v1/teams/%d", apiEndpoint, teamID)

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create DeleteTeam request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute DeleteTeam request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("DeleteTeam request failed: status %d, body %s", resp.StatusCode, string(bodyBytes))
	}

	fmt.Printf("Successfully deleted team with ID %d\n", teamID)
	return nil
}
