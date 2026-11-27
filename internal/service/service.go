package service

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"svnpanel/internal/audit"
	"svnpanel/internal/cfg"
	"svnpanel/internal/dto"
	"svnpanel/internal/svncfg"
)

type Service struct {
	Config   *cfg.Config
	Manager  *svncfg.Manager
	Scanner  svncfg.Scanner
	Audit    *audit.Store
	Sessions *SessionManager
	limiter  *loginLimiter
}

func New(c *cfg.Config) *Service {
	manager := svncfg.NewManager(c.SVN.PasswdFile, c.SVN.AuthzFile, c.Backup.Dir, c.Backup.Keep)
	return &Service{
		Config:  c,
		Manager: manager,
		Scanner: svncfg.Scanner{
			RepoRoot:     c.SVN.RepoRoot,
			SvnlookPath:  c.SVN.SvnlookPath,
			Timeout:      time.Duration(c.SVN.CommandTimeoutSec) * time.Second,
			MaxOutput:    c.SVN.MaxCommandOutputMiB << 20,
			MaxPathCount: c.SVN.MaxPathsPerLevel,
		},
		Audit:    audit.NewStore(c.Audit.Dir),
		Sessions: NewSessionManager(time.Duration(c.Session.IdleMinutes)*time.Minute, time.Duration(c.Session.AbsoluteHours)*time.Hour),
		limiter:  newLoginLimiter(),
	}
}

func (s *Service) Validate() error {
	if err := s.Manager.Validate(); err != nil {
		return err
	}
	info, err := s.Manager.Read()
	if err != nil {
		return err
	}
	_ = info
	if err := s.Scanner.Check(context.Background()); err != nil {
		return err
	}
	if _, err := s.Scanner.Discover(context.Background()); err != nil {
		return err
	}
	return nil
}

func (s *Service) Login(username, password, clientIP string) (dto.LoginResult, Session, error) {
	key := "login:" + clientIP
	if !s.limiter.Allowed(key) {
		return dto.LoginResult{}, Session{}, ErrRateLimited
	}
	userOK := subtle.ConstantTimeCompare([]byte(username), []byte(s.Config.Admin.Username)) == 1
	passOK := subtle.ConstantTimeCompare([]byte(password), []byte(s.Config.Admin.Password)) == 1
	if !userOK || !passOK {
		s.limiter.Fail(key)
		_ = s.Audit.Append(audit.Event{
			Actor: username, Action: "auth.login", Outcome: "failure", ClientIP: clientIP,
			Message: "管理员身份验证失败",
		})
		return dto.LoginResult{}, Session{}, ErrUnauthorized
	}
	if err := s.Audit.Append(audit.Event{
		Actor: username, Action: "auth.login", Outcome: "success", ClientIP: clientIP,
	}); err != nil {
		return dto.LoginResult{}, Session{}, fmt.Errorf("写入审计失败: %w", err)
	}
	s.limiter.Reset(key)
	session, err := s.Sessions.Create(username)
	if err != nil {
		return dto.LoginResult{}, Session{}, fmt.Errorf("创建会话失败: %w", err)
	}
	return dto.LoginResult{
		Username: username, CSRF: session.CSRF,
		ExpiresAt: session.CreatedAt.Add(time.Duration(s.Config.Session.AbsoluteHours) * time.Hour).Format(time.RFC3339),
	}, session, nil
}

func (s *Service) Authenticate(token string) (Session, error) {
	session, ok := s.Sessions.Get(token)
	if !ok {
		return Session{}, ErrUnauthorized
	}
	return session, nil
}

func (s *Service) Logout(session Session, clientIP string) {
	s.Sessions.Delete(session.Token)
	_ = s.Audit.Append(audit.Event{
		Actor: session.Username, Action: "auth.logout", Outcome: "success", ClientIP: clientIP,
	})
}

