# Relays the router's SMB share into a docker network that has no other
# path to the home LAN (e.g. only a VPN sidecar does): mounts the CIFS
# share here, using whatever network access this container has, then
# re-exports it as NFSv4 so the Docker host — which Docker always lets
# reach containers on its own bridge networks — can mount it normally and
# feed it into the rest of the stack exactly as README describes.
FROM itsthenetwork/nfs-server-alpine:latest

RUN apk add --no-cache cifs-utils bash

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
