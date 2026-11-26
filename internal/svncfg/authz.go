package svncfg

import (
	"errors"
	"fmt"
	"path"
	"strings"
	"unicode/utf8"
)

type Permission string

const (
	PermissionNone        Permission = "none"
	PermissionRead        Permission = "read"
	PermissionWrite       Permission = "write"
	PermissionUnsupported Permission = "unsupported"
)

type IdentityKind string

const (
	IdentityUser          IdentityKind = "user"
	IdentityGroup         IdentityKind = "group"
	IdentityAll           IdentityKind = "all"
	IdentityAuthenticated IdentityKind = "authenticated"
	IdentityAnonymous     IdentityKind = "anonymous"
	IdentityUnsupported   IdentityKind = "unsupported"
)

type ACLIdentity struct {
	Kind IdentityKind `json:"kind"`
	Name string       `json:"name,omitempty"`
}

type ACLEntry struct {
	RawIdentity string      `json:"rawIdentity"`
	Identity    ACLIdentity `json:"identity"`
	Permission  Permission  `json:"permission"`
	RawValue    string      `json:"rawValue"`
	Line        int         `json:"line"`
	Unsupported bool        `json:"unsupported"`
}

type AuthzGroup struct {
	Name        string
	Members     []string
	Line        int
	Unsupported bool
	Remark      string `json:"remark,omitempty"`
	metadata    entityMetadata
}

type GroupPermission struct {
	Repo       string     `json:"repo"`
	Path       string     `json:"path"`
	Permission Permission `json:"permission"`
}

type authzSection struct {
	Name      string
	Header    int
	End       int
	Repo      string
	Path      string
	Kind      string
	Generic   bool
	Duplicate bool
}

type AuthzDoc struct {
	Doc          *TextDocument
	Sections     []authzSection
	HasAliases   bool
	HasAmbiguous bool
}

func ParseAuthz(data []byte) (*AuthzDoc, error) {
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("%w: authz 不是有效 UTF-8", ErrValidation)
	}
	doc := &AuthzDoc{Doc: ParseDocument(data)}
	for i, line := range doc.Doc.Lines {
		trimmed := strings.TrimSpace(line.Text)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			name := strings.TrimSpace(trimmed[1 : len(trimmed)-1])
			section := authzSection{Name: name, Header: i, End: len(doc.Doc.Lines) - 1}
			switch name {
			case "groups":
				section.Kind = "groups"
			case "aliases":
				section.Kind = "aliases"
			default:
				section.Kind = "acl"
				if colon := strings.IndexByte(name, ':'); colon >= 0 {
					section.Repo = strings.TrimSpace(name[:colon])
					section.Path = NormalizePath(name[colon+1:])
				} else {
					section.Generic = true
					section.Path = NormalizePath(name)
				}
			}
			if len(doc.Sections) > 0 {
				doc.Sections[len(doc.Sections)-1].End = i - 1
			}
			doc.Sections = append(doc.Sections, section)
		}
	}
	doc.detectAliases()
	doc.detectDuplicates()
	return doc, nil
}

func (d *AuthzDoc) detectAliases() {
	for _, section := range d.Sections {
		if section.Kind != "aliases" {
			continue
		}
		for i := section.Header + 1; i <= section.End && i < len(d.Doc.Lines); i++ {
			line := strings.TrimSpace(d.Doc.Lines[i].Text)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			d.HasAliases = true
			return
		}
	}
}

func (d *AuthzDoc) detectDuplicates() {
	seen := make(map[string]int)
	for i := range d.Sections {
		section := &d.Sections[i]
		key := section.Kind + "\x00" + section.Name
		if section.Kind == "acl" {
			key = section.Kind + "\x00" + section.Repo + "\x00" + section.Path
		}
		if _, ok := seen[key]; ok {
			section.Duplicate = true
			d.HasAmbiguous = true
		} else {
			seen[key] = i
		}
	}
}

func NormalizePath(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if value == "" {
		return "/"
	}
	if !strings.HasPrefix(value, "/") {
		value = "/" + value
	}
	cleaned := path.Clean(value)
	if cleaned == "." {
		return "/"
	}
	return cleaned
}

func ValidateRepoPath(value string) error {
	if strings.ContainsAny(value, "\x00\r\n[]") {
		return fmt.Errorf("%w: 路径包含非法字符", ErrValidation)
	}
	parts := strings.Split(strings.TrimPrefix(value, "/"), "/")
	for _, part := range parts {
		if part == ".." {
			return fmt.Errorf("%w: 路径不能包含 ..", ErrValidation)
		}
	}
	return nil
}