func (s *Service) ListUsers() (dto.UsersResponse, error) {
	state, err := s.Manager.Read()
	if err != nil {
		return dto.UsersResponse{}, err
	}
	items := make([]dto.User, 0, len(state.Passwd.Users))
	for _, user := range state.Passwd.Users {
		items = append(items, dto.User{
			Name: user.Name, Disabled: user.Disabled, Encoding: user.Encoding,
			Remark: user.Remark, Department: user.Department,
		})
	}
	sortUsers(items)
	return dto.UsersResponse{
		Items: items, Revisions: state.Revisions,
		Departments: append([]string(nil), state.Passwd.Departments...),
	}, nil
}

func (s *Service) GetSvnserveInfo(ctx context.Context) dto.SvnserveInfo {
	port := s.Config.SVN.SvnservePort
	if port <= 0 {
		port = 3690
	}
	executable := strings.TrimSpace(s.Config.SVN.SvnservePath)
	if executable == "" {
		executable = "svnserve"
	}
	info := dto.SvnserveInfo{
		Executable: executable,
		Port:       port,
		RepoRoot:   s.Config.SVN.RepoRoot,
		Passwd:     svnserveFileInfo(s.Config.SVN.PasswdFile),
		Authz:      svnserveFileInfo(s.Config.SVN.AuthzFile),
		CheckedAt:  time.Now().UTC().Format(time.RFC3339),
	}
	var issues []string

	resolved, err := exec.LookPath(executable)
	if err != nil {
		issues = append(issues, "svnserve 可执行文件不可用: "+err.Error())
	} else {
		info.ResolvedPath = resolved
		timeout := time.Duration(s.Config.SVN.CommandTimeoutSec) * time.Second
		if timeout <= 0 {
			timeout = 5 * time.Second
		}
		commandCtx, cancel := context.WithTimeout(ctx, timeout)
		output, commandErr := exec.CommandContext(commandCtx, resolved, "--version").CombinedOutput()
		cancel()
		if commandErr != nil {
			issues = append(issues, "svnserve --version 执行失败: "+commandErr.Error())
		} else {
			info.Available = true
			info.VersionOutput = strings.TrimSpace(string(output))
			for _, line := range strings.Split(info.VersionOutput, "\n") {
				if line = strings.TrimSpace(line); line != "" {
					info.Version = line
					break
				}
			}
		}
	}

	connection, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), 1500*time.Millisecond)
	if err == nil {
		info.PortOpen = true
		_ = connection.Close()
	} else {
		issues = append(issues, fmt.Sprintf("端口 %d 无法连接: %v", port, err))
	}

	repositories, err := s.Scanner.Discover(ctx)
	if err != nil {
		issues = append(issues, "仓库扫描失败: "+err.Error())
		repositories = []svncfg.Repository{}
	}
	info.Repositories = repositories
	for _, repository := range repositories {
		info.TotalLatestRevision += repository.Revision
	}
	info.Error = strings.Join(issues, "\n")
	return info
}

func svnserveFileInfo(path string) dto.SvnserveFileInfo {
	info := dto.SvnserveFileInfo{Path: path}
	stat, err := os.Stat(path)
	if err != nil {
		return info
	}
	info.Exists = true
	info.Mode = fmt.Sprintf("%04o", stat.Mode().Perm())
	info.Size = stat.Size()
	info.ModifiedAt = stat.ModTime().UTC().Format(time.RFC3339)
	return info
}

