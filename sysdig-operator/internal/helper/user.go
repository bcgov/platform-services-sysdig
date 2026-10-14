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

// SysdigUser represents one user object from GET /api/users.
type SysdigUser struct {
	ID    int64  `json:"id"`
	Email string `json:"username"`
}

// TeamUserRole is the final Name/Role/UserID struct
type TeamUserRole struct {
	Name   string
	Role   string
	UserID int64
}

// UsersResponse wraps the list returned by Sysdig under "users".
type UsersResponse struct {
	Users []SysdigUser `json:"users"`
}

// FetchUsers includes users with pending invitations so retries do not re-invite them.
func FetchUsers(apiEndpoint, token, filterEmail string) ([]SysdigUser, error) {
	endpoint := fmt.Sprintf("%s/api/users", apiEndpoint)

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

	var wrapper UsersResponse
	if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if wrapper.Users == nil {
		return nil, fmt.Errorf("FetchUsers response has no users list")
	}
	if filterEmail == "" {
		return wrapper.Users, nil
	}
	for _, user := range wrapper.Users {
		if strings.EqualFold(user.Email, filterEmail) {
			return []SysdigUser{user}, nil
		}
	}
	return nil, nil
}

// InviteUser sends a new-user invitation and returns the user ID for team membership.
func InviteUser(apiEndpoint, token, email string) (int64, error) {
	url := fmt.Sprintf("%s/api/users", apiEndpoint)
	body, _ := json.Marshal(map[string]string{"username": email})

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

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("InviteUser: status %d, body %s",
			resp.StatusCode, string(bodyInfo))
	}

	var cr struct {
		User struct {
			ID int64 `json:"id"`
		} `json:"user"`
	}
	if err := json.Unmarshal(bodyInfo, &cr); err != nil {
		return 0, fmt.Errorf("parsing InviteUser response JSON: %w", err)
	}
	if cr.User.ID == 0 {
		return 0, fmt.Errorf("InviteUser response has no user ID")
	}
	return cr.User.ID, nil
}
