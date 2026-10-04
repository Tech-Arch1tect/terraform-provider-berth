package client

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	berth "github.com/tech-arch1tect/berth-go-api-client"
)

var ErrNotFound = errors.New("not found")

type NotFoundError struct {
	Detail string
}

func (e *NotFoundError) Error() string { return e.Detail }

func (e *NotFoundError) Is(target error) bool { return target == ErrNotFound }

func notFound(detail string) error {
	return &NotFoundError{Detail: detail}
}

type Client struct {
	api    *berth.APIClient
	ctx    context.Context
	apiKey string
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

func (c *Client) ListRoles() ([]berth.RoleWithPermissions, error) {
	resp, httpResp, err := c.api.AdminAPI.ApiV1AdminRolesGet(c.ctx).Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
			return nil, notFound(fmt.Sprintf("failed to list roles: %s", httpResp.Status))
		}
		return nil, fmt.Errorf("failed to list roles: %w", err)
	}

	return resp.Data.Roles, nil
}

func (c *Client) GetRole(id uint) (*berth.RoleWithPermissions, error) {
	roles, err := c.ListRoles()
	if err != nil {
		return nil, err
	}

	for i, role := range roles {
		if uint(role.Id) == id {
			return &roles[i], nil
		}
	}

	return nil, notFound("role not found")
}

func (c *Client) CreateRole(name, description string) (*berth.RoleWithPermissions, error) {
	req := berth.NewCreateRoleRequest(description, name)

	resp, _, err := c.api.AdminAPI.ApiV1AdminRolesPost(c.ctx).CreateRoleRequest(*req).Execute()
	if err != nil {
		return nil, fmt.Errorf("failed to create role: %w", err)
	}

	return &resp.Data, nil
}

func (c *Client) UpdateRole(id uint, name, description string) (*berth.RoleWithPermissions, error) {
	req := berth.NewUpdateRoleRequest(description, name)

	resp, _, err := c.api.AdminAPI.ApiV1AdminRolesIdPut(c.ctx, int32(id)).UpdateRoleRequest(*req).Execute()
	if err != nil {
		return nil, fmt.Errorf("failed to update role: %w", err)
	}

	return &resp.Data, nil
}

func (c *Client) DeleteRole(id uint) error {
	_, _, err := c.api.AdminAPI.ApiV1AdminRolesIdDelete(c.ctx, int32(id)).Execute()
	if err != nil {
		return fmt.Errorf("failed to delete role: %w", err)
	}
	return nil
}

func (c *Client) ListRolePermissions(roleID uint) ([]berth.StackPermissionRule, []berth.PermissionInfo, error) {
	resp, httpResp, err := c.api.AdminAPI.ApiV1AdminRolesRoleIdStackPermissionsGet(c.ctx, int32(roleID)).Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
			return nil, nil, notFound(fmt.Sprintf("failed to list role permissions: %s", httpResp.Status))
		}
		return nil, nil, fmt.Errorf("failed to list role permissions: %w", err)
	}

	return resp.Data.PermissionRules, resp.Data.Permissions, nil
}

func (c *Client) GetRolePermission(roleID, permissionID uint) (*berth.StackPermissionRule, error) {
	perms, _, err := c.ListRolePermissions(roleID)
	if err != nil {
		return nil, err
	}

	for i, perm := range perms {
		if uint(perm.Id) == permissionID {
			return &perms[i], nil
		}
	}

	return nil, notFound("permission not found")
}

func (c *Client) CreateRolePermission(roleID, serverID, permissionID uint, stackPattern string) (*berth.StackPermissionRule, error) {
	req := berth.NewCreateStackPermissionRequest(int32(permissionID), int32(serverID), stackPattern)

	_, _, err := c.api.AdminAPI.ApiV1AdminRolesRoleIdStackPermissionsPost(c.ctx, int32(roleID)).CreateStackPermissionRequest(*req).Execute()
	if err != nil {
		return nil, fmt.Errorf("failed to create role permission: %w", err)
	}

	return &berth.StackPermissionRule{
		ServerId:     int32(serverID),
		PermissionId: int32(permissionID),
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

func (c *Client) ListPermissions() ([]berth.PermissionInfo, error) {
	resp, httpResp, err := c.api.AdminAPI.ApiV1AdminPermissionsGet(c.ctx).Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
			return nil, notFound(fmt.Sprintf("failed to list permissions: %s", httpResp.Status))
		}
		return nil, fmt.Errorf("failed to list permissions: %w", err)
	}

	return resp.Data.Permissions, nil
}