func (s *Service) CreateUser(revisions svncfg.Revisions, name, password, remark, department, clientIP string) (dto.OperationResult, error) {
	ctx, cancel := s.operationContext()
	defer cancel()
	state, err := s.Manager.Apply(ctx, revisions,
		func(passwd *svncfg.PasswdDoc, _ *svncfg.AuthzDoc) error {
			normalized, err := validateDepartmentInDoc(passwd, department)
			if err != nil {
				return err
			}
			return passwd.AddUser(name, password, remark, normalized)
		},
		func(backup string) error {
			return s.Audit.Append(audit.Event{
				Actor: s.Config.Admin.Username, Action: "user.create", Outcome: "success",
				Target: name, ClientIP: clientIP,
				Details: map[string]any{"backup": backup},
			})
		})
	if err != nil {
		s.auditFailure("user.create", name, clientIP, err)
		return dto.OperationResult{}, err
	}
	return dto.OperationResult{Revisions: state.Revisions, Message: "用户已创建"}, nil
}

func (s *Service) SetUserMetadata(revisions svncfg.Revisions, name, remark, department, clientIP string) (dto.OperationResult, error) {
	ctx, cancel := s.operationContext()
	defer cancel()
	state, err := s.Manager.Apply(ctx, revisions,
		func(passwd *svncfg.PasswdDoc, _ *svncfg.AuthzDoc) error {
			normalized, err := validateDepartmentInDoc(passwd, department)
			if err != nil {
				return err
			}
			return passwd.SetMetadata(name, remark, normalized)
		},
		func(_ string) error {
			return s.Audit.Append(audit.Event{
				Actor: s.Config.Admin.Username, Action: "user.metadata.update", Outcome: "success",
				Target: name, ClientIP: clientIP,
			})
		})
	if err != nil {
		s.auditFailure("user.metadata.update", name, clientIP, err)
		return dto.OperationResult{}, err
	}
	return dto.OperationResult{Revisions: state.Revisions, Message: "用户备注已更新"}, nil
}

func (s *Service) GetRawFiles() (dto.RawFilesResponse, error) {
	state, err := s.Manager.Read()
	if err != nil {
		return dto.RawFilesResponse{}, err
	}
	return dto.RawFilesResponse{
		Passwd: dto.RawFile{
			Content:  string(state.RawPasswd),
			Revision: state.Revisions.Passwd,
		},
		Authz: dto.RawFile{
			Content:  string(state.RawAuthz),
			Revision: state.Revisions.Authz,
		},
		Revisions: state.Revisions,
	}, nil
}

func (s *Service) UpdateRawFile(
	revisions svncfg.Revisions,
	name, content, clientIP string,
) (dto.OperationResult, error) {
	kind, err := svncfg.ParseRawFileKind(name)
	if err != nil {
		return dto.OperationResult{}, err
	}
	oldRevision := revisions.Authz
	if kind == svncfg.RawFilePasswd {
		oldRevision = revisions.Passwd
	}
	ctx, cancel := s.operationContext()
	defer cancel()
	state, err := s.Manager.ApplyRaw(ctx, kind, revisions, []byte(content),
		func(_ string) error {
			return s.Audit.Append(audit.Event{
				Actor: s.Config.Admin.Username, Action: "file." + string(kind) + ".update",
				Outcome: "success", Target: string(kind), ClientIP: clientIP,
				Details: map[string]any{"oldRevision": oldRevision},
			})
		})
	if err != nil {
		s.auditFailure("file."+string(kind)+".update", string(kind), clientIP, err)
		return dto.OperationResult{}, err
	}
	return dto.OperationResult{Revisions: state.Revisions, Message: "原生文件已保存"}, nil
}

func (s *Service) SetUserDisabled(revisions svncfg.Revisions, name string, disabled bool, clientIP string) (dto.OperationResult, error) {
	action := "user.disable"
	if !disabled {
		action = "user.enable"
	}
	ctx, cancel := s.operationContext()
	defer cancel()
	state, err := s.Manager.Apply(ctx, revisions,
		func(passwd *svncfg.PasswdDoc, _ *svncfg.AuthzDoc) error {
			return passwd.SetDisabled(name, disabled)
		},
		func(_ string) error {
			return s.Audit.Append(audit.Event{
				Actor: s.Config.Admin.Username, Action: action, Outcome: "success", Target: name, ClientIP: clientIP,
			})
		})
	if err != nil {
		s.auditFailure(action, name, clientIP, err)
		return dto.OperationResult{}, err
	}
	return dto.OperationResult{Revisions: state.Revisions, Message: "用户状态已更新"}, nil
}

