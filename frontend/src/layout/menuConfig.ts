export interface MenuItem {
  name: string;
  path: string;
  icon: string;
}

export const menuConfig: MenuItem[] = [
  { name: "SVN 用户", path: "/users", icon: "users" },
  { name: "SVN 分组", path: "/groups", icon: "group" },
  { name: "路径权限", path: "/permissions", icon: "lock" },
  { name: "原生文件", path: "/files", icon: "file" },
  { name: "审计日志", path: "/audits", icon: "log" },
];
