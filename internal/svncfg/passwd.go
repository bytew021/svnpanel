package svncfg

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

const disabledPrefix = "#disabled#"

var identityNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

type PasswdUser struct {
	Name       string
	Value      string
	Disabled   bool
	Line       int
	Encoding   string
	Remark     string `json:"remark,omitempty"`
	Department string `json:"department,omitempty"`
	metadata   entityMetadata
}

type PasswdDoc struct {
	Doc   *TextDocument
	Users map[string]PasswdUser
}

func ParsePasswd(data []byte) (*PasswdDoc, error) {
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("%w: passwd 不是有效 UTF-8", ErrValidation)
	}
	doc := ParseDocument(data)
	result := &PasswdDoc{Doc: doc, Users: make(map[string]PasswdUser)}
	for i, line := range doc.Lines {
		name, value, disabled, ok := parsePasswdLine(line.Text)
		if !ok {
			continue
		}
		if _, exists := result.Users[name]; exists {
			return nil, fmt.Errorf("%w: passwd 中存在重复用户 %s", ErrAmbiguous, name)
		}
		metadata, err := parseMetadataBefore(doc.Lines, i)
		if err != nil {
			return nil, fmt.Errorf("解析用户 %s 的元数据失败: %w", name, err)
		}
		result.Users[name] = PasswdUser{
			Name: name, Value: value, Disabled: disabled, Line: i, Encoding: PasswordEncoding(value),
			Remark: metadata.Remark, Department: metadata.Department, metadata: metadata,
		}
	}
	return result, nil
}

func parsePasswdLine(text string) (name, value string, disabled, ok bool) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return "", "", false, false
	}
	if strings.HasPrefix(trimmed, disabledPrefix) {
		disabled = true
		trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, disabledPrefix))
	}
	if strings.HasPrefix(trimmed, "#") {
		return "", "", false, false
	}
	eq := strings.IndexByte(trimmed, '=')
	if eq <= 0 {
		return "", "", false, false
	}
	name = strings.TrimSpace(trimmed[:eq])
	value = strings.TrimSpace(trimmed[eq+1:])
	if name == "" || identityNamePattern.MatchString(name) == false {
		return "", "", false, false
	}
	return name, value, disabled, true
}

func PasswordEncoding(value string) string {
	switch {
	case strings.HasPrefix(value, "$apr1$"):
		return "apr1"
	case strings.HasPrefix(value, "{SHA}"):
		return "sha1"
	case strings.HasPrefix(value, "$2a$"), strings.HasPrefix(value, "$2b$"), strings.HasPrefix(value, "$2y$"), strings.HasPrefix(value, "$2x$"):
		return "bcrypt"
	default:
		return "plain"
	}
}

func ValidateIdentityName(name string) error {
	if !identityNamePattern.MatchString(name) {
		return fmt.Errorf("%w: 名称只能包含字母、数字及 ._-，且必须以字母或数字开头", ErrValidation)
	}
	return nil
}

func ValidatePassword(password string) error {
	if len(password) < 8 || len(password) > 128 {
		return fmt.Errorf("%w: 密码长度应为 8-128 个字符", ErrValidation)
	}
	if strings.TrimSpace(password) != password {
		return fmt.Errorf("%w: 密码首尾不能包含空白", ErrValidation)
	}
	if strings.ContainsAny(password, "\x00\r\n") {
		return fmt.Errorf("%w: 密码不能包含换行或 NUL", ErrValidation)
	}
	return nil
}

func (d *PasswdDoc) AddUser(name, password, remark, department string) error {
	if err := ValidateIdentityName(name); err != nil {
		return err
	}
	if err := ValidatePassword(password); err != nil {
		return err
	}
	remark, err := NormalizeRemark(remark)
	if err != nil {
		return err
	}
	department, err = NormalizeDepartment(department)
	if err != nil {
		return err
	}
	if _, exists := d.Users[name]; exists {
		return fmt.Errorf("%w: 用户 %s 已存在", ErrConflict, name)
	}
	for _, text := range metadataLines(remark, department) {
		d.Doc.AppendLine(text)
	}
	d.Doc.AppendLine(name + " = " + password)
	d.reindex()
	return nil
}

func (d *PasswdDoc) SetDisabled(name string, disabled bool) error {
	user, ok := d.Users[name]
	if !ok {
		return fmt.Errorf("%w: 用户 %s", ErrNotFound, name)
	}
	if user.Disabled == disabled {
		return nil
	}
	line := d.Doc.Lines[user.Line].Text
	eq := strings.IndexByte(line, '=')
	if eq < 0 {
		return fmt.Errorf("%w: 无法定位用户行", ErrAmbiguous)
	}
	nameStart := findFirstNonSpace(line, 0, eq)
	nameEnd := eq
	for nameEnd > nameStart && (line[nameEnd-1] == ' ' || line[nameEnd-1] == '\t') {
		nameEnd--
	}
	target := name
	if disabled {
		target = disabledPrefix + name
	}
	d.Doc.Lines[user.Line].Text = replaceRange(line, nameStart, nameEnd, target)
	user.Disabled = disabled
	d.Users[name] = user
	d.reindex()
	return nil
}

func (d *PasswdDoc) SetPassword(name, password string) error {
	user, ok := d.Users[name]
	if !ok {
		return fmt.Errorf("%w: 用户 %s", ErrNotFound, name)
	}
	if err := ValidatePassword(password); err != nil {
		return err
	}
	line := d.Doc.Lines[user.Line].Text
	eq := strings.IndexByte(line, '=')
	if eq < 0 {
		return fmt.Errorf("%w: 无法定位用户行", ErrAmbiguous)
	}
	start := eq + 1
	for start < len(line) && (line[start] == ' ' || line[start] == '\t') {
		start++
	}
	end := len(line)
	for end > start && (line[end-1] == ' ' || line[end-1] == '\t') {
		end--
	}
	d.Doc.Lines[user.Line].Text = replaceRange(line, start, end, password)
	user.Value = password
	user.Encoding = PasswordEncoding(password)
	d.Users[name] = user
	d.reindex()
	return nil
}

func (d *PasswdDoc) DeleteUser(name string) error {
	user, ok := d.Users[name]
	if !ok {
		return fmt.Errorf("%w: 用户 %s", ErrNotFound, name)
	}
	d.Doc.RemoveLine(user.Line)
	delete(d.Users, name)
	for key, value := range d.Users {
		if value.Line > user.Line {
			value.Line--
			d.Users[key] = value
		}
	}
	d.reindex()
	return nil
}

func (d *PasswdDoc) SetMetadata(name, remark, department string) error {
	user, ok := d.Users[name]
	if !ok {
		return fmt.Errorf("%w: 用户 %s", ErrNotFound, name)
	}
	remark, err := NormalizeRemark(remark)
	if err != nil {
		return err
	}
	department, err = NormalizeDepartment(department)
	if err != nil {
		return err
	}
	start := user.metadata.StartLine
	if start < 0 || start > user.Line {
		start = user.Line
	}
	d.Doc.ReplaceBlock(start, user.Line, metadataLines(remark, department))
	d.reindex()
	return nil
}

func (d *PasswdDoc) reindex() {
	parsed, err := ParsePasswd(d.Doc.Bytes())
	if err == nil {
		d.Users = parsed.Users
	}
}

func findFirstNonSpace(s string, start, end int) int {
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	return start
}