func (s *Service) ResetPassword(revisions svncfg.Revisions, name, password, clientIP string) (dto.OperationResult, error) {
	ctx, cancel := s.operationContext()
	defer cancel()
	state, err := s.Manager.Apply(ctx, revisions,
		func(passwd *svncfg.PasswdDoc, _ *svncfg.AuthzDoc) error {
			return passwd.SetPassword(name, password)
		},
		func(_ string) error {
			return s.Audit.Append(audit.Event{
				Actor: s.Config.Admin.Username, Action: "user.password.reset", Outcome: "success", Target: name, ClientIP: clientIP,
			})
		})
	if err != nil {
		s.auditFailure("user.password.reset", name, clientIP, err)
		return dto.OperationResult{}, err
	}
	return dto.OperationResult{Revisions: state.Revisions, Message: "密码已重置"}, nil
}

func (s *Service) RevealPassword(session Session, name, clientIP string) (dto.RevealResult, error) {
	state, err := s.Manager.Read()
	if err != nil {
		s.auditFailureAs(session.Username, "user.password.reveal", name, clientIP, err)
		return dto.RevealResult{}, err
	}
	user, ok := state.Passwd.Users[name]
	if !ok {
		s.auditFailureAs(session.Username, "user.password.reveal", name, clientIP, svncfg.ErrNotFound)
		return dto.RevealResult{}, fmt.Errorf("%w: 用户 %s", svncfg.ErrNotFound, name)
	}
	if err := s.Audit.Append(audit.Event{
		Actor: session.Username, Action: "user.password.reveal", Outcome: "success",
		Target: name, ClientIP: clientIP,
	}); err != nil {
		return dto.RevealResult{}, fmt.Errorf("审计写入失败，拒绝返回密码: %w", err)
	}
	return dto.RevealResult{Value: user.Value, Encoding: user.Encoding}, nil
}

func (s *Service) DeleteUser(revisions svncfg.Revisions, name, clientIP string) (dto.OperationResult, error) {
	ctx, cancel := s.operationContext()
	defer cancel()
	state, err := s.Manager.Apply(ctx, revisions,
		func(passwd *svncfg.PasswdDoc, authz *svncfg.AuthzDoc) error {
			refs := authz.UserReferences(name)
			if len(refs) > 0 {
				return fmt.Errorf("%w: 用户 %s 仍被 %d 处引用", svncfg.ErrConflict, name, len(refs))
			}
			return passwd.DeleteUser(name)
		},
		func(_ string) error {
			return s.Audit.Append(audit.Event{
				Actor: s.Config.Admin.Username, Action: "user.delete", Outcome: "success", Target: name, ClientIP: clientIP,
			})
		})
	if err != nil {
		s.auditFailure("user.delete", name, clientIP, err)
		return dto.OperationResult{}, err
	}
	return dto.OperationResult{Revisions: state.Revisions, Message: "用户已删除"}, nil
}

func (s *Service) AddDepartment(revisions svncfg.Revisions, name, clientIP string) (dto.OperationResult, error) {
	ctx, cancel := s.operationContext()
	defer cancel()
	state, err := s.Manager.Apply(ctx, revisions,
		func(passwd *svncfg.PasswdDoc, _ *svncfg.AuthzDoc) error {
			return passwd.AddDepartment(name)
		},
		func(_ string) error {
			return s.Audit.Append(audit.Event{
				Actor: s.Config.Admin.Username, Action: "department.create", Outcome: "success",
				Target: name, ClientIP: clientIP,
			})
		})
	if err != nil {
		s.auditFailure("department.create", name, clientIP, err)
		return dto.OperationResult{}, err
	}
	return dto.OperationResult{Revisions: state.Revisions, Message: "部门已添加"}, nil
}

