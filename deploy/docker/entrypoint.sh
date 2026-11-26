#!/bin/sh
set -eu

REPO_ROOT=${SVN_REPO_ROOT:-/srv/svn/repos}
AUTH_DIR=${SVN_AUTH_DIR:-/srv/svn/etc}
PASSWD_FILE=${AUTH_DIR}/passwd
AUTHZ_FILE=${AUTH_DIR}/authz
STATE_DIR=${SVNPANEL_STATE_DIR:-/var/lib/svnpanel}
RUNTIME_DIR=${SVNPANEL_RUNTIME_DIR:-/run/svnpanel}
CONFIG_PATH=${SVNPANEL_CONFIG_FILE:-${RUNTIME_DIR}/config.toml}
SVN_PORT=${SVN_PORT:-3690}
HTTP_PORT=${SVNPANEL_HTTP_PORT:-8080}
SVN_SERVE_BIN=${SVN_SERVE_BIN:-svnserve}
SVNPANEL_BIN=${SVNPANEL_BIN:-/usr/local/bin/svnpanel}
SVN_REPO_INIT_BIN=${SVN_REPO_INIT_BIN:-/usr/local/bin/svn-repo-init}

log() {
    printf '%s [svnpanel] %s\n' "$(date -u '+%Y-%m-%dT%H:%M:%SZ')" "$*"
}

die() {
    printf '%s [svnpanel] ERROR: %s\n' "$(date -u '+%Y-%m-%dT%H:%M:%SZ')" "$*" >&2
    exit 1
}

validate_port() {
    case "$2" in
        ''|*[!0-9]*) die "$1 必须是数字" ;;
    esac
    if [ "$2" -lt 1 ] || [ "$2" -gt 65535 ]; then
        die "$1 必须在 1-65535 范围内"
    fi
}

validate_single_line() {
    value=$1
    name=$2
    stripped=$(printf '%s' "$value" | tr -d '\r\n')
    if [ "$stripped" != "$value" ]; then
        die "$name 不能包含换行符"
    fi
}

toml_escape() {
    printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g'
}

toml_csv_array() {
    input=$1
    default_value=$2
    name=$3
    if [ -z "$input" ]; then
        input=$default_value
    fi

    old_ifs=$IFS
    IFS=,
    result=
    for item in $input; do
        item=$(printf '%s' "$item" | sed 's/^[[:space:]]*//; s/[[:space:]]*$//')
        if [ -z "$item" ]; then
            IFS=$old_ifs
            die "$name 不能包含空值"
        fi
        validate_single_line "$item" "$name"
        escaped=$(toml_escape "$item")
        if [ -n "$result" ]; then
            result="${result}, "
        fi
        result="${result}\"${escaped}\""
    done
    IFS=$old_ifs

    if [ -z "$result" ]; then
        die "$name 至少需要一个值"
    fi
    printf '[%s]' "$result"
}

validate_port SVN_PORT "$SVN_PORT"
validate_port SVNPANEL_HTTP_PORT "$HTTP_PORT"

case "${SVNPANEL_COOKIE_SECURE:-false}" in
    true|false) ;;
    *) die "SVNPANEL_COOKIE_SECURE 只能是 true 或 false" ;;
esac

mkdir -p "$REPO_ROOT" "$AUTH_DIR" "$STATE_DIR/audit" \
    "$STATE_DIR/backups" "$RUNTIME_DIR"

if [ ! -f "$PASSWD_FILE" ]; then
    printf '# svnserve password database\n# 面板会保留此文件中的注释和未修改内容\n' >"$PASSWD_FILE"
fi
if [ ! -f "$AUTHZ_FILE" ]; then
    printf '# svnserve authorization database\n\n[groups]\n' >"$AUTHZ_FILE"
fi
chmod 0600 "$PASSWD_FILE" "$AUTHZ_FILE"

if [ -n "${SVN_INIT_REPO:-}" ]; then
    "$SVN_REPO_INIT_BIN" "$SVN_INIT_REPO"
fi

if [ -n "${SVNPANEL_CONFIG_FILE:-}" ]; then
    [ -r "$CONFIG_PATH" ] || die "无法读取配置文件 $CONFIG_PATH"
    if grep -Eq \
        '^[[:space:]]*Password[[:space:]]*=[[:space:]]*"(change-me|replace-with)' \
        "$CONFIG_PATH"; then
        die "配置文件中的管理员密码仍然是示例值"
    fi
    log "使用挂载配置 $CONFIG_PATH"