func (c *Client) ListRoleAssignablePermissions() ([]berth.PermissionInfo, error) {
	resp, httpResp, err := c.api.AdminAPI.ApiV1AdminPermissionsGet(c.ctx).Type_("role").Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
			return nil, notFound(fmt.Sprintf("failed to list role-assignable permissions: %s", httpResp.Status))
		}
		return nil, fmt.Errorf("failed to list role-assignable permissions: %w", err)
	}

	return resp.Data.Permissions, nil
}

func (c *Client) GetRoleAssignablePermissionByName(name string) (*berth.PermissionInfo, error) {
	permissions, err := c.ListRoleAssignablePermissions()
	if err != nil {
		return nil, err
	}

	for i, permission := range permissions {
		if permission.Name == name {
			return &permissions[i], nil
		}
	}

	return nil, fmt.Errorf("role-assignable permission '%s' not found", name)
}

func (c *Client) GetPermissionByName(name string) (*berth.PermissionInfo, error) {
	permissions, err := c.ListPermissions()
	if err != nil {
		return nil, err
	}

	for i, permission := range permissions {
		if permission.Name == name {
			return &permissions[i], nil
		}
	}

	return nil, fmt.Errorf("permission '%s' not found", name)
}

func (c *Client) ListServers() ([]berth.ServerInfo, error) {
	resp, httpResp, err := c.api.AdminAPI.ApiV1AdminServersGet(c.ctx).Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
			return nil, notFound(fmt.Sprintf("failed to list servers: %s", httpResp.Status))
		}
		return nil, fmt.Errorf("failed to list servers: %w", err)
	}

	return resp.Data.Servers, nil
}

func (c *Client) ListS3Buckets() ([]berth.BucketResponse, error) {
	resp, httpResp, err := c.api.S3BucketsAPI.ApiV1AdminS3BucketsGet(c.ctx).Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
			return nil, notFound(fmt.Sprintf("failed to list s3 buckets: %s", httpResp.Status))
		}
		return nil, fmt.Errorf("failed to list s3 buckets: %w", err)
	}

	return resp.Data, nil
}

func (c *Client) GetS3Bucket(id uint) (*berth.BucketResponse, error) {
	resp, httpResp, err := c.api.S3BucketsAPI.ApiV1AdminS3BucketsIdGet(c.ctx, int32(id)).Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
			return nil, notFound(fmt.Sprintf("failed to get s3 bucket: %s", httpResp.Status))
		}
		return nil, fmt.Errorf("failed to get s3 bucket: %w", err)
	}

	return &resp.Data, nil
}

func (c *Client) CreateS3Bucket(label, endpoint, region, bucketName, accessKeyID, secretAccessKey string) (*berth.BucketResponse, error) {
	req := berth.CreateRequest{
		AccessKeyId:     accessKeyID,
		BucketName:      bucketName,
		Endpoint:        endpoint,
		Label:           label,
		Region:          region,
		SecretAccessKey: secretAccessKey,
	}

	resp, httpResp, err := c.api.S3BucketsAPI.ApiV1AdminS3BucketsPost(c.ctx).CreateRequest(req).Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
			return nil, notFound(fmt.Sprintf("failed to create s3 bucket: %s", httpResp.Status))
		}
		return nil, fmt.Errorf("failed to create s3 bucket: %w", err)
	}

	return &resp.Data, nil
}

func (c *Client) UpdateS3Bucket(id uint, label, endpoint, region, bucketName, accessKeyID, secretAccessKey string) (*berth.BucketResponse, error) {
	req := berth.UpdateRequest{
		AccessKeyId:     accessKeyID,
		BucketName:      bucketName,
		Endpoint:        endpoint,
		Label:           label,
		Region:          region,
		SecretAccessKey: &secretAccessKey,
	}

	resp, httpResp, err := c.api.S3BucketsAPI.ApiV1AdminS3BucketsIdPut(c.ctx, int32(id)).UpdateRequest(req).Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
			return nil, notFound(fmt.Sprintf("failed to update s3 bucket: %s", httpResp.Status))
		}
		return nil, fmt.Errorf("failed to update s3 bucket: %w", err)
	}

	return &resp.Data, nil
}