func (s *Service) RenameDepartment(revisions svncfg.Revisions, oldName, newName, clientIP string) (dto.OperationResult, error) {
	ctx, cancel := s.operationContext()
	defer cancel()
	state, err := s.Manager.Apply(ctx, revisions,
		func(passwd *svncfg.PasswdDoc, _ *svncfg.AuthzDoc) error {
			return passwd.RenameDepartment(oldName, newName)
		},
		func(_ string) error {
			return s.Audit.Append(audit.Event{
				Actor: s.Config.Admin.Username, Action: "department.rename", Outcome: "success",
				Target: oldName, ClientIP: clientIP,
				Details: map[string]any{"newName": newName},
			})
		})
	if err != nil {
		s.auditFailure("department.rename", oldName, clientIP, err)
		return dto.OperationResult{}, err
	}
	return dto.OperationResult{Revisions: state.Revisions, Message: "部门已重命名"}, nil
}

func (s *Service) DeleteDepartment(revisions svncfg.Revisions, name, clientIP string) (dto.OperationResult, error) {
	ctx, cancel := s.operationContext()
	defer cancel()
	state, err := s.Manager.Apply(ctx, revisions,
		func(passwd *svncfg.PasswdDoc, _ *svncfg.AuthzDoc) error {
			return passwd.DeleteDepartment(name)
		},
		func(_ string) error {
			return s.Audit.Append(audit.Event{
				Actor: s.Config.Admin.Username, Action: "department.delete", Outcome: "success",
				Target: name, ClientIP: clientIP,
			})
		})
	if err != nil {
		s.auditFailure("department.delete", name, clientIP, err)
		return dto.OperationResult{}, err
	}
	return dto.OperationResult{Revisions: state.Revisions, Message: "部门已删除"}, nil
}

func (s *Service) BatchUsers(
	revisions svncfg.Revisions,
	names []string,
	action, password, department, clientIP string,
) (dto.OperationResult, error) {
	uniqueNames := make([]string, 0, len(names))
	seen := make(map[string]bool)
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		uniqueNames = append(uniqueNames, name)
	}
	if len(uniqueNames) == 0 {
		return dto.OperationResult{}, fmt.Errorf("%w: 至少选择一个用户", svncfg.ErrValidation)
	}
	if action == "reset_password" {
		if err := svncfg.ValidatePassword(password); err != nil {
			return dto.OperationResult{}, err
		}
	}
	switch action {
	case "enable", "disable", "reset_password", "set_department":
	default:
		return dto.OperationResult{}, fmt.Errorf("%w: 不支持的批量操作 %s", svncfg.ErrValidation, action)
	}

	ctx, cancel := s.operationContext()
	defer cancel()
	state, err := s.Manager.Apply(ctx, revisions,
		func(passwd *svncfg.PasswdDoc, _ *svncfg.AuthzDoc) error {
			if action == "set_department" {
				var err error
				department, err = validateDepartmentInDoc(passwd, department)
				if err != nil {
					return err
				}
			}
			for _, name := range uniqueNames {
				if _, ok := passwd.Users[name]; !ok {
					return fmt.Errorf("%w: 用户 %s", svncfg.ErrNotFound, name)
				}
			}
			for _, name := range uniqueNames {
				switch action {
				case "enable":
					if err := passwd.SetDisabled(name, false); err != nil {
						return err
					}
				case "disable":
					if err := passwd.SetDisabled(name, true); err != nil {
						return err
					}
				case "reset_password":
					if err := passwd.SetPassword(name, password); err != nil {
						return err
					}
				case "set_department":
					user := passwd.Users[name]
					if err := passwd.SetMetadata(name, user.Remark, department); err != nil {
						return err
					}
				}
			}
			return nil
		},
		func(_ string) error {
			return s.Audit.Append(audit.Event{
				Actor: s.Config.Admin.Username, Action: "user.batch." + action, Outcome: "success",
				ClientIP: clientIP,
				Details:  map[string]any{"names": uniqueNames, "department": department},
			})
		})
	if err != nil {
		s.auditFailure("user.batch."+action, strings.Join(uniqueNames, ","), clientIP, err)
		return dto.OperationResult{}, err
	}
	return dto.OperationResult{Revisions: state.Revisions, Message: "批量操作已完成"}, nil
}

