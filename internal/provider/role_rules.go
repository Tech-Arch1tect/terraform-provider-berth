package provider

import (
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/tech-arch1tect/terraform-provider-berth/internal/client"
)

type roleRuleKey struct {
	serverID     uint
	permissionID uint
	stackPattern string
}

func normaliseStackPattern(stackPattern string) string {
	if stackPattern == "" {
		return "*"
	}
	return stackPattern
}

func stackPatternValue(stackPattern types.String) string {
	if stackPattern.IsNull() || stackPattern.IsUnknown() {
		return "*"
	}
	return normaliseStackPattern(stackPattern.ValueString())
}

func roleRuleKeyOf(rule client.RolePermission) roleRuleKey {
	return roleRuleKey{
		serverID:     rule.ServerID,
		permissionID: rule.PermissionID,
		stackPattern: normaliseStackPattern(rule.StackPattern),
	}
}

func permissionIDsByName(permissions []client.Permission) map[string]uint {
	ids := make(map[string]uint, len(permissions))
	for _, permission := range permissions {
		ids[permission.Name] = permission.ID
	}
	return ids
}

func claimedRoleRuleKeys(permissions []RolePermissionInline, permissionIDs map[string]uint) ([]roleRuleKey, error) {
	claimed := make([]roleRuleKey, 0, len(permissions))
	for _, permission := range permissions {
		name := permission.PermissionName.ValueString()
		permissionID, ok := permissionIDs[name]
		if !ok {
			return nil, fmt.Errorf("permission '%s' not found", name)
		}
		claimed = append(claimed, roleRuleKey{
			serverID:     uint(permission.ServerID.ValueInt64()),
			permissionID: permissionID,
			stackPattern: stackPatternValue(permission.StackPattern),
		})
	}
	return claimed, nil
}

func refreshedRoleRules(rules []client.RolePermission, permissions []client.Permission, current []RolePermissionInline) ([]RolePermissionInline, error) {
	claimed, err := claimedRoleRuleKeys(current, permissionIDsByName(permissions))
	if err != nil {
		return nil, err
	}

	names := make(map[uint]string, len(permissions))
	for _, permission := range permissions {
		names[permission.ID] = permission.Name
	}

	refreshed := make([]RolePermissionInline, 0, len(current))
	for _, key := range claimed {
		for _, rule := range rules {
			if roleRuleKeyOf(rule) != key {
				continue
			}
			refreshed = append(refreshed, RolePermissionInline{
				ID:             types.StringValue(strconv.FormatUint(uint64(rule.ID), 10)),
				ServerID:       types.Int64Value(int64(rule.ServerID)),
				PermissionName: types.StringValue(names[rule.PermissionID]),
				StackPattern:   types.StringValue(normaliseStackPattern(rule.StackPattern)),
			})
			break
		}
	}
	return refreshed, nil
}

func rolePermissionsTouched(plan, state *RoleResourceModel) bool {
	return len(plan.Permissions) > 0 || len(state.Permissions) > 0 ||
		len(plan.PermissionSets) > 0 || len(state.PermissionSets) > 0
}

func plannedRoleRules(model *RoleResourceModel, permissionIDs map[string]uint) ([]client.RolePermission, error) {
	desired := make([]client.RolePermission, 0)
	seen := make(map[roleRuleKey]bool)

	add := func(serverID uint, permissionName, stackPattern string) error {
		permissionID, ok := permissionIDs[permissionName]
		if !ok {
			return fmt.Errorf("permission '%s' not found", permissionName)
		}
		key := roleRuleKey{
			serverID:     serverID,
			permissionID: permissionID,
			stackPattern: normaliseStackPattern(stackPattern),
		}
		if !seen[key] {
			seen[key] = true
			desired = append(desired, client.RolePermission{
				ServerID:     serverID,
				PermissionID: permissionID,
				StackPattern: key.stackPattern,
			})
		}
		return nil
	}

	for _, set := range model.PermissionSets {
		for _, serverID := range set.ServerIDs {
			for _, permission := range set.Permissions {
				if err := add(uint(serverID.ValueInt64()), permission.Name.ValueString(), stackPatternValue(permission.Pattern)); err != nil {
					return nil, err
				}
			}
		}
	}

	for _, permission := range model.Permissions {
		if err := add(uint(permission.ServerID.ValueInt64()), permission.PermissionName.ValueString(), stackPatternValue(permission.StackPattern)); err != nil {
			return nil, err
		}
	}

	return desired, nil
}

func roleRuleChanges(desired, previous, current []client.RolePermission) (deletions []uint, creations []client.RolePermission) {
	desiredKeys := make(map[roleRuleKey]bool, len(desired))
	for _, rule := range desired {
		desiredKeys[roleRuleKeyOf(rule)] = true
	}

	previousKeys := make(map[roleRuleKey]bool, len(previous))
	for _, rule := range previous {
		previousKeys[roleRuleKeyOf(rule)] = true
	}

	currentKeys := make(map[roleRuleKey]bool, len(current))
	for _, rule := range current {
		key := roleRuleKeyOf(rule)
		currentKeys[key] = true
		if previousKeys[key] && !desiredKeys[key] {
			deletions = append(deletions, rule.ID)
		}
	}

	for _, rule := range desired {
		if !currentKeys[roleRuleKeyOf(rule)] {
			creations = append(creations, rule)
		}
	}

	return deletions, creations
}

func (r *RoleResource) reconcileRoleRules(roleID uint, plan, state *RoleResourceModel) error {
	permissions, err := r.client.ListPermissions()
	if err != nil {
		return err
	}
	permissionIDs := permissionIDsByName(permissions)

	desired, err := plannedRoleRules(plan, permissionIDs)
	if err != nil {
		return err
	}

	previous, err := plannedRoleRules(state, permissionIDs)
	if err != nil {
		return err
	}

	current, _, err := r.client.ListRolePermissions(roleID)
	if err != nil {
		return err
	}

	deletions, creations := roleRuleChanges(desired, previous, current)

	for _, ruleID := range deletions {
		if err := r.client.DeleteRolePermission(roleID, ruleID); err != nil {
			return err
		}
	}

	for _, rule := range creations {
		if _, err := r.client.CreateRolePermission(roleID, rule.ServerID, rule.PermissionID, rule.StackPattern); err != nil {
			return err
		}
	}

	if len(plan.Permissions) > 0 {
		rules, _, err := r.client.ListRolePermissions(roleID)
		if err != nil {
			return err
		}
		for i, permission := range plan.Permissions {
			serverID := uint(permission.ServerID.ValueInt64())
			stackPattern := stackPatternValue(permission.StackPattern)
			permissionID := permissionIDs[permission.PermissionName.ValueString()]
			for _, rule := range rules {
				if rule.ServerID == serverID && rule.PermissionID == permissionID && rule.StackPattern == stackPattern {
					plan.Permissions[i].ID = types.StringValue(strconv.FormatUint(uint64(rule.ID), 10))
					plan.Permissions[i].StackPattern = types.StringValue(stackPattern)
					break
				}
			}
		}
	}

	return nil
}

func (r *RoleResource) refreshClaimedRoleRules(roleID uint, data *RoleResourceModel) error {
	if len(data.Permissions) == 0 {
		return nil
	}

	permissions, err := r.client.ListPermissions()
	if err != nil {
		return err
	}

	rules, _, err := r.client.ListRolePermissions(roleID)
	if err != nil {
		return err
	}

	data.Permissions, err = refreshedRoleRules(rules, permissions, data.Permissions)
	return err
}
