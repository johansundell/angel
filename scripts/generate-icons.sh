#!/usr/bin/env bash
# Generates PNG icons for the PWA manifest, iOS touch icon, and browser favicon
# from source SVG assets.
#
# Usage:
#   ./scripts/generate-icons.sh
#   make icons
set -euo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || true)"
if [[ -z "$REPO_ROOT" ]]; then
	echo "Error: not inside a git repository." >&2
	exit 1
fi
cd "$REPO_ROOT"

if command -v rsvg-convert >/dev/null 2>&1; then
	RENDERER="rsvg-convert"
elif command -v magick >/dev/null 2>&1; then
	RENDERER="magick"
elif command -v convert >/dev/null 2>&1; then
	RENDERER="convert"
else
	echo "Error: neither rsvg-convert nor ImageMagick (magick/convert) was found on PATH." >&2
	echo "Install librsvg2-bin or imagemagick to generate icons." >&2
	exit 1
fi

echo "Using renderer: $RENDERER"

render_svg() {
	local src="$1"
	local dest="$2"
	local w="$3"
	local h="$4"

	if [[ ! -f "$src" ]]; then
		echo "Error: source vector not found: $src" >&2
		exit 1
	fi

	echo "Rendering $dest (${w}x${h}) from $src..."
	if [[ "$RENDERER" == "rsvg-convert" ]]; then
		rsvg-convert -w "$w" -h "$h" "$src" -o "$dest"
	elif [[ "$RENDERER" == "magick" ]]; then
		magick -background none -density 384 "$src" -resize "${w}x${h}" "$dest"
	elif [[ "$RENDERER" == "convert" ]]; then
		convert -background none -density 384 "$src" -resize "${w}x${h}" "$dest"
	fi
}

render_svg "assets/img/app-icon.svg" "assets/img/icon-192.png" 192 192
render_svg "assets/img/app-icon.svg" "assets/img/icon-512.png" 512 512
render_svg "assets/img/app-icon-maskable.svg" "assets/img/icon-maskable-512.png" 512 512
render_svg "assets/img/app-icon.svg" "assets/img/apple-touch-icon.png" 180 180
render_svg "assets/img/favicon.svg" "assets/img/favicon.png" 96 96

echo "Successfully generated all icons."