func (s *Service) ListGroups() (dto.GroupsResponse, error) {
	state, err := s.Manager.Read()
	if err != nil {
		return dto.GroupsResponse{}, err
	}
	groups, err := state.Authz.Groups()
	if err != nil {
		return dto.GroupsResponse{}, err
	}
	items := make([]dto.Group, 0, len(groups))
	for _, group := range groups {
		items = append(items, dto.Group{
			Name: group.Name, Members: group.Members, Unsupported: group.Unsupported, Remark: group.Remark,
			Permissions: state.Authz.GroupPermissions(group.Name),
		})
	}
	sort.Slice(items, func(i, j int) bool { return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name) })
	return dto.GroupsResponse{Items: items, Revisions: state.Revisions}, nil
}

func (s *Service) CreateGroup(revisions svncfg.Revisions, name, remark string, members []string, clientIP string) (dto.OperationResult, error) {
	ctx, cancel := s.operationContext()
	defer cancel()
	state, err := s.Manager.Apply(ctx, revisions,
		func(passwd *svncfg.PasswdDoc, authz *svncfg.AuthzDoc) error {
			for _, member := range members {
				if _, ok := passwd.Users[member]; !ok {
					return fmt.Errorf("%w: 成员 %s 不存在", svncfg.ErrValidation, member)
				}
			}
			return authz.AddGroup(name, remark, members)
		},
		func(_ string) error {
			return s.Audit.Append(audit.Event{
				Actor: s.Config.Admin.Username, Action: "group.create", Outcome: "success", Target: name, ClientIP: clientIP,
			})
		})
	if err != nil {
		s.auditFailure("group.create", name, clientIP, err)
		return dto.OperationResult{}, err
	}
	return dto.OperationResult{Revisions: state.Revisions, Message: "分组已创建"}, nil
}

func (s *Service) SetGroupMembers(revisions svncfg.Revisions, name string, members []string, remark *string, clientIP string) (dto.OperationResult, error) {
	ctx, cancel := s.operationContext()
	defer cancel()
	state, err := s.Manager.Apply(ctx, revisions,
		func(passwd *svncfg.PasswdDoc, authz *svncfg.AuthzDoc) error {
			for _, member := range members {
				if _, ok := passwd.Users[member]; !ok {
					return fmt.Errorf("%w: 成员 %s 不存在", svncfg.ErrValidation, member)
				}
			}
			if len(members) == 0 && len(authz.GroupReferences(name)) > 0 {
				return fmt.Errorf("%w: 分组 %s 仍被 ACL 引用，不能清空成员", svncfg.ErrConflict, name)
			}
			if err := authz.SetGroupMembers(name, members); err != nil {
				return err
			}
			if remark != nil {
				return authz.SetGroupRemark(name, *remark)
			}
			return nil
		},
		func(_ string) error {
			return s.Audit.Append(audit.Event{
				Actor: s.Config.Admin.Username, Action: "group.members.update", Outcome: "success", Target: name, ClientIP: clientIP,
			})
		})
	if err != nil {
		s.auditFailure("group.members.update", name, clientIP, err)
		return dto.OperationResult{}, err
	}
	return dto.OperationResult{Revisions: state.Revisions, Message: "分组成员已更新"}, nil
}

