#!/usr/bin/env bash
# Installs a published GitHub release of angel on this server and restarts the
# service. If /healthz does not answer afterwards, the previous binary is put
# back. The Deploy workflow (.github/workflows/deploy.yml) runs it over SSH;
# see docs/deploy-vps.md for the setup.
#
# Usage: deploy.sh vX.Y.Z
set -euo pipefail

# Where the service's binary lives, the systemd service name and the health
# check URL (PORT in the service's .env).
DIR=/opt/angel
SERVICE=angel
HEALTH_URL=http://127.0.0.1:8080/healthz
REPO=johansundell/angel

TAG="${1:-}"
if [[ ! "$TAG" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
	echo "usage: deploy.sh vX.Y.Z (got: '$TAG')" >&2
	exit 2
fi

case "$(uname -m)" in
	x86_64) ARCH=amd64 ;;
	aarch64 | arm64) ARCH=arm64 ;;
	armv*) ARCH=arm ;;
	*) echo "unsupported architecture: $(uname -m)" >&2; exit 2 ;;
esac

NAME="angel_${TAG}_linux_${ARCH}"
URL="https://github.com/${REPO}/releases/download/${TAG}"

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT
cd "$WORK"

# make release publishes the release before it uploads the files, so a deploy
# started by the release event may have to wait for them.
for f in "$NAME.tar.gz" "$NAME.sha512"; do
	curl -fsSL --retry 20 --retry-delay 15 --retry-all-errors -o "$f" "$URL/$f"
done
sha512sum -c "$NAME.sha512"
tar -xzf "$NAME.tar.gz" angel

# Swap the binary with a rename, which is safe while the old one is running.
install -m 0755 angel "$DIR/angel.new"
cp -p "$DIR/angel" "$DIR/angel.prev"
mv "$DIR/angel.new" "$DIR/angel"
systemctl restart "$SERVICE"

for _ in $(seq 1 15); do
	if curl -fsS -H 'Accept: application/json' "$HEALTH_URL"; then
		echo
		echo "deployed $TAG"
		exit 0
	fi
	sleep 2
done

echo "$HEALTH_URL did not answer after deploying $TAG; rolling back" >&2
mv "$DIR/angel.prev" "$DIR/angel"
systemctl restart "$SERVICE"
exit 1
