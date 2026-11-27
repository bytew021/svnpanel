package dto

import "svnpanel/internal/svncfg"

type User struct {
	Name       string `json:"name"`
	Disabled   bool   `json:"disabled"`
	Encoding   string `json:"encoding"`
	Remark     string `json:"remark,omitempty"`
	Department string `json:"department,omitempty"`
}

type UsersResponse struct {
	Items       []User           `json:"items"`
	Revisions   svncfg.Revisions `json:"revisions"`
	Departments []string         `json:"departments"`
}

type RawFile struct {
	Content  string `json:"content"`
	Revision string `json:"revision"`
}

type RawFilesResponse struct {
	Passwd    RawFile          `json:"passwd"`
	Authz     RawFile          `json:"authz"`
	Revisions svncfg.Revisions `json:"revisions"`
}

type SnapshotsResponse struct {
	Items []svncfg.SnapshotInfo `json:"items"`
}

type SnapshotContent struct {
	ID        string  `json:"id"`
	CreatedAt string  `json:"createdAt"`
	Passwd    RawFile `json:"passwd"`
	Authz     RawFile `json:"authz"`
}

type SvnserveFileInfo struct {
	Path       string `json:"path"`
	Exists     bool   `json:"exists"`
	Mode       string `json:"mode,omitempty"`
	Size       int64  `json:"size,omitempty"`
	ModifiedAt string `json:"modifiedAt,omitempty"`
}

type SvnserveInfo struct {
	Available           bool                `json:"available"`
	Executable          string              `json:"executable"`
	ResolvedPath        string              `json:"resolvedPath,omitempty"`
	Version             string              `json:"version,omitempty"`
	VersionOutput       string              `json:"versionOutput,omitempty"`
	Port                int                 `json:"port"`
	PortOpen            bool                `json:"portOpen"`
	RepoRoot            string              `json:"repoRoot"`
	Passwd              SvnserveFileInfo    `json:"passwd"`
	Authz               SvnserveFileInfo    `json:"authz"`
	Repositories        []svncfg.Repository `json:"repositories"`
	TotalLatestRevision int64               `json:"totalLatestRevision"`
	CheckedAt           string              `json:"checkedAt"`
	Error               string              `json:"error,omitempty"`
}

type Group struct {
	Name        string                   `json:"name"`
	Members     []string                 `json:"members"`
	Unsupported bool                     `json:"unsupported"`
	Remark      string                   `json:"remark,omitempty"`
	Permissions []svncfg.GroupPermission `json:"permissions"`
}

type GroupsResponse struct {
	Items     []Group          `json:"items"`
	Revisions svncfg.Revisions `json:"revisions"`
}

type ACLIdentity = svncfg.ACLIdentity

type ACLResponse struct {
	Items     []svncfg.ACLEntry       `json:"items"`
	Effective *svncfg.EffectiveAccess `json:"effective,omitempty"`
	Revisions svncfg.Revisions        `json:"revisions"`
}

type LoginResult struct {
	Username  string `json:"username"`
	CSRF      string `json:"csrf"`
	ExpiresAt string `json:"expiresAt"`
}

type RevealResult struct {
	Value    string `json:"value"`
	Encoding string `json:"encoding"`
}

type OperationResult struct {
	Revisions svncfg.Revisions `json:"revisions"`
	Message   string           `json:"message,omitempty"`
}
