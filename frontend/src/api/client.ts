import axios, { AxiosError } from "axios";
import type {
  ACLResponse,
  EffectiveAccess,
  GroupsResponse,
  Group,
  GroupPermission,
  IdentityKind,
  OperationResult,
  PathNode,
  Permission,
  RawFileName,
  RawFilesResponse,
  Revisions,
  Repository,
  SnapshotContent,
  SnapshotInfo,
  SvnserveInfo,
  UsersResponse,
} from "@/types";

export const api = axios.create({
  baseURL: "/api/v1",
  withCredentials: true,
  timeout: 15000,
});

let csrfToken = "";
let unauthorizedHandler: (() => void) | undefined;

export function setCSRFToken(value: string) {
  csrfToken = value;
}

export function onUnauthorized(handler: () => void) {
  unauthorizedHandler = handler;
}

api.interceptors.request.use((config) => {
  if (csrfToken && config.method && !["get", "head"].includes(config.method.toLowerCase())) {
    config.headers.set("X-CSRF-Token", csrfToken);
  }
  return config;
});

api.interceptors.response.use(
  (response) => response,
  (error: AxiosError) => {
    const requestURL = error.config?.url ?? "";
    if (error.response?.status === 401 && !requestURL.endsWith("/auth/login")) {
      unauthorizedHandler?.();
    }
    return Promise.reject(error);
  },
);

export function errorMessage(error: unknown): string {
  if (axios.isAxiosError(error)) {
    const payload = error.response?.data as { error?: { message?: string } } | undefined;
    if (payload?.error?.message) return payload.error.message;
    if (error.code === "ECONNABORTED") return "请求超时，请稍后重试";
    if (!error.response) return "无法连接服务器";
  }
  return error instanceof Error ? error.message : "操作失败";
}

export async function login(username: string, password: string) {
  const { data } = await api.post<{ data: { username: string; csrf: string; expiresAt: string } }>(
    "/auth/login",
    { username, password },
  );
  return data.data;
}

export async function me() {
  const { data } = await api.get<{ data: { username: string; csrf: string } }>("/auth/me");
  return data.data;
}

export async function sessionInfo() {
  const { data } = await api.get<{
    data: { authenticated: boolean; username?: string; csrf?: string };
  }>("/auth/session");
  return data.data;
}

export async function logout() {
  await api.post("/auth/logout");
}

export async function listUsers() {
  const { data } = await api.get<{ data: UsersResponse }>("/users");
  return data.data;
}

export async function getRawFiles() {
  const { data } = await api.get<{ data: RawFilesResponse }>("/files");
  return data.data;
}

export async function updateRawFile(name: RawFileName, revisions: Revisions, content: string) {
  const { data } = await api.put<{ data: OperationResult }>(`/files/${name}`, {
    ...revisions,
    content,
  });
  return data.data;
}

export async function listSnapshots() {
  const { data } = await api.get<{ data: { items: SnapshotInfo[] } }>("/snapshots");
  return data.data.items;
}

export async function getSnapshot(id: string) {
  const { data } = await api.get<{ data: SnapshotContent }>(
    `/snapshots/${encodeURIComponent(id)}`,
  );
  return data.data;
}

export async function restoreSnapshot(revisions: Revisions, id: string) {
  const { data } = await api.post<{ data: OperationResult }>(
    `/snapshots/${encodeURIComponent(id)}/restore`,
    revisions,
  );
  return data.data;
}

export async function createUser(revisions: Revisions, name: string, password: string, remark = "", department = "") {
  const { data } = await api.post<{ data: OperationResult }>("/users", {
    ...revisions,
    name,
    password,
    remark,
    department,
  });
  return data.data;
}

export async function setUserMetadata(revisions: Revisions, name: string, remark: string, department: string) {
  const { data } = await api.put<{ data: OperationResult }>(`/users/${encodeURIComponent(name)}/metadata`, {
    ...revisions,
    remark,
    department,
  });
  return data.data;
}

export async function addDepartment(revisions: Revisions, name: string) {
  const { data } = await api.post<{ data: OperationResult }>("/departments", {
    ...revisions,
    name,
  });
  return data.data;
}

export async function renameDepartment(revisions: Revisions, oldName: string, name: string) {
  const { data } = await api.put<{ data: OperationResult }>(
    `/departments/${encodeURIComponent(oldName)}`,
    { ...revisions, name },
  );
  return data.data;
}

export async function deleteDepartment(revisions: Revisions, name: string) {
  const { data } = await api.delete<{ data: OperationResult }>(
    `/departments/${encodeURIComponent(name)}`,
    { data: revisions },
  );
  return data.data;
}

