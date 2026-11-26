package svncfg

import (
	"bytes"
	"strings"
	"testing"
)

func TestPasswdEditPreservesBytes(t *testing.T) {
	t.Parallel()
	original := append([]byte{0xef, 0xbb, 0xbf}, []byte("# keep this comment\r\nalice = old-password\r\n#disabled#bob = hash-value\r\n")...)
	doc, err := ParsePasswd(original)
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.SetPassword("alice", "new-password"); err != nil {
		t.Fatal(err)
	}
	if err := doc.SetDisabled("bob", false); err != nil {
		t.Fatal(err)
	}
	if err := doc.AddUser("carol", "carol-pass", "", ""); err != nil {
		t.Fatal(err)
	}
	got := doc.Doc.Bytes()
	if !bytes.HasPrefix(got, []byte{0xef, 0xbb, 0xbf}) {
		t.Fatal("BOM was not preserved")
	}
	text := string(got)
	for _, expected := range []string{
		"# keep this comment\r\n",
		"alice = new-password\r\n",
		"bob = hash-value\r\n",
		"carol = carol-pass\r\n",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("missing %q in %q", expected, text)
		}
	}
	if strings.Contains(text, "\n") && !strings.Contains(text, "\r\n") {
		t.Fatal("line ending was replaced")
	}
}

func TestPasswdDisabledMarkerAndEncoding(t *testing.T) {
	t.Parallel()
	doc, err := ParsePasswd([]byte("#disabled#alice = $apr1$abc\nbob = clear\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !doc.Users["alice"].Disabled || doc.Users["alice"].Encoding != "apr1" {
		t.Fatal("disabled/hash metadata not detected")
	}
	if doc.Users["bob"].Disabled || doc.Users["bob"].Encoding != "plain" {
		t.Fatal("plain user metadata not detected")
	}
	if err := doc.DeleteUser("bob"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(doc.Doc.Bytes()), "bob = clear") {
		t.Fatal("user line was not deleted")
	}
}

func TestPasswdMetadataRoundTrip(t *testing.T) {
	t.Parallel()
	doc, err := ParsePasswd([]byte("# svnpanel:remark: 示例负责人\n# svnpanel:department: 部门A\nalice = old-password\n"))
	if err != nil {
		t.Fatal(err)
	}
	alice := doc.Users["alice"]
	if alice.Remark != "示例负责人" || alice.Department != "部门A" {
		t.Fatalf("metadata not parsed: %+v", alice)
	}
	if err := doc.SetMetadata("alice", "更新后的示例备注", "部门B"); err != nil {
		t.Fatal(err)
	}
	if err := doc.SetPassword("alice", "new-password"); err != nil {
		t.Fatal(err)
	}
	if err := doc.SetDisabled("alice", true); err != nil {
		t.Fatal(err)
	}
	text := string(doc.Doc.Bytes())
	for _, expected := range []string{
		"# svnpanel:remark: 更新后的示例备注",
		"# svnpanel:department: 部门B",
		"#disabled#alice = new-password",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("missing %q in %q", expected, text)
		}
	}
	if err := doc.SetMetadata("alice", "", ""); err != nil {
		t.Fatal(err)
	}
	text = string(doc.Doc.Bytes())
	if strings.Contains(text, "# svnpanel:") {
		t.Fatalf("metadata markers were not removed: %q", text)
	}
}

func TestPasswdDepartmentDefinitions(t *testing.T) {
	t.Parallel()
	original := append([]byte{0xef, 0xbb, 0xbf}, []byte(
		"# svnpanel:departments: 部门A\r\n# svnpanel:departments: 部门B\r\n# svnpanel:remark: 示例负责人\r\n# svnpanel:department: 部门A\r\nalice = old-password\r\n",
	)...)
	doc, err := ParsePasswd(original)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(doc.Departments, ",") != "部门A,部门B" {
		t.Fatalf("unexpected departments: %+v", doc.Departments)
	}
	if err := doc.AddDepartment("部门C"); err != nil {
		t.Fatal(err)
	}
	if err := doc.RenameDepartment("部门A", "部门D"); err != nil {
		t.Fatal(err)
	}
	if doc.Users["alice"].Department != "部门D" {
		t.Fatalf("user department was not renamed: %+v", doc.Users["alice"])
	}
	if err := doc.DeleteDepartment("部门D"); err == nil {
		t.Fatal("in-use department must not be deleted")
	}
	if err := doc.DeleteDepartment("部门B"); err != nil {
		t.Fatal(err)
	}
	if strings.Join(doc.Departments, ",") != "部门D,部门C" {
		t.Fatalf("unexpected departments after edits: %+v", doc.Departments)
	}
	text := string(doc.Doc.Bytes())
	for _, expected := range []string{
		"# svnpanel:departments: 部门D\r\n",
		"# svnpanel:department: 部门D\r\n",
		"# svnpanel:departments: 部门C\r\n",
		"alice = old-password\r\n",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("missing %q in %q", expected, text)
		}
	}
	if strings.Contains(text, "# svnpanel:departments: 部门B") {
		t.Fatalf("deleted department remains: %q", text)
	}
	if !bytes.HasPrefix(doc.Doc.Bytes(), []byte{0xef, 0xbb, 0xbf}) {
		t.Fatal("BOM was not preserved")
	}
}
