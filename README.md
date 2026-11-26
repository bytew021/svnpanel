# SVN 权限面板

一个面向单组织、单台 Linux `svnserve` 主机的轻量管理面板。Go 后端直接维护现有 `passwd` 与 `authz` 文件，Vue 3 前端在生产构建时嵌入同一个二进制。

## 功能

- 单管理员登录，会话 Cookie + CSRF 防护
- SVN 用户新增、禁用、启用、重置密码、删除和表格内掩码查看
- 用户列表支持当前页多选，以及批量启用、停用、重置密码和设置部门
- SVN 普通分组、直接成员、当前路径权限汇总和权限编辑
- 用户备注、固定部门以及 SVN 分组备注，直接保存在认证文件上方的 `# svnpanel:` 注释中
- 仓库路径级 ACL：用户、分组、`*`、`$authenticated`、`$anonymous`
- `r`、`rw`、无权限三种结构化权限和有效权限预览
- `svnlook` 自动发现仓库、按层读取目录、允许手工补录路径
- revision 冲突保护、精准局部编辑、原子替换、最近 50 组备份
- 永久按日 JSONL 审计，记录登录、变更和密码查看

## 不包含

仓库创建/删除、目录内容浏览、HTTP/DAV、LDAP、Hook 编辑、定时任务和通知。

## 本地开发

需要 Go 1.26、Node.js、pnpm 和 `svnlook`。

```bash
make init-dev
make dev
```

另开终端：

```bash
cd frontend
pnpm install
pnpm dev
```

打开 `http://127.0.0.1:5173`。开发配置中的管理员账号为 `admin`，密码为 `change-me-before-use`。

Windows 本地开发可使用：

```bash
make dev-windows
```

对应配置为 `etc/config.windows.toml`，默认使用项目内的 `data/repos`、`etc/passwd` 和 `etc/authz`。如需复用 Linux 配置，可在 `[PathMap]` 中把 Linux 绝对路径映射到 Windows 路径：

```toml
[PathMap]
"/data/svn" = "D:/data/svn"
"/var/lib/svnpanel" = "D:/svnpanel-data"
```

Windows 路径建议使用 `/` 或 TOML 单引号字符串，避免反斜杠转义问题。

## 构建与验证

```bash
make verify
make build
make build-windows
```

`make build` 会先生成前端，再生成包含静态资源的 Linux amd64 二进制 `bin/svnpanel`。

## 生产配置

复制 `etc/config.production.toml.example` 到 `/etc/svnpanel/config.toml`，按实际路径修改，并确保：

```bash
chown svn:svn /etc/svnpanel/config.toml /srv/svn/etc/passwd /srv/svn/etc/authz
chmod 600 /etc/svnpanel/config.toml /srv/svn/etc/passwd /srv/svn/etc/authz
```

完整部署步骤见 [docs/部署说明.md](docs/部署说明.md)。

## Docker

镜像同时打包 `svnserve` 和 SVN Panel，可通过 Compose 启动：

```bash
cp .env.example .env
# 修改 .env 中的管理员密码
docker compose up -d --build
docker compose exec svnpanel svn-repo-init example
```

面板默认监听 `8080`，SVN 默认监听 `3690`。容器内仓库与认证文件位于 `/srv/svn`，审计与备份位于 `/var/lib/svnpanel`，两者都应挂载持久化卷。完整说明见 [docs/Docker部署.md](docs/Docker部署.md)。

GitHub Actions 会构建、冒烟测试镜像，并在 `main` 分支或 `v*` 标签推送时发布到 GHCR。发布 GitHub Release 时，还会按版本推送 `Docker Hub`，正式版本同步更新 `latest`。

每个 Release 还会生成并附加以 `svnpanel_<版本>_<系统>_<架构>` 命名的 Windows amd64、Linux amd64、Linux arm64 二进制及 `checksums.txt`。相关说明见 [docs/发版说明.md](docs/发版说明.md)。

## 重要约束

- `passwd` 和 `authz` 是唯一权限真源，系统不建立权限数据库。
- 面板不会修改 `svnserve.conf`，`svnserve` 也无需因用户或 ACL 变更而重启。
- 所有被管理仓库应位于同一个父目录，并共用一组 `passwd`、`authz`。
- 评论、空行、换行格式、BOM 和未知 authz 语法在未触及区域会保持原样。
- 配置中的管理员密码是明文，这是本项目明确接受的设计取舍。

## 备注与部门

部门下拉值由配置固定：

```toml
[Metadata]
Departments = ["部门A", "部门B", "部门C"]
```

面板写入的格式如下，其他人工注释不会被识别或改写：

```text
# svnpanel:remark: 示例用户
# svnpanel:department: 部门A
example_user = example-password
```

SVN 分组备注使用同一机制，但只支持备注：

```text
# svnpanel:remark: 示例分组
example_group = alice,bob,carol
```
