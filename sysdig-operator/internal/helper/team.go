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
	// Version int    `json:"version"`
}

// TeamUserRole holds user-role mapping for update payloads.
// type TeamUserRole struct {
// 	UserID int64  `json:"userId"`
// 	Role   string `json:"role"`
// }

// Scope defines a resource filtering scope for a team.
type Scope struct {
	Type       string `json:"type"`
	Expression string `json:"expression"`
}

// UISettings holds UI-related settings including entry point and theme.
type UISettings struct {
	EntryPoint *EntryPoint `json:"entryPoint,omitempty"`
	Theme      string      `json:"theme"`
}

// EntryPoint configures the default module and selection in the UI.
type EntryPoint struct {
	Module    string  `json:"module"`
	Selection *string `json:"selection,omitempty"`
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
	UISettings                UISettings      `json:"uiSettings,omitempty"`
	AdditionalTeamPermissions map[string]bool `json:"additionalTeamPermissions,omitempty"`
}

// UpdateTeamRequest is the payload for updating a team
type UpdateTeamRequest struct {
	CanUseAwsMetrics    bool   `json:"canUseAwsMetrics"`
	CanUseCustomEvents  bool   `json:"canUseCustomEvents"`
	CanUseSysdigCapture bool   `json:"canUseSysdigCapture"`
	Description         string `json:"description"`
	Name                string `json:"name"`
	ID                  int64  `json:"id"`
	Version             int    `json:"version"`
	Show                string `json:"show"`
	Theme               string `json:"theme"`
	Filter              string `json:"filter"`
	// UserRoles           []TeamUserRole `json:"userRoles"`
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

	// Log it for debugging
	fmt.Printf("DEBUG: FETCHING team response body:\n%s\n", string(bodyBytes))

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
// It populates Scopes based on provided namespace scopes.
func CreateTeam(apiEndpoint, token, name, description, product string, namespaces []string) (int64, error) {
	url := fmt.Sprintf("%s/platform/v1/teams", apiEndpoint)
	scopes := []Scope{
		{
			Type:       "HOST_CONTAINER",
			Expression: "container", // grants container access
		},
		{
			Type:       "AGENT", //bit different from API documentation: https://app.sysdigcloud.com/apidocs/monitor?_product=SDC#tag/Teams/operation/createTeamV1
			Expression: BuildFilterExpression(namespaces),
		}}
	ui := UISettings{Theme: "#73A1F7"}
	if product == "monitor" {
		ui.EntryPoint = &EntryPoint{Module: "Dashboards"}
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
		UISettings:                ui,
		AdditionalTeamPermissions: perms,
	}
	fmt.Printf("let me see see what is Create !!!!!: %+v /n", reqBody)
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
	fmt.Printf("DEBUG: create team response body:\n%s\n", string(bodyInfo))

	var created SysdigTeam
	if err := json.Unmarshal(bodyInfo, &created); err != nil {
		return 0, fmt.Errorf("parsing CreateUser response JSON: %w", err)
	}

	// fmt.Printf("DEBUG: create team, payload is 11111111: \n %+v\n", created)

	fmt.Printf("Successfully created a team with ID %d and name %s", created.ID, created.Name)
	// return created.ID, nil
	return created.ID, nil
}

// we don't update team. as we use membership api to manage access.
// func UpdateTeam(apiEndpoint, token, name, description string, id int64, version int, namespaces []string) (int64, error) {
// 	url := fmt.Sprintf("%s/platform/v1/teams/%d", apiEndpoint, id)
// 	reqBody := UpdateTeamRequest{
// 		CanUseAwsMetrics:    false,
// 		CanUseCustomEvents:  true,
// 		CanUseSysdigCapture: false,
// 		Description:         description,
// 		Name:                name,
// 		ID:                  id,
// 		Version:             version,
// 		Show:                "container",
// 		Theme:               "#73A1F7",
// 		Filter:              BuildFilterExpression(namespaces),
// 		// UserRoles:           users,
// 	}
// 	fmt.Printf("let me see see what is Update !!!!!: %v\n", reqBody)
// 	return putTeam(url, token, reqBody)
// }

// shared function to PUT a team
// func putTeam(url, token string, body interface{}) (int64, error) {
// 	payload, _ := json.Marshal(body)
// 	client := &http.Client{Timeout: 10 * time.Second}
// 	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(payload))
// 	if err != nil {
// 		return 0, err
// 	}
// 	req.Header.Set("Authorization", "Bearer "+token)
// 	req.Header.Set("Content-Type", "application/json")

// 	resp, err := client.Do(req)
// 	if err != nil {
// 		return 0, err
// 	}
// 	defer resp.Body.Close()

// 	if resp.StatusCode != http.StatusOK {
// 		b, _ := io.ReadAll(resp.Body)
// 		return 0, fmt.Errorf("UpdateTeam: status %d, body %s", resp.StatusCode, string(b))
// 	}

// 	var updated SysdigTeam
// 	if err := json.NewDecoder(resp.Body).Decode(&updated); err != nil {
// 		return 0, err
// 	}
// 	return updated.ID, nil
// }
