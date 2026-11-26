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