func ParsePermission(value string) Permission {
	switch strings.TrimSpace(value) {
	case "":
		return PermissionNone
	case "r":
		return PermissionRead
	case "rw":
		return PermissionWrite
	default:
		return PermissionUnsupported
	}
}

func PermissionValue(permission Permission) (string, error) {
	switch permission {
	case PermissionNone:
		return "", nil
	case PermissionRead:
		return "r", nil
	case PermissionWrite:
		return "rw", nil
	default:
		return "", fmt.Errorf("%w: 不支持权限 %s", ErrValidation, permission)
	}
}

func parseACLIdentity(raw string) ACLIdentity {
	raw = strings.TrimSpace(raw)
	switch raw {
	case "*":
		return ACLIdentity{Kind: IdentityAll}
	case "$authenticated":
		return ACLIdentity{Kind: IdentityAuthenticated}
	case "$anonymous":
		return ACLIdentity{Kind: IdentityAnonymous}
	}
	if strings.HasPrefix(raw, "~") {
		return ACLIdentity{Kind: IdentityUnsupported, Name: raw}
	}
	if strings.HasPrefix(raw, "@") {
		name := strings.TrimPrefix(raw, "@")
		if identityNamePattern.MatchString(name) {
			return ACLIdentity{Kind: IdentityGroup, Name: name}
		}
		return ACLIdentity{Kind: IdentityUnsupported, Name: raw}
	}
	if identityNamePattern.MatchString(raw) {
		return ACLIdentity{Kind: IdentityUser, Name: raw}
	}
	return ACLIdentity{Kind: IdentityUnsupported, Name: raw}
}

func identityRaw(identity ACLIdentity) (string, error) {
	switch identity.Kind {
	case IdentityUser:
		if err := ValidateIdentityName(identity.Name); err != nil {
			return "", err
		}
		return identity.Name, nil
	case IdentityGroup:
		if err := ValidateIdentityName(identity.Name); err != nil {
			return "", err
		}
		return "@" + identity.Name, nil
	case IdentityAll:
		return "*", nil
	case IdentityAuthenticated:
		return "$authenticated", nil
	case IdentityAnonymous:
		return "$anonymous", nil
	default:
		return "", fmt.Errorf("%w: 不支持的身份类型", ErrValidation)
	}
}

func (d *AuthzDoc) ACL(repo, repoPath string) ([]ACLEntry, error) {
	section, err := d.findACLSection(repo, repoPath)
	if errors.Is(err, ErrNotFound) {
		return []ACLEntry{}, nil
	}
	if err != nil {
		return nil, err
	}
	return d.parseACLEntries(*section), nil
}

func (d *AuthzDoc) parseACLEntries(section authzSection) []ACLEntry {
	var entries []ACLEntry
	for i := section.Header + 1; i <= section.End && i < len(d.Doc.Lines); i++ {
		text := d.Doc.Lines[i].Text
		trimmed := strings.TrimSpace(text)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		eq := strings.IndexByte(trimmed, '=')
		if eq <= 0 {
			continue
		}
		rawIdentity := strings.TrimSpace(trimmed[:eq])
		rawValue := strings.TrimSpace(trimmed[eq+1:])
		identity := parseACLIdentity(rawIdentity)
		permission := ParsePermission(rawValue)
		entries = append(entries, ACLEntry{
			RawIdentity: rawIdentity,
			Identity:    identity,
			Permission:  permission,
			RawValue:    rawValue,
			Line:        i,
			Unsupported: identity.Kind == IdentityUnsupported || permission == PermissionUnsupported,
		})
	}
	return entries
}

func (d *AuthzDoc) findACLSection(repo, repoPath string) (*authzSection, error) {
	repoPath = NormalizePath(repoPath)
	var found *authzSection
	for i := range d.Sections {
		section := &d.Sections[i]
		if section.Kind != "acl" || section.Generic || section.Repo != repo || section.Path != repoPath {
			continue
		}
		if section.Duplicate || found != nil {
			return nil, fmt.Errorf("%w: 仓库 %s 路径 %s 存在重复 section", ErrAmbiguous, repo, repoPath)
		}
		found = section
	}
	if found == nil {
		return nil, ErrNotFound
	}
	return found, nil
}

