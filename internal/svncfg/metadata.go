package svncfg

import (
	"fmt"
	"strings"
)

const (
	remarkPrefix      = "# svnpanel:remark:"
	departmentPrefix  = "# svnpanel:department:"
	departmentsPrefix = "# svnpanel:departments:"
	metadataPrefix    = "# svnpanel:"
	maxRemarkLength   = 500
	maxDepartments    = 200
)

type entityMetadata struct {
	Remark        string
	Department    string
	StartLine     int
	RemarkSet     bool
	DepartmentSet bool
}

func parseMetadataBefore(lines []Line, entryLine int) (entityMetadata, error) {
	metadata := entityMetadata{StartLine: entryLine}
	for i := entryLine - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(lines[i].Text)
		if !strings.HasPrefix(trimmed, metadataPrefix) {
			break
		}
		if strings.HasPrefix(trimmed, departmentsPrefix) {
			break
		}
		metadata.StartLine = i
		switch {
		case strings.HasPrefix(trimmed, remarkPrefix):
			if metadata.RemarkSet {
				return entityMetadata{}, fmt.Errorf("%w: 第 %d 行存在重复备注标记", ErrAmbiguous, i+1)
			}
			metadata.RemarkSet = true
			metadata.Remark = strings.TrimSpace(strings.TrimPrefix(trimmed, remarkPrefix))
		case strings.HasPrefix(trimmed, departmentPrefix):
			if metadata.DepartmentSet {
				return entityMetadata{}, fmt.Errorf("%w: 第 %d 行存在重复部门标记", ErrAmbiguous, i+1)
			}
			metadata.DepartmentSet = true
			metadata.Department = strings.TrimSpace(strings.TrimPrefix(trimmed, departmentPrefix))
		default:
			return entityMetadata{}, fmt.Errorf("%w: 第 %d 行包含未知的 svnpanel 元数据", ErrAmbiguous, i+1)
		}
	}
	return metadata, nil
}

func NormalizeRemark(value string) (string, error) {
	value = strings.TrimSpace(value)
	if len([]rune(value)) > maxRemarkLength {
		return "", fmt.Errorf("%w: 备注不能超过 %d 个字符", ErrValidation, maxRemarkLength)
	}
	if strings.ContainsAny(value, "\x00\r\n") {
		return "", fmt.Errorf("%w: 备注不能包含换行或 NUL", ErrValidation)
	}
	return value, nil
}

func NormalizeDepartment(value string) (string, error) {
	value = strings.TrimSpace(value)
	if len([]rune(value)) > 64 {
		return "", fmt.Errorf("%w: 部门名称过长", ErrValidation)
	}
	if strings.ContainsAny(value, "\x00\r\n#") {
		return "", fmt.Errorf("%w: 部门名称包含非法字符", ErrValidation)
	}
	return value, nil
}

func normalizeDepartments(values []string) ([]string, error) {
	if len(values) > maxDepartments {
		return nil, fmt.Errorf("%w: 部门不能超过 %d 个", ErrValidation, maxDepartments)
	}
	seen := make(map[string]bool, len(values))
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		value, err := NormalizeDepartment(value)
		if err != nil {
			return nil, err
		}
		if value == "" {
			return nil, fmt.Errorf("%w: 部门名称不能为空", ErrValidation)
		}
		if seen[value] {
			return nil, fmt.Errorf("%w: 部门 %s 重复", ErrAmbiguous, value)
		}
		seen[value] = true
		normalized = append(normalized, value)
	}
	return normalized, nil
}

func metadataLines(remark, department string) []string {
	lines := make([]string, 0, 2)
	if remark != "" {
		lines = append(lines, remarkPrefix+" "+remark)
	}
	if department != "" {
		lines = append(lines, departmentPrefix+" "+department)
	}
	return lines
}
