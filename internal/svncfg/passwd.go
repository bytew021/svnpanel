package svncfg

import (
	"fmt"
	"regexp"
	"sort"
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
	Doc         *TextDocument
	Users       map[string]PasswdUser
	Departments []string
}

func ParsePasswd(data []byte) (*PasswdDoc, error) {
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("%w: passwd 不是有效 UTF-8", ErrValidation)
	}
	doc := ParseDocument(data)
	result := &PasswdDoc{Doc: doc, Users: make(map[string]PasswdUser), Departments: []string{}}
	departmentSeen := make(map[string]bool)
	addDepartment := func(value string, explicit bool) error {
		normalized, err := NormalizeDepartment(value)
		if err != nil {
			return err
		}
		if explicit && normalized == "" {
			return fmt.Errorf("%w: 部门名称不能为空", ErrValidation)
		}
		if normalized == "" {
			return nil
		}
		if explicit && departmentSeen[normalized] {
			return fmt.Errorf("%w: passwd 中存在重复部门 %s", ErrAmbiguous, normalized)
		}
		if !departmentSeen[normalized] {
			departmentSeen[normalized] = true
			result.Departments = append(result.Departments, normalized)
			if len(result.Departments) > maxDepartments {
				return fmt.Errorf("%w: 部门不能超过 %d 个", ErrValidation, maxDepartments)
			}
		}
		return nil
	}
	for i, line := range doc.Lines {
		trimmed := strings.TrimSpace(line.Text)
		if strings.HasPrefix(trimmed, departmentsPrefix) {
			if err := addDepartment(strings.TrimPrefix(trimmed, departmentsPrefix), true); err != nil {
				return nil, fmt.Errorf("解析第 %d 行部门定义失败: %w", i+1, err)
			}
			continue
		}
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
		if err := addDepartment(metadata.Department, false); err != nil {
			return nil, fmt.Errorf("解析用户 %s 的部门失败: %w", name, err)
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

func (d *PasswdDoc) AddDepartment(name string) error {
	name, err := NormalizeDepartment(name)
	if err != nil {
		return err
	}
	if name == "" {
		return fmt.Errorf("%w: 部门名称不能为空", ErrValidation)
	}
	for _, current := range d.Departments {
		if current == name {
			return fmt.Errorf("%w: 部门 %s 已存在", ErrConflict, name)
		}
	}
	next := append(append([]string(nil), d.Departments...), name)
	if err := d.setDepartments(next); err != nil {
		return err
	}
	d.reindex()
	return nil
}

func (d *PasswdDoc) RenameDepartment(oldName, newName string) error {
	var err error
	oldName, err = NormalizeDepartment(oldName)
	if err != nil {
		return err
	}
	newName, err = NormalizeDepartment(newName)
	if err != nil {
		return err
	}
	if oldName == "" || newName == "" {
		return fmt.Errorf("%w: 部门名称不能为空", ErrValidation)
	}
	if oldName == newName {
		return nil
	}
	if !containsString(d.Departments, oldName) {
		return fmt.Errorf("%w: 部门 %s", ErrNotFound, oldName)
	}
	if containsString(d.Departments, newName) {
		return fmt.Errorf("%w: 部门 %s 已存在", ErrConflict, newName)
	}

	next := append([]string(nil), d.Departments...)
	for i, current := range next {
		if current == oldName {
			next[i] = newName
			break
		}
	}

	users := make([]string, 0)
	for name, user := range d.Users {
		if user.Department == oldName {
			users = append(users, name)
		}
	}
	sort.Slice(users, func(i, j int) bool {
		return d.Users[users[i]].Line > d.Users[users[j]].Line
	})
	for _, name := range users {
		user := d.Users[name]
		if err := d.SetMetadata(name, user.Remark, newName); err != nil {
			return err
		}
	}

	if err := d.setDepartments(next); err != nil {
		return err
	}
	d.reindex()
	return nil
}

func (d *PasswdDoc) DeleteDepartment(name string) error {
	var err error
	name, err = NormalizeDepartment(name)
	if err != nil {
		return err
	}
	if !containsString(d.Departments, name) {
		return fmt.Errorf("%w: 部门 %s", ErrNotFound, name)
	}
	for _, user := range d.Users {
		if user.Department == name {
			return fmt.Errorf("%w: 部门 %s 仍被用户 %s 使用", ErrConflict, name, user.Name)
		}
	}
	next := make([]string, 0, len(d.Departments)-1)
	for _, current := range d.Departments {
		if current != name {
			next = append(next, current)
		}
	}
	if err := d.setDepartments(next); err != nil {
		return err
	}
	d.reindex()
	return nil
}

func (d *PasswdDoc) setDepartments(departments []string) error {
	normalized, err := normalizeDepartments(departments)
	if err != nil {
		return err
	}
	for i := len(d.Doc.Lines) - 1; i >= 0; i-- {
		if strings.HasPrefix(strings.TrimSpace(d.Doc.Lines[i].Text), departmentsPrefix) {
			d.Doc.RemoveLine(i)
		}
	}
	for i, department := range normalized {
		d.Doc.InsertLine(i, departmentsPrefix+" "+department)
	}
	return nil
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func (d *PasswdDoc) reindex() {
	parsed, err := ParsePasswd(d.Doc.Bytes())
	if err == nil {
		d.Users = parsed.Users
		d.Departments = parsed.Departments
	}
}

func findFirstNonSpace(s string, start, end int) int {
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	return start
}