func (s *Service) DeleteGroup(revisions svncfg.Revisions, name, clientIP string) (dto.OperationResult, error) {
	ctx, cancel := s.operationContext()
	defer cancel()
	state, err := s.Manager.Apply(ctx, revisions,
		func(_ *svncfg.PasswdDoc, authz *svncfg.AuthzDoc) error {
			refs := authz.GroupReferences(name)
			if len(refs) > 0 {
				return fmt.Errorf("%w: 分组 %s 仍被 %d 条 ACL 引用", svncfg.ErrConflict, name, len(refs))
			}
			return authz.DeleteGroup(name)
		},
		func(_ string) error {
			return s.Audit.Append(audit.Event{
				Actor: s.Config.Admin.Username, Action: "group.delete", Outcome: "success", Target: name, ClientIP: clientIP,
			})
		})
	if err != nil {
		s.auditFailure("group.delete", name, clientIP, err)
		return dto.OperationResult{}, err
	}
	return dto.OperationResult{Revisions: state.Revisions, Message: "分组已删除"}, nil
}

func (s *Service) SetGroupPermissions(
	revisions svncfg.Revisions,
	name string,
	permissions []svncfg.GroupPermission,
	clientIP string,
) (dto.OperationResult, error) {
	ctx, cancel := s.operationContext()
	defer cancel()
	state, err := s.Manager.Apply(ctx, revisions,
		func(_ *svncfg.PasswdDoc, authz *svncfg.AuthzDoc) error {
			group, err := authz.GroupByName(name)
			if err != nil {
				return err
			}
			if len(group.Members) == 0 && len(permissions) > 0 {
				return fmt.Errorf("%w: 空分组 %s 不能设置路径权限", svncfg.ErrValidation, name)
			}
			return authz.ReplaceGroupPermissions(name, permissions)
		},
		func(_ string) error {
			return s.Audit.Append(audit.Event{
				Actor: s.Config.Admin.Username, Action: "group.permissions.update", Outcome: "success",
				Target: name, ClientIP: clientIP,
				Details: map[string]any{"count": len(permissions)},
			})
		})
	if err != nil {
		s.auditFailure("group.permissions.update", name, clientIP, err)
		return dto.OperationResult{}, err
	}
	return dto.OperationResult{Revisions: state.Revisions, Message: "分组权限已更新"}, nil
}

func (s *Service) ListRepositories(ctx context.Context) ([]svncfg.Repository, error) {
	return s.Scanner.Discover(ctx)
}

func (s *Service) ListPaths(ctx context.Context, repo, parent string) ([]svncfg.PathNode, error) {
	return s.Scanner.Tree(ctx, repo, parent)
}

func (s *Service) GetACL(repo, repoPath, user string) (dto.ACLResponse, error) {
	state, err := s.Manager.Read()
	if err != nil {
		return dto.ACLResponse{}, err
	}
	entries, err := state.Authz.ACL(repo, repoPath)
	if err != nil {
		return dto.ACLResponse{}, err
	}
	result := dto.ACLResponse{Items: entries, Revisions: state.Revisions}
	if user != "" {
		effective := state.Authz.Effective(repo, repoPath, user)
		result.Effective = &effective
	}
	return result, nil
}

