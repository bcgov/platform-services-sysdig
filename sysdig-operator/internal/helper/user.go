// internal/helper/users.go
package helpers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// SysdigUser represents one user object from GET /platform/v1/users
type SysdigUser struct {
	ID    int64  `json:"id"`
	Email string `json:"email"` // matches the "email" field in the JSON
	// add FirstName, LastName, etc. if you need them
}

// TeamUserRole is the final Name/Role/UserID struct
type TeamUserRole struct {
	Name   string
	Role   string
	UserID int64
}

// UsersResponse wraps the list returned by Sysdig under "data"
type UsersResponse struct {
	Data []SysdigUser `json:"data"`
}

// CreateUserRequest is the payload you POST when creating a new user
type CreateUserRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

// CreateUserResponse wraps the Sysdig response for POST /users
type CreateUserResponse struct {
	ID          int64   `json:"id"`
	Email       string  `json:"email,omitempty"`
	FirstName   *string `json:"firstName,omitempty"`
	LastName    *string `json:"lastName,omitempty"`
	IsAdmin     bool    `json:"isAdmin,omitempty"`
	Activation  string  `json:"activationStatus,omitempty"`
	DateCreated string  `json:"dateCreated,omitempty"`
	LastUpdated *string `json:"lastUpdated,omitempty"`
	Version     int     `json:"version,omitempty"`
}

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

// CreateUser creates a new user and returns its ID
func CreateUser(apiEndpoint, token, email, role string) (int64, error) {
	url := fmt.Sprintf("%s/platform/v1/users", apiEndpoint)
	payload := CreateUserRequest{Email: email, Role: role}
	body, _ := json.Marshal(payload)

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
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
	bodyInfo, err := io.ReadAll(resp.Body)

	if err != nil {
		return 0, fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		return 0, fmt.Errorf("CreateUser: status %d, body %s",
			resp.StatusCode, string(bodyInfo))
	}

	var cr CreateUserResponse
	if err := json.Unmarshal(bodyInfo, &cr); err != nil {
		return 0, fmt.Errorf("parsing CreateUser response JSON: %w", err)
	}
	fmt.Printf("I want to check what is comeback payload: \n %+v\n", cr)

	//Return the new user ID:
	return cr.ID, nil
}
