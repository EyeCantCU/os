# Check if we are in the Chainguard Build Ecosystem
if [ \( -n "${targets}" -a -n "$(env | grep targets.contextdir=)" \) -o "${TEST_PIPELINE}" = "true" ]; then
    echo "Running inside the Chainguard Build Ecosystem"
else
    if [ "$(id -u)" = 0 ]; then
        echo "https://artifactory.danskenet.net/artifactory/remote-alpine-cgr-danske" > /etc/apk/repositories
        echo "https://artifactory.danskenet.net/artifactory/remote-alpine-cgr-extras" >> /etc/apk/repositories
        echo "https://artifactory.danskenet.net/artifactory/remote-alpine-wolfi-packages" >> /etc/apk/repositories
    fi
fi
