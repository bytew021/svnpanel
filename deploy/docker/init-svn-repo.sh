#!/bin/sh
set -eu

repo_name=${1:-}
repo_root=${SVN_REPO_ROOT:-/srv/svn/repos}
auth_dir=${SVN_AUTH_DIR:-/srv/svn/etc}
realm=${SVN_REALM:-svn}

case "$repo_name" in
    '')
        printf '用法: svn-repo-init <仓库名>\n' >&2
        exit 2
        ;;
    *[!A-Za-z0-9._-]*)
        printf '仓库名只能包含字母、数字、点、下划线和短横线\n' >&2
        exit 2
        ;;
esac

repo_path="${repo_root}/${repo_name}"
if [ -d "$repo_path" ]; then
    printf '仓库已存在: %s\n' "$repo_path"
    exit 0
fi

mkdir -p "$repo_root" "$auth_dir"
svnadmin create "$repo_path"

cat >"$repo_path/conf/svnserve.conf" <<EOF
[general]
anon-access = none
auth-access = write
password-db = ${auth_dir}/passwd
authz-db = ${auth_dir}/authz
realm = ${realm}
EOF
chmod 0600 "$repo_path/conf/svnserve.conf"

printf '仓库已创建: %s\n' "$repo_path"
printf 'SVN URL: svn://<host>:3690/%s\n' "$repo_name"
