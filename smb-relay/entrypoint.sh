#!/bin/bash
set -euo pipefail

: "${SMB_HOST:?SMB_HOST is required}"
: "${SMB_SHARE:?SMB_SHARE is required}"
: "${SMB_USER:?SMB_USER is required}"
: "${SMB_PASSWORD:?SMB_PASSWORD is required}"
: "${LAN_CIDR:?LAN_CIDR is required (e.g. 192.168.1.0/24)}"
: "${LAN_GATEWAY_IP:?LAN_GATEWAY_IP is required - the VPN/gateway container address on this docker network}"

# This container only exists because the host has no route to SMB_HOST
# itself - only the gateway container on this docker network (e.g.
# wgdashboard) does. Same route nginx-balancer's own nginx adds for the
# same reason - see its docker-entrypoint-reload.sh.
ip route add "${LAN_CIDR}" via "${LAN_GATEWAY_IP}"

mkdir -p /smb
mount -t cifs "//${SMB_HOST}/${SMB_SHARE}" /smb \
  -o "username=${SMB_USER},password=${SMB_PASSWORD},vers=3.0,iocharset=utf8,ro"

mkdir -p /run/samba
exec smbd --foreground --no-process-group -s /etc/samba/smb.conf
