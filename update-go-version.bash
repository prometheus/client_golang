#!/bin/env bash

set -e

get_latest_versions() {
  curl -s https://go.dev/VERSION?m=text | sed -E -n 's/go([0-9]+\.[0-9]+|\.[0-9]+).*/\1/p'
}

# Extract the current stable version from JSON
current_version=$(grep -A 1 '"label": "stable"' supported_go_versions.json | grep '"version"' | sed 's/.*"version": "\([^"]*\)".*/\1/')
latest_version=$(get_latest_versions)

# Check for new version of Go, and generate go collector test files
# Update supported_go_versions.json: shift stable to oldstable, add new version as stable
if [[ ! $current_version =~ $latest_version ]]; then
  echo "New Go version available: $latest_version"
  echo "Updating supported_go_versions.json and generating Go Collector test files"

  # Get the current stable version (which will become oldstable)
  current_stable_version=$(grep -A 1 '"label": "stable"' supported_go_versions.json | grep '"version"' | sed 's/.*"version": "\([^"]*\)".*/\1/')

  # Create new JSON structure with new version as stable, current stable as oldstable
  cat > supported_go_versions.json <<EOF
{
  "versions": [
    {
      "label": "stable",
      "version": "$latest_version",
      "name": "Tests (stable)"
    },
    {
      "label": "oldstable",
      "version": "$current_stable_version",
      "name": "Tests (oldstable)"
    }
  ]
}
EOF
else
  echo "No new Go version detected. Current Go version is: $current_version"
fi
