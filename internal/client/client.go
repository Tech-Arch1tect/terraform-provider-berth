package client

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

type Role struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	IsAdmin     bool   `json:"is_admin"`
}

type Permission struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Resource    string `json:"resource"`
	Action      string `json:"action"`
	Description string `json:"description"`
}

type RolePermission struct {
	ID           uint   `json:"id"`
	ServerID     uint   `json:"server_id"`
	PermissionID uint   `json:"permission_id"`
	StackPattern string `json:"stack_pattern"`
}

func NewClient(baseURL, apiKey string, insecureSkipVerify bool) *Client {
	return &Client{
		BaseURL: baseURL,
		APIKey:  apiKey,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: insecureSkipVerify,
				},
			},
		},
	}
}

func (c *Client) doRequest(method, path string, body interface{}) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(jsonBody)
	}

	url := fmt.Sprintf("%s%s", c.BaseURL, path)
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

func (c *Client) ListRoles() ([]Role, error) {
	data, err := c.doRequest("GET", "/api/v1/admin/roles", nil)
	if err != nil {
		return nil, err
	}

	var response struct {
		Roles []Role `json:"roles"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return response.Roles, nil
}

func (c *Client) GetRole(id uint) (*Role, error) {
	roles, err := c.ListRoles()
	if err != nil {
		return nil, err
	}

	for _, role := range roles {
		if role.ID == id {
			return &role, nil
		}
	}

	return nil, fmt.Errorf("role not found")
}

func (c *Client) CreateRole(name, description string) (*Role, error) {
	body := map[string]string{
		"name":        name,
		"description": description,
	}

	data, err := c.doRequest("POST", "/api/v1/admin/roles", body)
	if err != nil {
		return nil, err
	}

	var role Role
	if err := json.Unmarshal(data, &role); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &role, nil
}

func (c *Client) UpdateRole(id uint, name, description string) (*Role, error) {
	body := map[string]string{
		"name":        name,
		"description": description,
	}

	path := fmt.Sprintf("/api/v1/admin/roles/%d", id)
	data, err := c.doRequest("PUT", path, body)
	if err != nil {
		return nil, err
	}

	var role Role
	if err := json.Unmarshal(data, &role); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &role, nil
}

func (c *Client) DeleteRole(id uint) error {
	path := fmt.Sprintf("/api/v1/admin/roles/%d", id)
	_, err := c.doRequest("DELETE", path, nil)
	return err
}

func (c *Client) ListRolePermissions(roleID uint) ([]RolePermission, []Permission, error) {
	path := fmt.Sprintf("/api/v1/admin/roles/%d/stack-permissions", roleID)
	data, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, nil, err
	}

	var response struct {
		PermissionRules []RolePermission `json:"permissionRules"`
		Permissions     []Permission     `json:"permissions"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return response.PermissionRules, response.Permissions, nil
}

func (c *Client) GetRolePermission(roleID, permissionID uint) (*RolePermission, error) {
	perms, _, err := c.ListRolePermissions(roleID)
	if err != nil {
		return nil, err
	}

	for _, perm := range perms {
		if perm.ID == permissionID {
			return &perm, nil
		}
	}

	return nil, fmt.Errorf("permission not found")
}

func (c *Client) CreateRolePermission(roleID, serverID, permissionID uint, stackPattern string) (*RolePermission, error) {
	body := map[string]interface{}{
		"server_id":     serverID,
		"permission_id": permissionID,
		"stack_pattern": stackPattern,
	}

	path := fmt.Sprintf("/api/v1/admin/roles/%d/stack-permissions", roleID)
	_, err := c.doRequest("POST", path, body)
	if err != nil {
		return nil, err
	}

	return &RolePermission{
		ServerID:     serverID,
		PermissionID: permissionID,
		StackPattern: stackPattern,
	}, nil
}

func (c *Client) DeleteRolePermission(roleID, permissionID uint) error {
	path := fmt.Sprintf("/api/v1/admin/roles/%d/stack-permissions/%d", roleID, permissionID)
	_, err := c.doRequest("DELETE", path, nil)
	return err
}

func (c *Client) ListPermissions() ([]Permission, error) {
	data, err := c.doRequest("GET", "/api/v1/admin/permissions", nil)
	if err != nil {
		return nil, err
	}

	var response struct {
		Permissions []Permission `json:"permissions"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return response.Permissions, nil
}

func (c *Client) GetPermissionByName(name string) (*Permission, error) {
	permissions, err := c.ListPermissions()
	if err != nil {
		return nil, err
	}

	for _, perm := range permissions {
		if perm.Name == name {
			return &perm, nil
		}
	}

	return nil, fmt.Errorf("permission '%s' not found", name)
}
