#!/bin/sh

# Overwrite /etc/apk/repositories with the Danske Artifactory Mirrors
cat <<EOF > /etc/apk/repositories
https://artifactory.danskenet.net/artifactory/remote-alpine-cgr-extras
https://artifactory.danskenet.net/artifactory/remote-alpine-wolfi-packages
EOF

# Execute any additional commands or scripts if needed
exec "$@"
