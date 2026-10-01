#!/bin/bash
set -euo pipefail

: "${SMB_HOST:?SMB_HOST is required}"
: "${SMB_SHARE:?SMB_SHARE is required}"
: "${SMB_USER:?SMB_USER is required}"
: "${SMB_PASSWORD:?SMB_PASSWORD is required}"

mkdir -p /smb
mount -t cifs "//${SMB_HOST}/${SMB_SHARE}" /smb \
  -o "username=${SMB_USER},password=${SMB_PASSWORD},vers=3.0,iocharset=utf8,uid=${SMB_UID:-1000},gid=${SMB_GID:-1000},ro"

export SHARED_DIRECTORY=/smb
export READ_ONLY="1"

exec /usr/bin/nfsd.sh