else
    admin_username=${SVNPANEL_ADMIN_USERNAME:-admin}
    admin_password=${SVNPANEL_ADMIN_PASSWORD:-}
    [ -n "$admin_username" ] || die "SVNPANEL_ADMIN_USERNAME 不能为空"
    [ -n "$admin_password" ] || die "必须设置 SVNPANEL_ADMIN_PASSWORD"
    case "$admin_password" in
        change-me-before-use|replace-with-a-long-random-password|replace-with-a-strong-password)
            die "SVNPANEL_ADMIN_PASSWORD 仍然是示例值，请设置为强密码"
            ;;
    esac

    validate_single_line "$admin_username" "SVNPANEL_ADMIN_USERNAME"
    validate_single_line "$admin_password" "SVNPANEL_ADMIN_PASSWORD"
    escaped_username=$(toml_escape "$admin_username")
    escaped_password=$(toml_escape "$admin_password")
    escaped_repo_root=$(toml_escape "$REPO_ROOT")
    escaped_passwd_file=$(toml_escape "$PASSWD_FILE")
    escaped_authz_file=$(toml_escape "$AUTHZ_FILE")
    escaped_audit_dir=$(toml_escape "$STATE_DIR/audit")
    escaped_backup_dir=$(toml_escape "$STATE_DIR/backups")
    departments_input=${SVNPANEL_DEPARTMENTS:-部门A,部门B,部门C}
    trusted_proxies_input=${SVNPANEL_TRUSTED_PROXIES:-127.0.0.1,::1}
    validate_single_line "$departments_input" "SVNPANEL_DEPARTMENTS"
    validate_single_line "$trusted_proxies_input" "SVNPANEL_TRUSTED_PROXIES"
    departments=$(toml_csv_array "$departments_input" "部门A,部门B,部门C" "SVNPANEL_DEPARTMENTS")
    trusted_proxies=$(toml_csv_array "$trusted_proxies_input" "127.0.0.1,::1" "SVNPANEL_TRUSTED_PROXIES")
    [ -n "$departments" ] || die "SVNPANEL_DEPARTMENTS 配置无效"
    [ -n "$trusted_proxies" ] || die "SVNPANEL_TRUSTED_PROXIES 配置无效"

    mkdir -p "$(dirname "$CONFIG_PATH")"
    umask 077
    cat >"${CONFIG_PATH}.tmp" <<EOF
[Global]
Title = "SVN 权限面板"
RunMode = "release"

[HTTP]
Host = "0.0.0.0"
Port = ${HTTP_PORT}
ReadTimeout = 15
WriteTimeout = 30
IdleTimeout = 60
ShutdownTimeout = 15
MaxContentLength = 1048576
TrustedProxies = ${trusted_proxies}

[Admin]
Username = "${escaped_username}"
Password = "${escaped_password}"

[SVN]
RepoRoot = "${escaped_repo_root}"
PasswdFile = "${escaped_passwd_file}"
AuthzFile = "${escaped_authz_file}"
SvnlookPath = "/usr/bin/svnlook"
CommandTimeoutSeconds = 5
MaxCommandOutputMiB = 4
MaxPathsPerLevel = 2000

[Audit]
Dir = "${escaped_audit_dir}"

[Backup]
Dir = "${escaped_backup_dir}"
Keep = 50

[Session]
IdleMinutes = 30
AbsoluteHours = 8
CookieSecure = ${SVNPANEL_COOKIE_SECURE:-false}

[Metadata]
Departments = ${departments}
EOF
    mv "${CONFIG_PATH}.tmp" "$CONFIG_PATH"
    log "已从环境变量生成运行配置 $CONFIG_PATH"
fi

log "启动 svnserve，监听端口 ${SVN_PORT}"
"$SVN_SERVE_BIN" --daemon --foreground \
    --root "$REPO_ROOT" \
    --listen-host 0.0.0.0 \
    --listen-port "$SVN_PORT" &
svn_pid=$!

log "启动 SVN Panel，监听端口 ${HTTP_PORT}"
"$SVNPANEL_BIN" -conf "$CONFIG_PATH" &
panel_pid=$!

shutdown() {
    trap - INT TERM
    log "正在停止服务"
    kill -TERM "$panel_pid" "$svn_pid" 2>/dev/null || true
    wait "$panel_pid" 2>/dev/null || true
    wait "$svn_pid" 2>/dev/null || true
    exit 0
}

trap shutdown INT TERM

while :; do
    if ! kill -0 "$svn_pid" 2>/dev/null; then
        set +e
        wait "$svn_pid"
        status=$?
        set -e
        log "svnserve 已退出，状态码 ${status}"
        kill -TERM "$panel_pid" 2>/dev/null || true
        wait "$panel_pid" 2>/dev/null || true
        [ "$status" -ne 0 ] || status=1
        exit "$status"
    fi
    if ! kill -0 "$panel_pid" 2>/dev/null; then
        set +e
        wait "$panel_pid"
        status=$?
        set -e
        log "SVN Panel 已退出，状态码 ${status}"
        kill -TERM "$svn_pid" 2>/dev/null || true
        wait "$svn_pid" 2>/dev/null || true
        [ "$status" -ne 0 ] || status=1
        exit "$status"
    fi
    sleep 1
done
