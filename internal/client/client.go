package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	berth "github.com/tech-arch1tect/berth-go-api-client"
)

type Client struct {
	api    *berth.APIClient
	ctx    context.Context
	apiKey string
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
	cfg := berth.NewConfiguration()
	cfg.Servers = berth.ServerConfigurations{
		{URL: baseURL},
	}
	cfg.Debug = false
	cfg.HTTPClient = &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: insecureSkipVerify,
			},
		},
	}

	apiClient := berth.NewAPIClient(cfg)

	ctx := context.WithValue(context.Background(), berth.ContextAccessToken, apiKey)

	return &Client{
		api:    apiClient,
		ctx:    ctx,
		apiKey: apiKey,
	}
}

func (c *Client) ListRoles() ([]Role, error) {
	resp, _, err := c.api.AdminAPI.ApiV1AdminRolesGet(c.ctx).Execute()
	if err != nil {
		return nil, fmt.Errorf("failed to list roles: %w", err)
	}

	roles := make([]Role, 0, len(resp.Data.Roles))
	for _, r := range resp.Data.Roles {
		roles = append(roles, Role{
			ID:          uint(r.Id),
			Name:        r.Name,
			Description: r.Description,
			IsAdmin:     r.IsAdmin,
		})
	}

	return roles, nil
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
	req := berth.NewCreateRoleRequest(description, name)

	resp, _, err := c.api.AdminAPI.ApiV1AdminRolesPost(c.ctx).CreateRoleRequest(*req).Execute()
	if err != nil {
		return nil, fmt.Errorf("failed to create role: %w", err)
	}

	return &Role{
		ID:          uint(resp.Data.Id),
		Name:        resp.Data.Name,
		Description: resp.Data.Description,
		IsAdmin:     resp.Data.IsAdmin,
	}, nil
}

func (c *Client) UpdateRole(id uint, name, description string) (*Role, error) {
	req := berth.NewUpdateRoleRequest(description, name)

	resp, _, err := c.api.AdminAPI.ApiV1AdminRolesIdPut(c.ctx, int32(id)).UpdateRoleRequest(*req).Execute()
	if err != nil {
		return nil, fmt.Errorf("failed to update role: %w", err)
	}

	return &Role{
		ID:          uint(resp.Data.Id),
		Name:        resp.Data.Name,
		Description: resp.Data.Description,
		IsAdmin:     resp.Data.IsAdmin,
	}, nil
}

func (c *Client) DeleteRole(id uint) error {
	_, _, err := c.api.AdminAPI.ApiV1AdminRolesIdDelete(c.ctx, int32(id)).Execute()
	if err != nil {
		return fmt.Errorf("failed to delete role: %w", err)
	}
	return nil
}

func (c *Client) ListRolePermissions(roleID uint) ([]RolePermission, []Permission, error) {
	resp, _, err := c.api.AdminAPI.ApiV1AdminRolesRoleIdStackPermissionsGet(c.ctx, int32(roleID)).Execute()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list role permissions: %w", err)
	}

	perms := make([]RolePermission, 0, len(resp.Data.PermissionRules))
	for _, p := range resp.Data.PermissionRules {
		perms = append(perms, RolePermission{
			ID:           uint(p.Id),
			ServerID:     uint(p.ServerId),
			PermissionID: uint(p.PermissionId),
			StackPattern: p.StackPattern,
		})
	}

	permissions := make([]Permission, 0, len(resp.Data.Permissions))
	for _, p := range resp.Data.Permissions {
		permissions = append(permissions, Permission{
			ID:          uint(p.Id),
			Name:        p.Name,
			Resource:    p.Resource,
			Action:      p.Action,
			Description: p.Description,
		})
	}

	return perms, permissions, nil
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
	req := berth.NewCreateStackPermissionRequest(int32(permissionID), int32(serverID), stackPattern)

	_, _, err := c.api.AdminAPI.ApiV1AdminRolesRoleIdStackPermissionsPost(c.ctx, int32(roleID)).CreateStackPermissionRequest(*req).Execute()
	if err != nil {
		return nil, fmt.Errorf("failed to create role permission: %w", err)
	}

	return &RolePermission{
		ServerID:     serverID,
		PermissionID: permissionID,
		StackPattern: stackPattern,
	}, nil
}

func (c *Client) DeleteRolePermission(roleID, permissionID uint) error {
	_, _, err := c.api.AdminAPI.ApiV1AdminRolesRoleIdStackPermissionsPermissionIdDelete(c.ctx, int32(roleID), int32(permissionID)).Execute()
	if err != nil {
		return fmt.Errorf("failed to delete role permission: %w", err)
	}
	return nil
}

func (c *Client) ListPermissions() ([]Permission, error) {
	resp, _, err := c.api.AdminAPI.ApiV1AdminPermissionsGet(c.ctx).Execute()
	if err != nil {
		return nil, fmt.Errorf("failed to list permissions: %w", err)
	}

	permissions := make([]Permission, 0, len(resp.Data.Permissions))
	for _, p := range resp.Data.Permissions {
		permissions = append(permissions, Permission{
			ID:          uint(p.Id),
			Name:        p.Name,
			Resource:    p.Resource,
			Action:      p.Action,
			Description: p.Description,
		})
	}

	return permissions, nil
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