func (c *Client) DeleteS3Bucket(id uint) error {
	_, httpResp, err := c.api.S3BucketsAPI.ApiV1AdminS3BucketsIdDelete(c.ctx, int32(id)).Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
			return notFound(fmt.Sprintf("failed to delete s3 bucket: %s", httpResp.Status))
		}
		return fmt.Errorf("failed to delete s3 bucket: %w", err)
	}
	return nil
}

func (c *Client) ListRegistryCredentials(serverID uint) ([]berth.RegistryCredentialInfo, error) {
	resp, httpResp, err := c.api.RegistriesAPI.ApiV1ServersServeridRegistriesGet(c.ctx, int32(serverID)).Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
			return nil, notFound(fmt.Sprintf("failed to list registry credentials: %s", httpResp.Status))
		}
		return nil, fmt.Errorf("failed to list registry credentials: %w", err)
	}

	return resp.Data.Credentials, nil
}

func (c *Client) GetRegistryCredential(serverID, id uint) (*berth.RegistryCredentialInfo, error) {
	resp, httpResp, err := c.api.RegistriesAPI.ApiV1ServersServeridRegistriesIdGet(c.ctx, int32(serverID), int32(id)).Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
			return nil, notFound(fmt.Sprintf("failed to get registry credential: %s", httpResp.Status))
		}
		return nil, fmt.Errorf("failed to get registry credential: %w", err)
	}

	return &resp.Data.Credential, nil
}

func (c *Client) CreateRegistryCredential(serverID uint, stackPattern, registryURL, imagePattern, username, password string) (*berth.RegistryCredentialInfo, error) {
	req := berth.CreateCredentialRequest{
		ImagePattern: credentialOptionalPointer(imagePattern),
		Password:     password,
		RegistryUrl:  registryURL,
		StackPattern: credentialOptionalPointer(stackPattern),
		Username:     username,
	}

	resp, httpResp, err := c.api.RegistriesAPI.ApiV1ServersServeridRegistriesPost(c.ctx, int32(serverID)).CreateCredentialRequest(req).Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
			return nil, notFound(fmt.Sprintf("failed to create registry credential: %s", httpResp.Status))
		}
		return nil, fmt.Errorf("failed to create registry credential: %w", err)
	}

	return &resp.Data.Credential, nil
}

func (c *Client) UpdateRegistryCredential(serverID, id uint, stackPattern, registryURL, imagePattern, username, password string) (*berth.RegistryCredentialInfo, error) {
	req := berth.UpdateCredentialRequest{
		ImagePattern: credentialOptionalPointer(imagePattern),
		Password:     credentialOptionalPointer(password),
		RegistryUrl:  registryURL,
		StackPattern: credentialOptionalPointer(stackPattern),
		Username:     username,
	}

	resp, httpResp, err := c.api.RegistriesAPI.ApiV1ServersServeridRegistriesIdPut(c.ctx, int32(serverID), int32(id)).UpdateCredentialRequest(req).Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
			return nil, notFound(fmt.Sprintf("failed to update registry credential: %s", httpResp.Status))
		}
		return nil, fmt.Errorf("failed to update registry credential: %w", err)
	}

	return &resp.Data.Credential, nil
}

func (c *Client) DeleteRegistryCredential(serverID, id uint) error {
	_, httpResp, err := c.api.RegistriesAPI.ApiV1ServersServeridRegistriesIdDelete(c.ctx, int32(serverID), int32(id)).Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
			return notFound(fmt.Sprintf("failed to delete registry credential: %s", httpResp.Status))
		}
		return fmt.Errorf("failed to delete registry credential: %w", err)
	}
	return nil
}

func credentialOptionalPointer(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func (c *Client) GetServer(id uint) (*berth.ServerInfo, error) {
	resp, httpResp, err := c.api.AdminAPI.ApiV1AdminServersIdGet(c.ctx, int32(id)).Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
			return nil, notFound(fmt.Sprintf("failed to get server: %s", httpResp.Status))
		}
		return nil, fmt.Errorf("failed to get server: %w", err)
	}

	return &resp.Data.Server, nil
}

