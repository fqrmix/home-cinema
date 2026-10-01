# Relays the router's SMB share into a docker network that has no other
# path to the home LAN (e.g. only a VPN sidecar does): mounts the CIFS
# share here, using whatever network access this container has, then
# re-shares it over SMB again (via Samba) so the Docker host — which
# Docker always lets reach containers on its own bridge networks — can
# mount it normally and feed it into the rest of the stack exactly as
# README describes.
#
# Samba, not NFS: the kernel NFS server can't export a CIFS mount at all
# (`exportfs: does not support NFS export` - CIFS doesn't implement the
# export_operations the kernel's nfsd needs). Samba is a userspace file
# server that just does normal file I/O, so it re-shares a CIFS mount
# fine - confirmed directly against a real router share.
FROM alpine:3.20

RUN apk add --no-cache cifs-utils samba bash

COPY smb.conf /etc/samba/smb.conf
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
