package service

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf8"

	"svnpanel/internal/dto"
)

type parsedSvnserveConfig struct {
	values   map[string]string
	warnings []string
}

func readSvnserveConfigInfo(configPath, passwdPath, authzPath string) dto.SvnserveConfigInfo {
	if strings.TrimSpace(configPath) == "" {
		configPath = filepath.Join(filepath.Dir(passwdPath), "svnserve.conf")
	}
	info := dto.SvnserveConfigInfo{
		Path:     configPath,
		Warnings: []string{},
	}
	stat, err := os.Stat(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			info.Warnings = append(info.Warnings, "svnserve.conf 不存在")
		} else {
			info.Warnings = append(info.Warnings, "无法读取 svnserve.conf 文件信息: "+err.Error())
		}
		return info
	}
	info.Exists = true
	info.Size = stat.Size()
	info.ModifiedAt = stat.ModTime().UTC().Format("2006-01-02T15:04:05Z07:00")

	data, err := os.ReadFile(configPath)
	if err != nil {
		info.Warnings = append(info.Warnings, "无法读取 svnserve.conf: "+err.Error())
		return info
	}
	info.Readable = true
	info.Content = string(data)
	if !utf8.Valid(data) {
		info.Warnings = append(info.Warnings, "svnserve.conf 不是有效 UTF-8")
		return info
	}

	parsed := parseSvnserveConfig(string(data))
	info.Warnings = append(info.Warnings, parsed.warnings...)
	info.PasswordDB = parsed.values["password-db"]
	info.PasswordDBResolved = resolveSvnserveConfigPath(configPath, info.PasswordDB)
	info.PasswordMatches = sameFilePath(info.PasswordDBResolved, passwdPath)
	info.AuthzDB = parsed.values["authz-db"]
	info.AuthzDBResolved = resolveSvnserveConfigPath(configPath, info.AuthzDB)
	info.AuthzMatches = sameFilePath(info.AuthzDBResolved, authzPath)
	info.Realm = parsed.values["realm"]
	info.AnonAccess = parsed.values["anon-access"]
	info.AuthAccess = parsed.values["auth-access"]

	if info.PasswordDB == "" {
		info.Warnings = append(info.Warnings, "svnserve.conf 未配置 password-db")
	} else if !info.PasswordMatches {
		info.Warnings = append(info.Warnings, fmt.Sprintf(
			"password-db 与面板的 passwd 不一致：%s",
			info.PasswordDBResolved,
		))
	}
	if info.AuthzDB == "" {
		info.Warnings = append(info.Warnings, "svnserve.conf 未配置 authz-db")
	} else if !info.AuthzMatches {
		info.Warnings = append(info.Warnings, fmt.Sprintf(
			"authz-db 与面板的 authz 不一致：%s",
			info.AuthzDBResolved,
		))
	}
	return info
}

func parseSvnserveConfig(content string) parsedSvnserveConfig {
	result := parsedSvnserveConfig{
		values:   map[string]string{},
		warnings: []string{},
	}
	section := ""
	sawGeneral := false
	scanner := bufio.NewScanner(strings.NewReader(content))
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(strings.TrimSpace(line[1 : len(line)-1]))
			if section == "general" {
				sawGeneral = true
			}
			continue
		}
		if section != "general" {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			result.warnings = append(result.warnings, fmt.Sprintf(
				"svnserve.conf 第 %d 行无法解析",
				lineNumber,
			))
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		if key == "" {
			result.warnings = append(result.warnings, fmt.Sprintf(
				"svnserve.conf 第 %d 行缺少配置名",
				lineNumber,
			))
			continue
		}
		if _, exists := result.values[key]; exists {
			result.warnings = append(result.warnings, fmt.Sprintf(
				"svnserve.conf 第 %d 行重复配置 %s，以最后一行为准",
				lineNumber,
				key,
			))
		}
		result.values[key] = strings.TrimSpace(value)
	}
	if err := scanner.Err(); err != nil {
		result.warnings = append(result.warnings, "读取 svnserve.conf 内容失败: "+err.Error())
	}
	if !sawGeneral {
		result.warnings = append(result.warnings, "svnserve.conf 缺少 [general] 配置段")
	}
	return result
}

func resolveSvnserveConfigPath(configPath, configured string) string {
	configured = strings.TrimSpace(configured)
	if configured == "" || strings.HasPrefix(configured, "^/") || strings.HasPrefix(configured, "file://") {
		return configured
	}
	if filepath.IsAbs(configured) {
		return filepath.Clean(configured)
	}
	return filepath.Clean(filepath.Join(filepath.Dir(configPath), configured))
}

func sameFilePath(left, right string) bool {
	if left == "" || right == "" {
		return false
	}
	leftAbs, leftErr := filepath.Abs(left)
	rightAbs, rightErr := filepath.Abs(right)
	if leftErr == nil {
		left = filepath.Clean(leftAbs)
	}
	if rightErr == nil {
		right = filepath.Clean(rightAbs)
	}
	leftInfo, leftStatErr := os.Stat(left)
	rightInfo, rightStatErr := os.Stat(right)
	if leftStatErr == nil && rightStatErr == nil {
		return os.SameFile(leftInfo, rightInfo)
	}
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}
