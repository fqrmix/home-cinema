#!/bin/sh
set -e

# Only set on the production overlay (see docker-compose.prod.yml), where
# this container sits on homelab-private-network and needs an explicit
# route to the router's LAN through the VPN/gateway container - same
# reason nginx-balancer's own nginx and smb-relay both add this route.
# Local/default deployments never set these, so this is a no-op there.
if [ -n "${LAN_CIDR:-}" ] && [ -n "${LAN_GATEWAY_IP:-}" ]; then
  ip route add "${LAN_CIDR}" via "${LAN_GATEWAY_IP}"
fi

exec /usr/local/bin/server
