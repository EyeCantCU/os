#!/bin/sh

# Append /etc/apk/repositories with the Danske Artifactory Mirrors
apk --repository="/etc/apk/danske-mirrors" update

# Execute any additional commands or scripts if needed
exec "$@"
