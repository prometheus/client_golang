#!/bin/bash

git fetch --tags

CURR_DIR=$(pwd)

VERSION=$(cat VERSION)
TAG_NAME="v${VERSION}"

echo "Current HEAD: $(git rev-parse HEAD)"
PREVIOUS_VERSION=$(git show HEAD~1:VERSION)
echo "Previous Tag Version: $PREVIOUS_VERSION"

MANUAL_START_SHA=$1
MANUAL_END_SHA=$2

# Get the start SHA based on the tag, if not manually provided
if [ -z "$MANUAL_START_SHA" ]; then
  START_SHA=$(git rev-list -n 1 "v${PREVIOUS_VERSION}")
else
  START_SHA=$MANUAL_START_SHA
fi

# Get the end SHA (latest commit on main branch), if not manually provided
if [ -z "$MANUAL_END_SHA" ]; then
  END_SHA=$(git rev-parse HEAD)
else
  END_SHA=$MANUAL_END_SHA
fi

temp_dir="$(mktemp -d)" && \
  git clone --depth=1 -q https://github.com/kubernetes/release.git "${temp_dir}" && \
  cd "${temp_dir}/"  && \
  go build ./cmd/release-notes/ && \
  mv release-notes /usr/local/bin/

release-notes \
  --start-sha "${START_SHA}" \
  --end-sha "${END_SHA}" \
  --org prometheus \
  --repo client_golang \
  --branch main \
  --required-author "" \
  --debug \
  --dependencies=false \
  --output="CHANGELOG_NEW.md" \
  --go-template "go-template:${CURR_DIR}/changelog-template.tpl"

cat "CHANGELOG_NEW.md"

# Append new changelog entries to Unreleased section
if grep -q "## Unreleased" "${CURR_DIR}/CHANGELOG.md"; then
    sed "/## Unreleased/r CHANGELOG_NEW.md" "${CURR_DIR}/CHANGELOG.md" > "CHANGELOG_TMP.md" &&
    mv "CHANGELOG_TMP.md" "${CURR_DIR}/CHANGELOG.md"
else
    printf '## Unreleased\n\n' > "CHANGELOG_TMP.md"
    cat CHANGELOG_NEW.md >> "CHANGELOG_TMP.md"
    printf '\n\n' >> "CHANGELOG_TMP.md"
    cat "${CURR_DIR}/CHANGELOG.md" >> "CHANGELOG_TMP.md"
    mv "CHANGELOG_TMP.md" "${CURR_DIR}/CHANGELOG.md"
fi