func (d *AuthzDoc) Groups() ([]AuthzGroup, error) {
	section, err := d.groupsSection()
	if errors.Is(err, ErrNotFound) {
		return []AuthzGroup{}, nil
	}
	if err != nil {
		return nil, err
	}
	groups := make([]AuthzGroup, 0)
	seen := make(map[string]bool)
	for i := section.Header + 1; i <= section.End && i < len(d.Doc.Lines); i++ {
		trimmed := strings.TrimSpace(d.Doc.Lines[i].Text)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		eq := strings.IndexByte(trimmed, '=')
		if eq <= 0 {
			continue
		}
		name := strings.TrimSpace(trimmed[:eq])
		if seen[name] {
			return nil, fmt.Errorf("%w: 分组 %s 重复", ErrAmbiguous, name)
		}
		seen[name] = true
		metadata, err := parseMetadataBefore(d.Doc.Lines, i)
		if err != nil {
			return nil, fmt.Errorf("解析分组 %s 的元数据失败: %w", name, err)
		}
		if metadata.DepartmentSet {
			return nil, fmt.Errorf("%w: 分组 %s 不支持部门标记", ErrAmbiguous, name)
		}
		members, unsupported := parseMembers(trimmed[eq+1:])
		groups = append(groups, AuthzGroup{
			Name: name, Members: members, Line: i, Unsupported: unsupported,
			Remark: metadata.Remark, metadata: metadata,
		})
	}
	return groups, nil
}

func parseMembers(value string) ([]string, bool) {
	parts := strings.Split(value, ",")
	members := make([]string, 0, len(parts))
	unsupported := false
	for _, part := range parts {
		member := strings.TrimSpace(part)
		if member == "" {
			continue
		}
		if strings.HasPrefix(member, "@") || !identityNamePattern.MatchString(member) {
			unsupported = true
		}
		members = append(members, member)
	}
	return members, unsupported
}

func (d *AuthzDoc) groupsSection() (*authzSection, error) {
	var found *authzSection
	for i := range d.Sections {
		section := &d.Sections[i]
		if section.Kind != "groups" {
			continue
		}
		if section.Duplicate || found != nil {
			return nil, fmt.Errorf("%w: [groups] 重复", ErrAmbiguous)
		}
		found = section
	}
	if found == nil {
		return nil, ErrNotFound
	}
	return found, nil
}

func (d *AuthzDoc) AddGroup(name, remark string, members []string) error {
	if err := ValidateIdentityName(name); err != nil {
		return err
	}
	groups, err := d.Groups()
	if err != nil {
		return err
	}
	for _, group := range groups {
		if group.Name == name {
			return fmt.Errorf("%w: 分组 %s 已存在", ErrConflict, name)
		}
	}
	for _, member := range members {
		if err := ValidateIdentityName(member); err != nil {
			return fmt.Errorf("%w: 成员 %s", err, member)
		}
	}
	remark, err = NormalizeRemark(remark)
	if err != nil {
		return err
	}
	line := name + " = " + strings.Join(uniqueStrings(members), ", ")
	section, err := d.groupsSection()
	if errors.Is(err, ErrNotFound) {
		index := d.firstACLSectionLine()
		d.Doc.InsertLine(index, line)
		for _, text := range metadataLines(remark, "") {
			d.Doc.InsertLine(index, text)
		}
		d.Doc.InsertLine(index, "[groups]")
		d.reindexSections()
		return nil
	}
	if err != nil {
		return err
	}
	d.Doc.InsertLine(section.End+1, line)
	for _, text := range metadataLines(remark, "") {
		d.Doc.InsertLine(section.End+1, text)
	}
	d.reindexSections()
	return nil
}

func (d *AuthzDoc) SetGroupMembers(name string, members []string) error {
	group, err := d.group(name)
	if err != nil {
		return err
	}
	if group.Unsupported {
		return fmt.Errorf("%w: 分组 %s 含不支持的结构", ErrAmbiguous, name)
	}
	for _, member := range members {
		if err := ValidateIdentityName(member); err != nil {
			return fmt.Errorf("%w: 成员 %s", err, member)
		}
	}
	members = uniqueStrings(members)
	text := d.Doc.Lines[group.Line].Text
	eq := strings.IndexByte(text, '=')
	start := eq + 1
	for start < len(text) && (text[start] == ' ' || text[start] == '\t') {
		start++
	}
	d.Doc.Lines[group.Line].Text = replaceRange(text, start, len(text), strings.Join(members, ", "))
	return nil
}

