package svncfg

import (
	"strings"
	"testing"
)

const sampleAuthz = `# global comment
[aliases]
ldap-harry = harry

[groups]
developers = alice, bob
qa = carol

[demo:/]
@developers = r
* =

[demo:/trunk]
bob = rw
$authenticated = r

[demo:/trunk/private]
bob =
`

func TestAuthzEditsPreserveUnrelatedContent(t *testing.T) {
	t.Parallel()
	doc, err := ParseAuthz([]byte(sampleAuthz))
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.SetACL("demo", "/trunk", ACLIdentity{Kind: IdentityUser, Name: "bob"}, PermissionWrite); err != nil {
		t.Fatal(err)
	}
	if err := doc.SetACL("demo", "/trunk/private", ACLIdentity{Kind: IdentityUser, Name: "bob"}, PermissionNone); err != nil {
		t.Fatal(err)
	}
	if err := doc.AddGroup("release", "", []string{"alice"}); err != nil {
		t.Fatal(err)
	}
	text := string(doc.Doc.Bytes())
	for _, expected := range []string{"# global comment", "ldap-harry = harry", "developers = alice, bob", "release = alice"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("missing %q in %s", expected, text)
		}
	}
}

func TestEffectivePermissionUsesNearestMatchingSection(t *testing.T) {
	t.Parallel()
	doc, err := ParseAuthz([]byte(sampleAuthz))
	if err != nil {
		t.Fatal(err)
	}
	root := doc.Effective("demo", "/", "alice")
	if root.Permission != PermissionRead || root.SourcePath != "/" {
		t.Fatalf("unexpected root access: %+v", root)
	}
	trunk := doc.Effective("demo", "/trunk/src", "bob")
	if trunk.Permission != PermissionWrite || trunk.SourcePath != "/trunk" {
		t.Fatalf("unexpected trunk access: %+v", trunk)
	}
	secret := doc.Effective("demo", "/trunk/private/file", "bob")
	if secret.Permission != PermissionNone || secret.SourcePath != "/trunk/private" {
		t.Fatalf("unexpected deny access: %+v", secret)
	}
	if secret.Supported {
		t.Fatal("aliases should make complete effective calculation unsupported")
	}
}

func TestEmptyAliasesSectionDoesNotDisableEffectivePermissions(t *testing.T) {
	t.Parallel()
	doc, err := ParseAuthz([]byte("[aliases]\n# joe = /CN=Joe\n\n[groups]\nusers = alice\n\n[demo:/]\n@users = r\n"))
	if err != nil {
		t.Fatal(err)
	}
	if doc.HasAliases {
		t.Fatal("comment-only aliases section must not be treated as configured aliases")
	}
	effective := doc.Effective("demo", "/project", "alice")
	if !effective.Supported || effective.Permission != PermissionRead {
		t.Fatalf("unexpected effective permissions: %+v", effective)
	}
}

func TestGroupReferencesBlockDeletion(t *testing.T) {
	t.Parallel()
	doc, err := ParseAuthz([]byte(sampleAuthz))
	if err != nil {
		t.Fatal(err)
	}
	refs := doc.GroupReferences("developers")
	if len(refs) != 1 || refs[0].Repo != "demo" || refs[0].Path != "/" {
		t.Fatalf("unexpected refs: %+v", refs)
	}
	userRefs := doc.UserReferences("bob")
	if len(userRefs) != 3 {
		t.Fatalf("unexpected user refs: %+v", userRefs)
	}
}

func TestAmbiguousDuplicateSectionRejected(t *testing.T) {
	t.Parallel()
	doc, err := ParseAuthz([]byte("[demo:/]\na = r\n[demo:/]\nb = r\n"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := doc.ACL("demo", "/"); err == nil {
		t.Fatal("duplicate section must be rejected")
	}
}

func TestGroupRemarkRoundTrip(t *testing.T) {
	t.Parallel()
	doc, err := ParseAuthz([]byte("[groups]\n# svnpanel:remark: 示例分组\nexample_group = alice, bob\n"))
	if err != nil {
		t.Fatal(err)
	}
	groups, err := doc.Groups()
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 || groups[0].Remark != "示例分组" {
		t.Fatalf("group remark not parsed: %+v", groups)
	}
	if err := doc.SetGroupRemark("example_group", "更新后的示例分组"); err != nil {
		t.Fatal(err)
	}
	if err := doc.SetGroupMembers("example_group", []string{"alice"}); err != nil {
		t.Fatal(err)
	}
	text := string(doc.Doc.Bytes())
	if !strings.Contains(text, "# svnpanel:remark: 更新后的示例分组\n") || !strings.Contains(text, "example_group = alice\n") {
		t.Fatalf("group metadata update failed: %q", text)
	}
	if err := doc.DeleteGroup("example_group"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(doc.Doc.Bytes()), "# svnpanel:remark:") {
		t.Fatal("group remark was not removed with group")
	}
}

func TestAddGroupWritesRemarkAboveEntry(t *testing.T) {
	t.Parallel()
	doc, err := ParseAuthz([]byte("[groups]\nqa = alice\n"))
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.AddGroup("release", "发布组", []string{"alice"}); err != nil {
		t.Fatal(err)
	}
	text := string(doc.Doc.Bytes())
	if !strings.Contains(text, "# svnpanel:remark: 发布组\nrelease = alice\n") {
		t.Fatalf("unexpected generated group metadata: %q", text)
	}
}

func TestGroupPermissionsReplace(t *testing.T) {
	t.Parallel()
	doc, err := ParseAuthz([]byte(sampleAuthz))
	if err != nil {
		t.Fatal(err)
	}
	current := doc.GroupPermissions("developers")
	if len(current) != 1 || current[0].Path != "/" || current[0].Permission != PermissionRead {
		t.Fatalf("unexpected current group permissions: %+v", current)
	}
	desired := []GroupPermission{
		{Repo: "demo", Path: "/trunk", Permission: PermissionWrite},
		{Repo: "demo", Path: "/test", Permission: PermissionNone},
	}
	if err := doc.ReplaceGroupPermissions("developers", desired); err != nil {
		t.Fatal(err)
	}
	updated := doc.GroupPermissions("developers")
	if len(updated) != 2 {
		t.Fatalf("unexpected updated permissions: %+v", updated)
	}
	if updated[0].Path == "/" {
		t.Fatal("old root permission was not removed")
	}
}

func TestDuplicateMetadataRejected(t *testing.T) {
	t.Parallel()
	_, err := ParsePasswd([]byte("# svnpanel:remark: first\n# svnpanel:remark: second\nalice = password\n"))
	if err == nil {
		t.Fatal("expected duplicate metadata error")
	}
}
