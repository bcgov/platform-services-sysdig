// internal/helper/fetch.go
package helpers

import (
	"encoding/json"
	"fmt"
	"io"

	// "json"
	"net/http"
	"time"
)

// // FetchTeams makes a GET request to the Sysdig API to retrieve current teams.
// func FetchTeams(apiEndpoint, token string) ([]byte, error) {

// 	// https://api.us1.sysdig.com/platform/v1/users
// 	url := fmt.Sprintf("%s/platform/v1/teams", apiEndpoint)

// 	// Create an HTTP client with a timeout.
// 	client := &http.Client{Timeout: 10 * time.Second}

// 	// Create the GET request.
// 	req, err := http.NewRequest("GET", url, nil)
// 	if err != nil {
// 		return nil, err
// 	}

// 	// Set the headers.
// 	req.Header.Set("Authorization", "Bearer "+token)
// 	req.Header.Set("Content-Type", "application/json")

// 	// Send the request.
// 	resp, err := client.Do(req)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer resp.Body.Close()

// 	// Check for a successful response.
// 	if resp.StatusCode != http.StatusOK {
// 		return nil, fmt.Errorf("FetchTeams: unexpected status code %d", resp.StatusCode)
// 	}

// 	// Read and return the response body.
// 	return ioutil.ReadAll(resp.Body)
// }

// FetchUsers calls GET /platform/v1/users and applies an optional email filter.
// If filterEmail is non-empty, the request uses the 'filter=email:<value>' query parameter.
func FetchUsers(apiEndpoint, token, filterEmail string) ([]SysdigUser, error) {
	// Build base URL
	endpoint := fmt.Sprintf("%s/platform/v1/users", apiEndpoint)
	// Append filter parameter if provided
	if filterEmail != "" {
		endpoint = fmt.Sprintf("%s?filter=email:%s", endpoint, filterEmail)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("FetchUsers: status %d, body %s", resp.StatusCode, string(body))
	}

	// Decode the wrapper and return the inner slice
	var wrapper UsersResponse
	if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return wrapper.Data, nil
}