func (d *AuthzDoc) DeleteGroup(name string) error {
	group, err := d.group(name)
	if err != nil {
		return err
	}
	if group.Unsupported {
		return fmt.Errorf("%w: 分组 %s 含不支持的结构", ErrAmbiguous, name)
	}
	start := group.metadata.StartLine
	if start < 0 || start > group.Line {
		start = group.Line
	}
	d.Doc.ReplaceBlock(start, group.Line+1, nil)
	d.reindexSections()
	return nil
}

func (d *AuthzDoc) SetGroupRemark(name, remark string) error {
	group, err := d.group(name)
	if err != nil {
		return err
	}
	if group.Unsupported {
		return fmt.Errorf("%w: 分组 %s 含不支持的结构", ErrAmbiguous, name)
	}
	remark, err = NormalizeRemark(remark)
	if err != nil {
		return err
	}
	start := group.metadata.StartLine
	if start < 0 || start > group.Line {
		start = group.Line
	}
	d.Doc.ReplaceBlock(start, group.Line, metadataLines(remark, ""))
	d.reindexSections()
	return nil
}

func (d *AuthzDoc) group(name string) (AuthzGroup, error) {
	groups, err := d.Groups()
	if err != nil {
		return AuthzGroup{}, err
	}
	for _, group := range groups {
		if group.Name == name {
			return group, nil
		}
	}
	return AuthzGroup{}, fmt.Errorf("%w: 分组 %s", ErrNotFound, name)
}

func (d *AuthzDoc) GroupByName(name string) (AuthzGroup, error) {
	return d.group(name)
}

func (d *AuthzDoc) SetACL(repo, repoPath string, identity ACLIdentity, permission Permission) error {
	if err := ValidateRepoPath(repoPath); err != nil {
		return err
	}
	repoPath = NormalizePath(repoPath)
	rawIdentity, err := identityRaw(identity)
	if err != nil {
		return err
	}
	rawPermission, err := PermissionValue(permission)
	if err != nil {
		return err
	}
	section, err := d.findACLSection(repo, repoPath)
	if errors.Is(err, ErrNotFound) {
		d.Doc.AppendLine("")
		d.Doc.AppendLine("[" + repo + ":" + repoPath + "]")
		d.Doc.AppendLine(rawIdentity + " = " + rawPermission)
		d.reindexSections()
		return nil
	}
	if err != nil {
		return err
	}
	matches := 0
	var target *ACLEntry
	for _, entry := range d.parseACLEntries(*section) {
		if entry.Identity == identity || (entry.Identity.Kind == identity.Kind && entry.Identity.Name == identity.Name) {
			matches++
			copy := entry
			target = &copy
		}
	}
	if matches > 1 {
		return fmt.Errorf("%w: 同一路径下身份 %s 重复", ErrAmbiguous, rawIdentity)
	}
	if target != nil {
		text := d.Doc.Lines[target.Line].Text
		eq := strings.IndexByte(text, '=')
		if eq < 0 {
			return fmt.Errorf("%w: 无法定位权限行", ErrAmbiguous)
		}
		start := eq + 1
		for start < len(text) && (text[start] == ' ' || text[start] == '\t') {
			start++
		}
		end := len(text)
		for end > start && (text[end-1] == ' ' || text[end-1] == '\t') {
			end--
		}
		d.Doc.Lines[target.Line].Text = replaceRange(text, start, end, rawPermission)
		return nil
	}
	d.Doc.InsertLine(section.End+1, rawIdentity+" = "+rawPermission)
	d.reindexSections()
	return nil
}

func (d *AuthzDoc) DeleteACL(repo, repoPath string, identity ACLIdentity) error {
	repoPath = NormalizePath(repoPath)
	section, err := d.findACLSection(repo, repoPath)
	if err != nil {
		return err
	}
	var matches []ACLEntry
	for _, entry := range d.parseACLEntries(*section) {
		if entry.Identity == identity || (entry.Identity.Kind == identity.Kind && entry.Identity.Name == identity.Name) {
			matches = append(matches, entry)
		}
	}
	if len(matches) == 0 {
		return fmt.Errorf("%w: 权限条目", ErrNotFound)
	}
	if len(matches) > 1 {
		return fmt.Errorf("%w: 同一路径下身份重复", ErrAmbiguous)
	}
	d.Doc.RemoveLine(matches[0].Line)
	d.reindexSections()
	return nil
}