func (s *Service) SetACL(revisions svncfg.Revisions, repo, repoPath string, identity svncfg.ACLIdentity, permission svncfg.Permission, clientIP string) (dto.OperationResult, error) {
	ctx, cancel := s.operationContext()
	defer cancel()
	state, err := s.Manager.Apply(ctx, revisions,
		func(passwd *svncfg.PasswdDoc, authz *svncfg.AuthzDoc) error {
			groups, _ := authz.Groups()
			if identity.Kind == svncfg.IdentityUser {
				if _, ok := passwd.Users[identity.Name]; !ok {
					return fmt.Errorf("%w: 用户 %s 不存在", svncfg.ErrValidation, identity.Name)
				}
			}
			if identity.Kind == svncfg.IdentityGroup {
				found := false
				memberCount := 0
				for _, group := range groups {
					if group.Name == identity.Name {
						found = true
						memberCount = len(group.Members)
					}
				}
				if !found {
					return fmt.Errorf("%w: 分组 %s 不存在", svncfg.ErrValidation, identity.Name)
				}
				if memberCount == 0 {
					return fmt.Errorf("%w: 空分组 %s 不能直接授予路径权限", svncfg.ErrValidation, identity.Name)
				}
			}
			return authz.SetACL(repo, repoPath, identity, permission)
		},
		func(_ string) error {
			return s.Audit.Append(audit.Event{
				Actor: s.Config.Admin.Username, Action: "acl.update", Outcome: "success",
				Target: identity.Name, Repo: repo, Path: repoPath, ClientIP: clientIP,
				Details: map[string]any{"kind": identity.Kind, "permission": permission},
			})
		})
	if err != nil {
		s.auditFailure("acl.update", repo+":"+repoPath, clientIP, err)
		return dto.OperationResult{}, err
	}
	return dto.OperationResult{Revisions: state.Revisions, Message: "权限已更新"}, nil
}

func (s *Service) DeleteACL(revisions svncfg.Revisions, repo, repoPath string, identity svncfg.ACLIdentity, clientIP string) (dto.OperationResult, error) {
	ctx, cancel := s.operationContext()
	defer cancel()
	state, err := s.Manager.Apply(ctx, revisions,
		func(_ *svncfg.PasswdDoc, authz *svncfg.AuthzDoc) error {
			return authz.DeleteACL(repo, repoPath, identity)
		},
		func(_ string) error {
			return s.Audit.Append(audit.Event{
				Actor: s.Config.Admin.Username, Action: "acl.delete", Outcome: "success",
				Target: identity.Name, Repo: repo, Path: repoPath, ClientIP: clientIP,
				Details: map[string]any{"kind": identity.Kind},
			})
		})
	if err != nil {
		s.auditFailure("acl.delete", repo+":"+repoPath, clientIP, err)
		return dto.OperationResult{}, err
	}
	return dto.OperationResult{Revisions: state.Revisions, Message: "权限已移除"}, nil
}

func (s *Service) QueryAudit(filter audit.Filter) (audit.Result, error) {
	return s.Audit.Query(filter)
}

func (s *Service) operationContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 10*time.Second)
}

func validateDepartmentInDoc(passwd *svncfg.PasswdDoc, department string) (string, error) {
	department, err := svncfg.NormalizeDepartment(department)
	if err != nil {
		return "", err
	}
	if department == "" {
		return "", nil
	}
	for _, allowed := range passwd.Departments {
		if department == allowed {
			return department, nil
		}
	}
	return "", fmt.Errorf("%w: 部门 %s 不存在，请先在部门管理中添加", svncfg.ErrValidation, department)
}

func (s *Service) auditFailure(action, target, clientIP string, err error) {
	message := "操作失败"
	if err != nil {
		message = err.Error()
	}
	_ = s.Audit.Append(audit.Event{
		Actor: s.Config.Admin.Username, Action: action, Outcome: "failure",
		Target: target, ClientIP: clientIP, Message: message,
	})
}

func (s *Service) auditFailureAs(actor, action, target, clientIP string, err error) {
	message := "操作失败"
	if err != nil {
		message = err.Error()
	}
	_ = s.Audit.Append(audit.Event{
		Actor: actor, Action: action, Outcome: "failure",
		Target: target, ClientIP: clientIP, Message: message,
	})
}

func sortUsers(users []dto.User) {
	for i := 0; i < len(users); i++ {
		for j := i + 1; j < len(users); j++ {
			if strings.ToLower(users[j].Name) < strings.ToLower(users[i].Name) {
				users[i], users[j] = users[j], users[i]
			}
		}
	}
}