func (c *Client) CreateServer(name, host string, port int64, accessToken, backupPassword, description string, backupsEnabled, isActive, skipSSLVerification *bool) (*berth.ServerInfo, error) {
	req := berth.NewServerCreateRequest(accessToken, host, name, int32(port))
	req.BackupPassword = serverOptionalString(backupPassword)
	req.Description = serverOptionalString(description)
	req.BackupsEnabled = backupsEnabled
	req.IsActive = isActive
	if skipSSLVerification != nil {
		req.SkipSslVerification.Set(skipSSLVerification)
	}

	resp, httpResp, err := c.api.AdminAPI.ApiV1AdminServersPost(c.ctx).ServerCreateRequest(*req).Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
			return nil, notFound(fmt.Sprintf("failed to create server: %s", httpResp.Status))
		}
		return nil, fmt.Errorf("failed to create server: %w", err)
	}

	return &resp.Data.Server, nil
}

func (c *Client) UpdateServer(id uint, name, host string, port int64, accessToken, backupPassword, description string, backupsEnabled, isActive, skipSSLVerification *bool, s3BucketID *int64) (*berth.ServerInfo, error) {
	req := berth.NewServerUpdateRequest(host, name, int32(port))
	req.AccessToken = serverOptionalString(accessToken)
	req.BackupPassword = serverOptionalString(backupPassword)
	req.Description = &description
	req.BackupsEnabled = backupsEnabled
	req.IsActive = isActive
	if skipSSLVerification != nil {
		req.SkipSslVerification.Set(skipSSLVerification)
	}
	if s3BucketID != nil {
		bucketID := int32(*s3BucketID)
		req.S3BucketId.Set(&bucketID)
	} else {
		req.S3BucketId.Set(nil)
	}

	resp, httpResp, err := c.api.AdminAPI.ApiV1AdminServersIdPut(c.ctx, int32(id)).ServerUpdateRequest(*req).Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
			return nil, notFound(fmt.Sprintf("failed to update server: %s", httpResp.Status))
		}
		return nil, fmt.Errorf("failed to update server: %w", err)
	}

	return &resp.Data.Server, nil
}

func (c *Client) DeleteServer(id uint) error {
	_, httpResp, err := c.api.AdminAPI.ApiV1AdminServersIdDelete(c.ctx, int32(id)).Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
			return notFound(fmt.Sprintf("failed to delete server: %s", httpResp.Status))
		}
		return fmt.Errorf("failed to delete server: %w", err)
	}
	return nil
}

func serverOptionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func (c *Client) GetUserRoles(userID uint) (*berth.UserInfo, error) {
	resp, httpResp, err := c.api.AdminAPI.ApiV1AdminUsersIdRolesGet(c.ctx, int32(userID)).Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
			return nil, notFound(fmt.Sprintf("failed to get user roles: %s", httpResp.Status))
		}
		return nil, fmt.Errorf("failed to get user roles: %w", err)
	}

	return &resp.Data.User, nil
}

func (c *Client) AssignRole(userID, roleID uint) error {
	req := berth.NewAssignRoleRequest(int32(roleID), int32(userID))

	_, httpResp, err := c.api.AdminAPI.ApiV1AdminUsersAssignRolePost(c.ctx).AssignRoleRequest(*req).Execute()
	if err != nil {
		return roleAssignmentError("failed to assign role", err, httpResp)
	}
	return nil
}

func (c *Client) RevokeRole(userID, roleID uint) error {
	req := berth.NewRevokeRoleRequest(int32(roleID), int32(userID))

	_, httpResp, err := c.api.AdminAPI.ApiV1AdminUsersRevokeRolePost(c.ctx).RevokeRoleRequest(*req).Execute()
	if err != nil {
		return roleAssignmentError("failed to revoke role", err, httpResp)
	}
	return nil
}

func roleAssignmentError(prefix string, err error, httpResp *http.Response) error {
	if httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
		return notFound(fmt.Sprintf("%s: %s", prefix, httpResp.Status))
	}

	var apiErr *berth.GenericOpenAPIError
	if errors.As(err, &apiErr) {
		var payload struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if jsonErr := json.Unmarshal(apiErr.Body(), &payload); jsonErr == nil && payload.Error.Message != "" {
			return fmt.Errorf("%s: %s", prefix, payload.Error.Message)
		}
	}

	return fmt.Errorf("%s: %w", prefix, err)
}
