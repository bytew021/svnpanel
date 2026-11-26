# Docker 部署

镜像同时包含 `svnserve`、`svnlook`、`svnadmin` 和 SVN Panel。容器以非 root 用户 `svn`（UID/GID `1000`）运行。

## 端口与持久化目录

| 端口 | 用途 |
| --- | --- |
| `8080/tcp` | Web 管理面板 |
| `3690/tcp` | `svn://` 协议 |

| 容器目录 | 内容 |
| --- | --- |
| `/srv/svn/repos` | 所有 SVN 仓库 |
| `/srv/svn/etc` | 共享的 `passwd`、`authz` |
| `/var/lib/svnpanel/audit` | 永久审计日志 |
| `/var/lib/svnpanel/backups` | 配置变更备份 |

必须持久化 `/srv/svn` 和 `/var/lib/svnpanel`。如果使用宿主机绑定目录，请确保 UID/GID `1000` 可以读写。

## Compose 启动

```bash
cp .env.example .env
# 编辑 .env，至少修改 SVNPANEL_ADMIN_PASSWORD
docker compose up -d --build
docker compose logs -f svnpanel
```

创建第一个仓库：

```bash
docker compose exec svnpanel svn-repo-init example
```

仓库创建脚本会生成使用共享 `passwd` 和 `authz` 的 `svnserve.conf`。随后：

1. 打开 `http://<host>:8080` 并登录管理面板。
2. 在面板新增 SVN 用户。
3. 给该用户配置仓库路径权限。
4. 使用 `svn://<host>:3690/example` 进行检出。

如果需要在首次启动时自动创建仓库，可在 `.env` 中设置：

```dotenv
SVN_INIT_REPO=example
```

该操作是幂等的，仓库已存在时不会覆盖。

## 直接运行镜像

```bash
docker run -d \
  --name svnpanel \
  --restart unless-stopped \
  -p 8080:8080 \
  -p 3690:3690 \
  -e SVNPANEL_ADMIN_PASSWORD='replace-with-a-strong-password' \
  -v svn-data:/srv/svn \
  -v svnpanel-data:/var/lib/svnpanel \
  ghcr.io/<owner>/svnpanel:latest
```

健康检查地址：

```text
http://127.0.0.1:8080/api/v1/healthz
```

## 环境变量

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `SVNPANEL_ADMIN_USERNAME` | `admin` | 面板管理员用户名 |
| `SVNPANEL_ADMIN_PASSWORD` | 无 | 必填；使用环境变量生成运行配置时不能为空或示例值 |
| `SVNPANEL_COOKIE_SECURE` | `false` | HTTPS 反向代理后应设为 `true` |
| `SVNPANEL_DEPARTMENTS` | `部门A,部门B,部门C` | 部门下拉值，英文逗号分隔 |
| `SVNPANEL_TRUSTED_PROXIES` | `127.0.0.1,::1` | 可信反向代理地址，逗号分隔 |
| `SVNPANEL_HTTP_PORT` | `8080` | 面板容器内监听端口 |
| `SVN_PORT` | `3690` | `svnserve` 监听端口 |
| `SVN_INIT_REPO` | 空 | 可选，启动时自动创建并重复使用时保持幂等的仓库名 |
| `SVNPANEL_CONFIG_FILE` | 空 | 可选，使用挂载的 TOML 配置并跳过环境变量生成 |

## 使用自定义配置

挂载配置后，管理员密码、部门、路径等以 TOML 文件为准：

```bash
docker run -d \
  --name svnpanel \
  -p 8080:8080 \
  -p 3690:3690 \
  -e SVNPANEL_CONFIG_FILE=/etc/svnpanel/config.toml \
  -v "$PWD/config.toml:/etc/svnpanel/config.toml:ro" \
  -v svn-data:/srv/svn \
  -v svnpanel-data:/var/lib/svnpanel \
  svnpanel:latest
```

配置中的路径必须使用容器内路径，参考 [config.toml.example](../deploy/docker/config.toml.example)。

## GitHub Actions

`.github/workflows/container.yml` 会：

- 在 Pull Request 中构建并启动容器，检查面板健康接口和 `svnserve` 响应。
- 在 `main` 分支、`v*` 标签或手动触发时构建 `linux/amd64`、`linux/arm64` 镜像。
- 推送至 `ghcr.io/<owner>/svnpanel`。非默认分支使用分支名标签，版本标签同时生成语义化版本标签。

首次发布后，需要在 GitHub 仓库设置中确认 Packages 权限允许 Actions 使用 `GITHUB_TOKEN` 写入 GHCR。

GitHub Release 还会通过 `.github/workflows/release-binaries.yml` 生成 Windows amd64、Linux amd64 和 Linux arm64 二进制，详细说明见 [发版说明](发版说明.md)。

### Docker Hub 正式发版

`.github/workflows/release-dockerhub.yml` 在 GitHub Release 发布时执行：

- 使用 Release 标签重新构建并启动容器，检查面板和 `svnserve`。
- 构建 `linux/amd64`、`linux/arm64` 镜像并推送到 `<DockerHub用户>/svnpanel`。
- 正式版本同时更新 `major.minor` 和 `latest`；预发布版本只推送具体版本与 SHA 标签，不覆盖稳定标签。

在 GitHub 仓库的 `Settings -> Secrets and variables -> Actions` 中添加：

| Secret | 内容 |
| --- | --- |
| `DOCKERHUB_USERNAME` | Docker Hub 用户名 |
| `DOCKERHUB_TOKEN` | 具有 Read & Write 权限的 Docker Hub access token |

发布 `v1.2.3` 的 Release 后，镜像标签示例：

```text
<DockerHub用户>/svnpanel:1.2.3
<DockerHub用户>/svnpanel:1.2
<DockerHub用户>/svnpanel:latest
```

## 安全建议

- 面板端口应通过 HTTPS 反向代理开放；设置 `SVNPANEL_COOKIE_SECURE=true`。
- `SVN_INIT_REPO` 仅用于初始化，长期运行可不设置。
- 不要把 `.env` 或挂载的实际配置文件提交到 Git。
- 定期备份 `/srv/svn` 和 `/var/lib/svnpanel`。
