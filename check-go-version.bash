#!/bin/env bash

set -e

get_latest_versions() {
  curl -s https://go.dev/VERSION?m=text | sed -E -n 's/go([0-9]+\.[0-9]+|\.[0-9]+).*/\1/p'
}

current_version=$(cat go_versions.txt | head -n 1)
latest_version=$(get_latest_versions)

# Append new version to go_versions.txt at the top
if [[ ! $current_version =~ $latest_version ]]; then
  echo "New Go version available: $latest_version"
  echo "Updating go_versions.txt and generating Go Collector test files"
  sed -i "1i $latest_version" go_versions.txt
  make generate-go-collector-test-files
else
  echo "No new Go version detected. Current Go version is: $current_version"
fi
