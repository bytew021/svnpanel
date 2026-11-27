export interface Revisions {
  passwd: string;
  authz: string;
}

export interface User {
  name: string;
  disabled: boolean;
  encoding: "plain" | "apr1" | "sha1" | "bcrypt";
  remark?: string;
  department?: string;
}

export interface UsersResponse {
  items: User[];
  revisions: Revisions;
  departments: string[];
}

export type RawFileName = "passwd" | "authz";

export interface RawFile {
  content: string;
  revision: string;
}

export interface RawFilesResponse {
  passwd: RawFile;
  authz: RawFile;
  revisions: Revisions;
}

export interface Group {
  name: string;
  members: string[];
  unsupported: boolean;
  remark?: string;
  permissions: GroupPermission[];
}

export interface GroupPermission {
  repo: string;
  path: string;
  permission: Permission;
}

export interface GroupsResponse {
  items: Group[];
  revisions: Revisions;
}

export type IdentityKind = "user" | "group" | "all" | "authenticated" | "anonymous" | "unsupported";
export type Permission = "none" | "read" | "write" | "unsupported";

export interface ACLIdentity {
  kind: IdentityKind;
  name?: string;
}

export interface ACLEntry {
  rawIdentity: string;
  identity: ACLIdentity;
  permission: Permission;
  rawValue: string;
  line: number;
  unsupported: boolean;
}

export interface EffectiveAccess {
  permission: Permission;
  sourcePath?: string;
  sources: string[];
  supported: boolean;
  warnings?: string[];
  repo: string;
  path: string;
  user: string;
}

export interface ACLResponse {
  items: ACLEntry[];
  effective?: EffectiveAccess;
  revisions: Revisions;
}

export interface Repository {
  name: string;
  revision: number;
}

export interface PathNode {
  name: string;
  path: string;
}

export interface OperationResult {
  revisions: Revisions;
  message?: string;
}