export async function setUserDisabled(revisions: Revisions, name: string, disabled: boolean) {
  const { data } = await api.patch<{ data: OperationResult }>(`/users/${encodeURIComponent(name)}`, {
    ...revisions,
    disabled,
  });
  return data.data;
}

export async function resetPassword(revisions: Revisions, name: string, password: string) {
  const { data } = await api.put<{ data: OperationResult }>(`/users/${encodeURIComponent(name)}/password`, {
    ...revisions,
    password,
  });
  return data.data;
}

export async function revealPassword(name: string) {
  const { data } = await api.post<{ data: { value: string; encoding: string } }>(
    `/users/${encodeURIComponent(name)}/reveal`,
    {},
  );
  return data.data;
}

export async function deleteUser(revisions: Revisions, name: string) {
  const { data } = await api.delete<{ data: OperationResult }>(`/users/${encodeURIComponent(name)}`, {
    data: revisions,
  });
  return data.data;
}

export type BatchUserAction = "enable" | "disable" | "reset_password" | "set_department";

export async function batchUsers(
  revisions: Revisions,
  names: string[],
  action: BatchUserAction,
  options: { password?: string; department?: string } = {},
) {
  const { data } = await api.post<{ data: OperationResult }>("/users/batch", {
    ...revisions,
    names,
    action,
    password: options.password ?? "",
    department: options.department ?? "",
  });
  return data.data;
}

export async function listGroups() {
  const { data } = await api.get<{ data: GroupsResponse }>("/groups");
  return data.data;
}

export async function createGroup(revisions: Revisions, name: string, remark: string, members: string[]) {
  const { data } = await api.post<{ data: OperationResult }>("/groups", { ...revisions, name, remark, members });
  return data.data;
}

export async function setGroupMembers(revisions: Revisions, name: string, members: string[], remark: string) {
  const { data } = await api.put<{ data: OperationResult }>(`/groups/${encodeURIComponent(name)}/members`, {
    ...revisions,
    members,
    remark,
  });
  return data.data;
}

export async function setGroupPermissions(revisions: Revisions, name: string, permissions: GroupPermission[]) {
  const { data } = await api.put<{ data: OperationResult }>(`/groups/${encodeURIComponent(name)}/permissions`, {
    ...revisions,
    permissions,
  });
  return data.data;
}

export async function deleteGroup(revisions: Revisions, name: string) {
  const { data } = await api.delete<{ data: OperationResult }>(`/groups/${encodeURIComponent(name)}`, {
    data: revisions,
  });
  return data.data;
}

export async function listRepositories() {
  const { data } = await api.get<{ data: { items: Repository[] } }>("/repositories");
  return data.data.items;
}

export async function getSvnserveInfo() {
  const { data } = await api.get<{ data: SvnserveInfo }>("/svnserve");
  return data.data;
}

export async function listPaths(repo: string, parent: string) {
  const { data } = await api.get<{ data: { parent: string; items: PathNode[] } }>(
    `/repositories/${encodeURIComponent(repo)}/tree`,
    { params: { parent } },
  );
  return data.data;
}

export async function getACL(repo: string, path: string, user = "") {
  const { data } = await api.get<{ data: ACLResponse }>(`/repositories/${encodeURIComponent(repo)}/acl`, {
    params: { path, user: user || undefined },
  });
  return data.data;
}

export async function setACL(
  revisions: Revisions,
  repo: string,
  path: string,
  kind: IdentityKind,
  name: string,
  permission: Permission,
) {
  const { data } = await api.post<{ data: OperationResult }>(
    `/repositories/${encodeURIComponent(repo)}/acl`,
    { ...revisions, path, kind, name, permission },
  );
  return data.data;
}

export async function deleteACL(revisions: Revisions, repo: string, path: string, kind: IdentityKind, name: string) {
  const { data } = await api.delete<{ data: OperationResult }>(
    `/repositories/${encodeURIComponent(repo)}/acl`,
    { data: { ...revisions, path, kind, name } },
  );
  return data.data;
}

export async function effectiveACL(repo: string, path: string, user: string) {
  const { data } = await api.get<{ data: ACLResponse }>(`/repositories/${encodeURIComponent(repo)}/acl`, {
    params: { path, user },
  });
  return data.data.effective as EffectiveAccess;
}

export interface AuditEvent {
  id: string;
  timestamp: string;
  actor: string;
  action: string;
  outcome: string;
  target?: string;
  repo?: string;
  path?: string;
  clientIp?: string;
  message?: string;
  details?: Record<string, unknown>;
}

export async function listAudits(params: Record<string, string | number | undefined>) {
  const { data } = await api.get<{
    data: { items: AuditEvent[]; total: number; page: number; pageSize: number };
  }>("/audits", { params });
  return data.data;
}

export type { Group };