func (d *AuthzDoc) firstACLSectionLine() int {
	for _, section := range d.Sections {
		if section.Kind == "acl" {
			return section.Header
		}
	}
	return len(d.Doc.Lines)
}

func (d *AuthzDoc) reindexSections() {
	// Re-parse from bytes so later edits always use current line indexes.
	parsed, err := ParseAuthz(d.Doc.Bytes())
	if err == nil {
		d.Sections = parsed.Sections
		d.HasAliases = parsed.HasAliases
		d.HasAmbiguous = parsed.HasAmbiguous
	}
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

type Reference struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
	Repo string `json:"repo,omitempty"`
	Path string `json:"path,omitempty"`
}

func (d *AuthzDoc) UserReferences(name string) []Reference {
	var refs []Reference
	groups, groupErr := d.Groups()
	if groupErr != nil || d.HasAmbiguous {
		refs = append(refs, Reference{Kind: "ambiguous", Name: "authz 存在歧义，无法确认全部引用"})
	}
	for _, group := range groups {
		for _, member := range group.Members {
			if member == name {
				refs = append(refs, Reference{Kind: "group", Name: group.Name})
			}
		}
	}
	for _, section := range d.Sections {
		if section.Kind != "acl" {
			continue
		}
		for _, entry := range d.parseACLEntries(section) {
			if entry.Identity.Kind == IdentityUser && entry.Identity.Name == name {
				refs = append(refs, Reference{Kind: "acl", Name: section.Repo, Repo: section.Repo, Path: section.Path})
			}
		}
	}
	return refs
}

func (d *AuthzDoc) GroupReferences(name string) []Reference {
	var refs []Reference
	if d.HasAmbiguous {
		refs = append(refs, Reference{Kind: "ambiguous", Name: "authz 存在歧义，无法确认全部引用"})
	}
	for _, section := range d.Sections {
		if section.Kind != "acl" {
			continue
		}
		for _, entry := range d.parseACLEntries(section) {
			if entry.Identity.Kind == IdentityGroup && entry.Identity.Name == name {
				refs = append(refs, Reference{Kind: "acl", Name: section.Repo, Repo: section.Repo, Path: section.Path})
			}
		}
	}
	return refs
}

func (d *AuthzDoc) GroupPermissions(name string) []GroupPermission {
	permissions := make([]GroupPermission, 0)
	for _, section := range d.Sections {
		if section.Kind != "acl" || section.Generic {
			continue
		}
		for _, entry := range d.parseACLEntries(section) {
			if entry.Identity.Kind == IdentityGroup && entry.Identity.Name == name {
				permissions = append(permissions, GroupPermission{
					Repo: section.Repo, Path: section.Path, Permission: entry.Permission,
				})
			}
		}
	}
	return permissions
}

func (d *AuthzDoc) ReplaceGroupPermissions(name string, desired []GroupPermission) error {
	type key struct {
		repo string
		path string
	}
	seen := make(map[key]bool)
	for _, item := range desired {
		item.Path = NormalizePath(item.Path)
		if err := ValidateRepoPath(item.Path); err != nil {
			return err
		}
		if strings.TrimSpace(item.Repo) == "" {
			return fmt.Errorf("%w: 仓库不能为空", ErrValidation)
		}
		k := key{repo: item.Repo, path: item.Path}
		if seen[k] {
			return fmt.Errorf("%w: 仓库 %s 路径 %s 重复", ErrValidation, item.Repo, item.Path)
		}
		seen[k] = true
		if _, err := PermissionValue(item.Permission); err != nil {
			return err
		}
	}

	existing := d.GroupPermissions(name)
	desiredKeys := make(map[key]bool)
	for _, item := range desired {
		desiredKeys[key{repo: item.Repo, path: NormalizePath(item.Path)}] = true
	}
	for _, item := range existing {
		if !desiredKeys[key{repo: item.Repo, path: NormalizePath(item.Path)}] {
			if err := d.DeleteACL(item.Repo, item.Path, ACLIdentity{Kind: IdentityGroup, Name: name}); err != nil {
				return err
			}
		}
	}
	for _, item := range desired {
		if err := d.SetACL(
			item.Repo, item.Path, ACLIdentity{Kind: IdentityGroup, Name: name}, item.Permission,
		); err != nil {
			return err
		}
	}
	return nil
}
