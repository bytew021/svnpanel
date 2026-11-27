package center_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"svnpanel/internal/app/center"
	"svnpanel/internal/cfg"
	"svnpanel/internal/service"
	"svnpanel/internal/svncfg"
	"svnpanel/pkg/httpx"

	"github.com/gin-gonic/gin"
)

type testAPI struct {
	t       *testing.T
	handler http.Handler
	cookie  *http.Cookie
	csrf    string
	revs    svncfg.Revisions
}

func TestUserGroupACLFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	passwdPath := filepath.Join(dir, "passwd")
	authzPath := filepath.Join(dir, "authz")
	repoRoot := filepath.Join(dir, "repos")
	if err := os.MkdirAll(repoRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(passwdPath, []byte(
		"# svnpanel:departments: 部门A\n# svnpanel:departments: 部门B\n# users\nalice = old-password\n#disabled#bob = disabled-password\n",
	), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(authzPath, []byte("# acl\n[groups]\nqa = bob\n\n[demo:/trunk]\nqa = r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "svnserve.conf"), []byte(
		"[general]\npassword-db = passwd\nauthz-db = authz\nrealm = test\n",
	), 0o600); err != nil {
		t.Fatal(err)
	}
	config := &cfg.Config{
		Global: cfg.GlobalConfig{Title: "test", RunMode: "release"},
		HTTP:   cfg.HTTPConfig{Host: "127.0.0.1", Port: 8080, MaxContentLength: 1 << 20},
		Admin:  cfg.AdminConfig{Username: "admin", Password: "admin-pass"},
		SVN: cfg.SVNConfig{
			RepoRoot: repoRoot, PasswdFile: passwdPath, AuthzFile: authzPath,
			SvnlookPath: "svnlook", SvnservePath: "svnserve", SvnservePort: 3690,
			CommandTimeoutSec: 2, MaxCommandOutputMiB: 1, MaxPathsPerLevel: 100,
		},
		Audit:   cfg.AuditConfig{Dir: filepath.Join(dir, "audit")},
		Backup:  cfg.BackupConfig{Dir: filepath.Join(dir, "backups"), Keep: 50},
		Session: cfg.SessionConfig{IdleMinutes: 30, AbsoluteHours: 8, CookieSecure: false},
	}
	svc := service.New(config)
	if err := svc.Manager.Validate(); err != nil {
		t.Fatal(err)
	}
	engine := httpx.NewEngine(*config)
	center.Config(engine, svc)
	api := &testAPI{t: t, handler: engine}

	sessionResponse := api.do(http.MethodGet, "/api/v1/auth/session", nil, false, false)
	if sessionResponse.Code != http.StatusOK {
		t.Fatalf("session probe failed: %d %s", sessionResponse.Code, sessionResponse.Body.String())
	}

	login := api.do(http.MethodPost, "/api/v1/auth/login", map[string]any{
		"username": "admin", "password": "admin-pass",
	}, false, false)
	if login.Code != http.StatusOK {
		t.Fatalf("login failed: %d %s", login.Code, login.Body.String())
	}
	for _, cookie := range login.Result().Cookies() {
		if cookie.Name == "svnpanel_session" {
			api.cookie = cookie
		}
	}
	if api.cookie == nil {
		t.Fatal("session cookie missing")
	}
	var loginData struct {
		Data struct {
			CSRF string `json:"csrf"`
		} `json:"data"`
	}
	decode(t, login, &loginData)
	api.csrf = loginData.Data.CSRF

	svnserveResponse := api.do(http.MethodGet, "/api/v1/svnserve", nil, true, false)
	var svnserveInfo struct {
		Data struct {
			Executable     string              `json:"executable"`
			Port           int                 `json:"port"`
			RepoRoot       string              `json:"repoRoot"`
			Items          []svncfg.Repository `json:"repositories"`
			SvnserveConfig struct {
				Path            string `json:"path"`
				PasswordMatches bool   `json:"passwordMatches"`
				AuthzMatches    bool   `json:"authzMatches"`
				Realm           string `json:"realm"`
			} `json:"svnserveConfig"`
		} `json:"data"`
	}
	decode(t, svnserveResponse, &svnserveInfo)
	if svnserveInfo.Data.Executable != "svnserve" || svnserveInfo.Data.Port != 3690 {
		t.Fatalf("unexpected svnserve info: %+v", svnserveInfo.Data)
	}
	if svnserveInfo.Data.Items == nil {
		t.Fatal("empty svnserve repository list must be encoded as []")
	}
	if !svnserveInfo.Data.SvnserveConfig.PasswordMatches ||
		!svnserveInfo.Data.SvnserveConfig.AuthzMatches ||
		svnserveInfo.Data.SvnserveConfig.Realm != "test" {
		t.Fatalf("unexpected svnserve config: %+v", svnserveInfo.Data.SvnserveConfig)
	}

	repositories := api.do(http.MethodGet, "/api/v1/repositories", nil, true, false)
	var repositoryData struct {
		Data struct {
			Items []any `json:"items"`
		} `json:"data"`
	}
	decode(t, repositories, &repositoryData)
	if repositoryData.Data.Items == nil {
		t.Fatal("empty repository list must be encoded as []")
	}

	usersResp := api.do(http.MethodGet, "/api/v1/users", nil, true, false)
	var users struct {
		Data struct {
			Items []struct {
				Name string `json:"name"`
			} `json:"items"`
			Departments []string         `json:"departments"`
			Revisions   svncfg.Revisions `json:"revisions"`
		} `json:"data"`
	}
	decode(t, usersResp, &users)
	api.revs = users.Data.Revisions
	if len(users.Data.Items) != 2 {
		t.Fatalf("unexpected users: %+v", users.Data.Items)
	}
	if len(users.Data.Departments) != 2 {
		t.Fatalf("unexpected departments: %+v", users.Data.Departments)
	}

	rawFilesResponse := api.do(http.MethodGet, "/api/v1/files", nil, true, false)
	var rawFiles struct {
		Data struct {
			Passwd struct {
				Content string `json:"content"`
			} `json:"passwd"`
			Authz struct {
				Content string `json:"content"`
			} `json:"authz"`
		} `json:"data"`
	}
	decode(t, rawFilesResponse, &rawFiles)
	if !strings.Contains(rawFiles.Data.Passwd.Content, "alice = old-password") {
		t.Fatalf("raw passwd content missing: %q", rawFiles.Data.Passwd.Content)
	}
	rawAuthz := rawFiles.Data.Authz.Content + "\n# raw editor comment\n"
	rawUpdate := api.do(http.MethodPut, "/api/v1/files/authz", map[string]any{
		"passwd": api.revs.Passwd, "authz": api.revs.Authz, "content": rawAuthz,
	}, true, true)
	if rawUpdate.Code != http.StatusOK {
		t.Fatalf("raw authz update failed: %d %s", rawUpdate.Code, rawUpdate.Body.String())
	}
	api.readRevisions(rawUpdate, true)
	rawFilesResponse = api.do(http.MethodGet, "/api/v1/files", nil, true, false)
	decode(t, rawFilesResponse, &rawFiles)
	if !strings.Contains(rawFiles.Data.Authz.Content, "# raw editor comment") {
		t.Fatal("raw authz update was not persisted")
	}

	snapshotsResponse := api.do(http.MethodGet, "/api/v1/snapshots", nil, true, false)
	var snapshots struct {
		Data struct {
			Items []struct {
				ID string `json:"id"`
			} `json:"items"`
		} `json:"data"`
	}
	decode(t, snapshotsResponse, &snapshots)
	if len(snapshots.Data.Items) == 0 {
		t.Fatal("snapshot list is empty")
	}
	snapshotID := snapshots.Data.Items[0].ID
	snapshotResponse := api.do(
		http.MethodGet, "/api/v1/snapshots/"+url.PathEscape(snapshotID), nil, true, false,
	)
	var snapshotContent struct {
		Data struct {
			Authz struct {
				Content string `json:"content"`
			} `json:"authz"`
		} `json:"data"`
	}
	decode(t, snapshotResponse, &snapshotContent)
	if strings.Contains(snapshotContent.Data.Authz.Content, "# raw editor comment") {
		t.Fatal("snapshot contains post-edit content")
	}
	restoreResponse := api.do(
		http.MethodPost, "/api/v1/snapshots/"+url.PathEscape(snapshotID)+"/restore",
		map[string]any{"passwd": api.revs.Passwd, "authz": api.revs.Authz}, true, true,
	)
	if restoreResponse.Code != http.StatusOK {
		t.Fatalf("snapshot restore failed: %d %s", restoreResponse.Code, restoreResponse.Body.String())
	}
	api.readRevisions(restoreResponse, true)

	addedDepartment := api.do(http.MethodPost, "/api/v1/departments", map[string]any{
		"passwd": api.revs.Passwd, "authz": api.revs.Authz, "name": "部门C",
	}, true, true)
	if addedDepartment.Code != http.StatusCreated {
		t.Fatalf("add department failed: %d %s", addedDepartment.Code, addedDepartment.Body.String())
	}
	api.readRevisions(addedDepartment, true)

	renamedDepartment := api.do(http.MethodPut, "/api/v1/departments/"+url.PathEscape("部门C"), map[string]any{
		"passwd": api.revs.Passwd, "authz": api.revs.Authz, "name": "部门D",
	}, true, true)
	if renamedDepartment.Code != http.StatusOK {
		t.Fatalf("rename department failed: %d %s", renamedDepartment.Code, renamedDepartment.Body.String())
	}
	api.readRevisions(renamedDepartment, false)

	created := api.do(http.MethodPost, "/api/v1/users", map[string]any{
		"passwd": api.revs.Passwd, "authz": api.revs.Authz,
		"name": "carol", "password": "carol-password", "remark": "示例成员", "department": "部门D",
		"groups": []string{"qa"},
	}, true, true)
	if created.Code != http.StatusCreated {
		t.Fatalf("create user failed: %d %s", created.Code, created.Body.String())
	}
	api.readRevisions(created, true)

	groupsAfterCreate := api.do(http.MethodGet, "/api/v1/groups", nil, true, false)
	var createdUserGroups struct {
		Data struct {
			Items []struct {
				Name    string   `json:"name"`
				Members []string `json:"members"`
			} `json:"items"`
		} `json:"data"`
	}
	decode(t, groupsAfterCreate, &createdUserGroups)
	qaMembers := []string{}
	for _, group := range createdUserGroups.Data.Items {
		if group.Name == "qa" {
			qaMembers = group.Members
		}
	}
	if strings.Join(qaMembers, ",") != "bob,carol" {
		t.Fatalf("created user was not added to selected group: %+v", qaMembers)
	}

	failedCreate := api.do(http.MethodPost, "/api/v1/users", map[string]any{
		"passwd": api.revs.Passwd, "authz": api.revs.Authz,
		"name": "dave", "password": "dave-password", "groups": []string{"missing-group"},
	}, true, true)
	if failedCreate.Code != http.StatusNotFound {
		t.Fatalf("expected missing group failure, got %d %s", failedCreate.Code, failedCreate.Body.String())
	}
	usersAfterFailure := api.do(http.MethodGet, "/api/v1/users", nil, true, false)
	var afterFailure struct {
		Data struct {
			Items []struct {
				Name string `json:"name"`
			} `json:"items"`
		} `json:"data"`
	}
	decode(t, usersAfterFailure, &afterFailure)
	for _, user := range afterFailure.Data.Items {
		if user.Name == "dave" {
			t.Fatal("failed group assignment must roll back user creation")
		}
	}

	renameInUse := api.do(http.MethodPut, "/api/v1/departments/"+url.PathEscape("部门D"), map[string]any{
		"passwd": api.revs.Passwd, "authz": api.revs.Authz, "name": "部门E",
	}, true, true)
	if renameInUse.Code != http.StatusOK {
		t.Fatalf("rename in-use department failed: %d %s", renameInUse.Code, renameInUse.Body.String())
	}
	api.readRevisions(renameInUse, false)

	usersAfterRename := api.do(http.MethodGet, "/api/v1/users", nil, true, false)
	var renamedUsers struct {
		Data struct {
			Items []struct {
				Name       string `json:"name"`
				Department string `json:"department"`
			} `json:"items"`
		} `json:"data"`
	}
	decode(t, usersAfterRename, &renamedUsers)
	carolDepartment := ""
	for _, user := range renamedUsers.Data.Items {
		if user.Name == "carol" {
			carolDepartment = user.Department
		}
	}
	if carolDepartment != "部门E" {
		t.Fatalf("department rename did not update user: %q", carolDepartment)
	}

	deleteInUse := api.do(http.MethodDelete, "/api/v1/departments/"+url.PathEscape("部门E"), map[string]any{
		"passwd": api.revs.Passwd, "authz": api.revs.Authz,
	}, true, true)
	if deleteInUse.Code != http.StatusConflict {
		t.Fatalf("expected in-use department delete conflict, got %d", deleteInUse.Code)
	}

	metadataUpdated := api.do(http.MethodPut, "/api/v1/users/carol/metadata", map[string]any{
		"passwd": api.revs.Passwd, "authz": api.revs.Authz,
		"remark": "更新后的示例成员", "department": "部门B",
	}, true, true)
	if metadataUpdated.Code != http.StatusOK {
		t.Fatalf("update metadata failed: %d %s", metadataUpdated.Code, metadataUpdated.Body.String())
	}
	api.readRevisions(metadataUpdated, false)

	deletedDepartment := api.do(http.MethodDelete, "/api/v1/departments/"+url.PathEscape("部门E"), map[string]any{
		"passwd": api.revs.Passwd, "authz": api.revs.Authz,
	}, true, true)
	if deletedDepartment.Code != http.StatusOK {
		t.Fatalf("delete department failed: %d %s", deletedDepartment.Code, deletedDepartment.Body.String())
	}
	api.readRevisions(deletedDepartment, false)

	batch := api.do(http.MethodPost, "/api/v1/users/batch", map[string]any{
		"passwd": api.revs.Passwd, "authz": api.revs.Authz,
		"names": []string{"alice", "carol"}, "action": "set_department", "department": "部门B",
	}, true, true)
	if batch.Code != http.StatusOK {
		t.Fatalf("batch update failed: %d %s", batch.Code, batch.Body.String())
	}
	api.readRevisions(batch, false)

	missingCSRF := api.do(http.MethodPost, "/api/v1/groups", map[string]any{
		"passwd": api.revs.Passwd, "authz": api.revs.Authz,
		"name": "release", "members": []string{"carol"},
	}, true, false)
	if missingCSRF.Code != http.StatusForbidden {
		t.Fatalf("expected csrf failure, got %d", missingCSRF.Code)
	}

	group := api.do(http.MethodPost, "/api/v1/groups", map[string]any{
		"passwd": api.revs.Passwd, "authz": api.revs.Authz,
		"name": "release", "remark": "发布验证组", "members": []string{"carol"},
	}, true, true)
	if group.Code != http.StatusCreated {
		t.Fatalf("create group failed: %d %s", group.Code, group.Body.String())
	}
	api.readRevisions(group, true)

	groupUpdated := api.do(http.MethodPut, "/api/v1/groups/release/members", map[string]any{
		"passwd": api.revs.Passwd, "authz": api.revs.Authz,
		"members": []string{"carol"}, "remark": "更新后的验证组",
	}, true, true)
	if groupUpdated.Code != http.StatusOK {
		t.Fatalf("update group failed: %d %s", groupUpdated.Code, groupUpdated.Body.String())
	}
	api.readRevisions(groupUpdated, false)

	acl := api.do(http.MethodPost, "/api/v1/repositories/demo/acl", map[string]any{
		"passwd": api.revs.Passwd, "authz": api.revs.Authz,
		"path": "/trunk", "kind": "group", "name": "release", "permission": "write",
	}, true, true)
	if acl.Code != http.StatusOK {
		t.Fatalf("set acl failed: %d %s", acl.Code, acl.Body.String())
	}
	api.readRevisions(acl, true)

	effective := api.do(http.MethodGet, "/api/v1/repositories/demo/acl?path=%2Ftrunk&user=carol", nil, true, false)
	var aclResp struct {
		Data struct {
			Effective svncfg.EffectiveAccess `json:"effective"`
		} `json:"data"`
	}
	decode(t, effective, &aclResp)
	if aclResp.Data.Effective.Permission != svncfg.PermissionWrite {
		t.Fatalf("unexpected effective permission: %+v", aclResp.Data.Effective)
	}

	groupPermissions := api.do(http.MethodPut, "/api/v1/groups/release/permissions", map[string]any{
		"passwd": api.revs.Passwd, "authz": api.revs.Authz,
		"permissions": []map[string]any{
			{"repo": "demo", "path": "/trunk", "permission": "read"},
		},
	}, true, true)
	if groupPermissions.Code != http.StatusOK {
		t.Fatalf("update group permissions failed: %d %s", groupPermissions.Code, groupPermissions.Body.String())
	}
	api.readRevisions(groupPermissions, false)

	effectiveAfter := api.do(http.MethodGet, "/api/v1/repositories/demo/acl?path=%2Ftrunk&user=carol", nil, true, false)
	var aclAfter struct {
		Data struct {
			Effective svncfg.EffectiveAccess `json:"effective"`
		} `json:"data"`
	}
	decode(t, effectiveAfter, &aclAfter)
	if aclAfter.Data.Effective.Permission != svncfg.PermissionRead {
		t.Fatalf("unexpected permission after group update: %+v", aclAfter.Data.Effective)
	}

	deleteBlocked := api.do(http.MethodDelete, "/api/v1/users/carol", map[string]any{
		"passwd": api.revs.Passwd, "authz": api.revs.Authz,
	}, true, true)
	if deleteBlocked.Code != http.StatusConflict {
		t.Fatalf("expected referenced user conflict, got %d %s", deleteBlocked.Code, deleteBlocked.Body.String())
	}

	reveal := api.do(http.MethodPost, "/api/v1/users/alice/reveal", map[string]any{}, true, true)
	if reveal.Code != http.StatusOK {
		t.Fatalf("reveal failed: %d %s", reveal.Code, reveal.Body.String())
	}
	var revealData struct {
		Data struct {
			Value string `json:"value"`
		} `json:"data"`
	}
	decode(t, reveal, &revealData)
	if revealData.Data.Value != "old-password" {
		t.Fatalf("unexpected revealed value %q", revealData.Data.Value)
	}

	removeACL := api.do(http.MethodDelete, "/api/v1/repositories/demo/acl", map[string]any{
		"passwd": api.revs.Passwd, "authz": api.revs.Authz,
		"path": "/trunk", "kind": "group", "name": "release",
	}, true, true)
	if removeACL.Code != http.StatusOK {
		t.Fatalf("remove acl failed: %d %s", removeACL.Code, removeACL.Body.String())
	}
	api.readRevisions(removeACL, false)

	removeGroup := api.do(http.MethodDelete, "/api/v1/groups/release", map[string]any{
		"passwd": api.revs.Passwd, "authz": api.revs.Authz,
	}, true, true)
	if removeGroup.Code != http.StatusOK {
		t.Fatalf("remove group failed: %d %s", removeGroup.Code, removeGroup.Body.String())
	}
	api.readRevisions(removeGroup, false)

	removeFromInitialGroup := api.do(http.MethodPut, "/api/v1/groups/qa/members", map[string]any{
		"passwd": api.revs.Passwd, "authz": api.revs.Authz,
		"members": []string{"bob"}, "remark": "",
	}, true, true)
	if removeFromInitialGroup.Code != http.StatusOK {
		t.Fatalf("remove initial group member failed: %d %s", removeFromInitialGroup.Code, removeFromInitialGroup.Body.String())
	}
	api.readRevisions(removeFromInitialGroup, false)

	removeUser := api.do(http.MethodDelete, "/api/v1/users/carol", map[string]any{
		"passwd": api.revs.Passwd, "authz": api.revs.Authz,
	}, true, true)
	if removeUser.Code != http.StatusOK {
		t.Fatalf("remove user failed: %d %s", removeUser.Code, removeUser.Body.String())
	}
	api.readRevisions(removeUser, false)

	audits := api.do(http.MethodGet, "/api/v1/audits?pageSize=100", nil, true, false)
	if audits.Code != http.StatusOK {
		t.Fatalf("audit query failed: %d %s", audits.Code, audits.Body.String())
	}
	if bytes.Contains(audits.Body.Bytes(), []byte("old-password")) {
		t.Fatal("audit response leaked password")
	}

	passwd, err := os.ReadFile(passwdPath)
	if err != nil {
		t.Fatal(err)
	}
	authz, err := os.ReadFile(authzPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(passwd, []byte("# users")) || !bytes.Contains(authz, []byte("# acl")) {
		t.Fatal("comments were not preserved")
	}
}

func (a *testAPI) do(method, url string, body any, authenticated, csrf bool) *httptest.ResponseRecorder {
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		data, err := json.Marshal(body)
		if err != nil {
			a.t.Fatal(err)
		}
		reader = bytes.NewReader(data)
	}
	request := httptest.NewRequest(method, url, reader)
	request.Header.Set("Content-Type", "application/json")
	if authenticated && a.cookie != nil {
		request.AddCookie(a.cookie)
	}
	if csrf {
		request.Header.Set("X-CSRF-Token", a.csrf)
	}
	recorder := httptest.NewRecorder()
	a.handler.ServeHTTP(recorder, request)
	return recorder
}

func (a *testAPI) readRevisions(recorder *httptest.ResponseRecorder, allowCreated bool) {
	var response struct {
		Data struct {
			Revisions svncfg.Revisions `json:"revisions"`
		} `json:"data"`
	}
	decode(a.t, recorder, &response)
	if response.Data.Revisions.Passwd == "" || response.Data.Revisions.Authz == "" {
		a.t.Fatal("response revisions missing")
	}
	a.revs = response.Data.Revisions
}

func decode(t *testing.T, recorder *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.Unmarshal(recorder.Body.Bytes(), target); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, recorder.Body.String())
	}
}
