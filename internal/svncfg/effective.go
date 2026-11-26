package svncfg

import (
	"fmt"
	"strings"
)

type EffectiveAccess struct {
	Permission Permission `json:"permission"`
	SourcePath string     `json:"sourcePath,omitempty"`
	Sources    []string   `json:"sources"`
	Supported  bool       `json:"supported"`
	Warnings   []string   `json:"warnings,omitempty"`
	Repo       string     `json:"repo"`
	Path       string     `json:"path"`
	User       string     `json:"user"`
}

func (d *AuthzDoc) Effective(repo, repoPath, user string) EffectiveAccess {
	repoPath = NormalizePath(repoPath)
	result := EffectiveAccess{
		Permission: PermissionNone,
		Sources:    []string{},
		Supported:  true,
		Repo:       repo,
		Path:       repoPath,
		User:       user,
	}
	if d.HasAliases {
		result.Supported = false
		result.Warnings = append(result.Warnings, "文件包含 aliases，无法完全计算有效权限")
	}
	if d.HasAmbiguous {
		result.Supported = false
		result.Warnings = append(result.Warnings, "authz 存在重复 section 或分组，无法完全计算有效权限")
	}
	groups, err := d.Groups()
	if err != nil {
		result.Supported = false
		result.Warnings = append(result.Warnings, "分组定义存在歧义，无法完全计算有效权限")
		groups = nil
	}
	userGroups := make(map[string]bool)
	for _, group := range groups {
		if group.Unsupported {
			result.Supported = false
			result.Warnings = append(result.Warnings, fmt.Sprintf("分组 %s 含嵌套或未知成员，未参与计算", group.Name))
			continue
		}
		for _, member := range group.Members {
			if member == user {
				userGroups[group.Name] = true
			}
		}
	}

	ancestors := pathAncestors(repoPath)
	for _, candidatePath := range ancestors {
		section := d.preferredSection(repo, candidatePath)
		if section == nil {
			continue
		}
		entries := d.parseACLEntries(*section)
		permission := PermissionNone
		matched := false
		for _, entry := range entries {
			if entry.Unsupported {
				result.Supported = false
				result.Warnings = append(result.Warnings, "目标路径 section 含不支持的反转、别名或权限表达式")
			}
			if !entryMatches(entry.Identity, user, userGroups) {
				continue
			}
			matched = true
			result.Sources = append(result.Sources, entry.RawIdentity+"="+entry.RawValue)
			if entry.Unsupported {
				continue
			}
			permission = combinePermission(permission, entry.Permission)
		}
		if matched {
			result.Permission = permission
			result.SourcePath = candidatePath
			return result
		}
	}
	return result
}

func (d *AuthzDoc) preferredSection(repo, repoPath string) *authzSection {
	var generic *authzSection
	for i := range d.Sections {
		section := &d.Sections[i]
		if section.Kind != "acl" || section.Path != repoPath || section.Duplicate {
			continue
		}
		if !section.Generic && section.Repo == repo {
			return section
		}
		if section.Generic {
			generic = section
		}
	}
	return generic
}

func entryMatches(identity ACLIdentity, user string, groups map[string]bool) bool {
	switch identity.Kind {
	case IdentityAll:
		return true
	case IdentityAuthenticated:
		return strings.TrimSpace(user) != ""
	case IdentityAnonymous:
		return strings.TrimSpace(user) == ""
	case IdentityUser:
		return strings.TrimSpace(user) != "" && identity.Name == user
	case IdentityGroup:
		return groups[identity.Name]
	default:
		return false
	}
}

func combinePermission(current, next Permission) Permission {
	if current == PermissionWrite || next == PermissionWrite {
		return PermissionWrite
	}
	if current == PermissionRead || next == PermissionRead {
		return PermissionRead
	}
	return PermissionNone
}

func pathAncestors(value string) []string {
	value = NormalizePath(value)
	var result []string
	for {
		result = append(result, value)
		if value == "/" {
			break
		}
		value = NormalizePath(strings.TrimSuffix(value, "/"))
		idx := strings.LastIndex(value, "/")
		if idx <= 0 {
			value = "/"
		} else {
			value = value[:idx]
		}
	}
	return result
}
