#!/bin/bash
# This script replaces all instances of the old import path with the new one in Go files

echo "Fixing import paths in Go files..."

# Find all .go files and apply the replacement
find . -type f -name "*.go" -exec sed -i 's|github.com/fibratus/portal|github.com/N0vaSky/portal|g' {} \;

echo "Import paths fixed."