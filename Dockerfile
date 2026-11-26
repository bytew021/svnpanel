# syntax=docker/dockerfile:1.7

FROM --platform=$BUILDPLATFORM node:22-bookworm-slim AS frontend-build

WORKDIR /src/frontend
RUN npm install --global --no-fund --no-audit pnpm@11.22.0
COPY frontend/package.json frontend/pnpm-lock.yaml frontend/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile
COPY frontend/ ./
RUN pnpm build

FROM --platform=$BUILDPLATFORM golang:1.26-bookworm AS backend-build

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG BUILD_TIME=unknown

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend-build /src/frontend/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -buildvcs=false -tags=embed -trimpath \
    -ldflags "-s -w -X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME}" \
    -o /out/svnpanel .

FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        ca-certificates \
        curl \
        subversion \
        tini \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --gid 1000 svn \
    && useradd --uid 1000 --gid svn --home-dir /srv/svn \
        --shell /usr/sbin/nologin --no-create-home svn \
    && mkdir -p \
        /srv/svn/repos \
        /srv/svn/etc \
        /var/lib/svnpanel/audit \
        /var/lib/svnpanel/backups \
        /run/svnpanel \
    && chown -R svn:svn /srv/svn /var/lib/svnpanel /run/svnpanel

COPY --from=backend-build /out/svnpanel /usr/local/bin/svnpanel
COPY deploy/docker/entrypoint.sh /usr/local/bin/svnpanel-entrypoint
COPY deploy/docker/init-svn-repo.sh /usr/local/bin/svn-repo-init

RUN chmod 0755 \
        /usr/local/bin/svnpanel \
        /usr/local/bin/svnpanel-entrypoint \
        /usr/local/bin/svn-repo-init

ENV SVN_REPO_ROOT=/srv/svn/repos \
    SVN_AUTH_DIR=/srv/svn/etc \
    SVN_PORT=3690 \
    SVNPANEL_HTTP_PORT=8080 \
    SVNPANEL_ADMIN_USERNAME=admin \
    SVNPANEL_COOKIE_SECURE=false \
    SVNPANEL_TRUSTED_PROXIES=127.0.0.1,::1

WORKDIR /srv/svn
VOLUME ["/srv/svn", "/var/lib/svnpanel"]
EXPOSE 3690 8080
STOPSIGNAL SIGTERM

HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
    CMD curl --fail --silent --show-error \
        "http://127.0.0.1:${SVNPANEL_HTTP_PORT}/api/v1/healthz" >/dev/null || exit 1

USER svn
ENTRYPOINT ["/usr/bin/tini", "--", "/usr/local/bin/svnpanel-entrypoint"]
