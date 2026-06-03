#!/usr/bin/env bash
set -euo pipefail

if [ $# -ne 1 ]; then
    echo "Usage: $0 <major|minor|patch>"
    exit 1
fi

TYPE="$1"
FILE="internal/core/constants.go"

CURRENT=$(grep -oP '^const Version = "\K[^"]+' "$FILE")

IFS='.' read -r MAJOR MINOR PATCH <<< "$CURRENT"

case "$TYPE" in
    major) MAJOR=$((MAJOR + 1)); MINOR=0; PATCH=0 ;;
    minor) MINOR=$((MINOR + 1)); PATCH=0 ;;
    patch) PATCH=$((PATCH + 1)) ;;
    *) echo "Usage: $0 <major|minor|patch>"; exit 1 ;;
esac

NEWVER="$MAJOR.$MINOR.$PATCH"
sed -i "s/^const Version = \"$CURRENT\"/const Version = \"$NEWVER\"/" "$FILE"

cd "$(git rev-parse --show-toplevel)"

git add "$FILE"
git commit -m "chore(release): v$NEWVER"
git tag "v$NEWVER"

echo "Bumped v$CURRENT → v$NEWVER"
echo "Tagged v$NEWVER"
echo ""
echo "Push with:"
echo "  git push --follow-tags"
