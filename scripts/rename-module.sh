#!/bin/bash

# Script to rename Go module for publishing
# Usage: ./scripts/rename-module.sh <new-module-name>
# Example: ./scripts/rename-module.sh github.com/username/sg-emulator

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Get current module name from go.mod
CURRENT_MODULE=$(grep "^module " "$PROJECT_ROOT/go.mod" | awk '{print $2}')

if [ -z "$1" ]; then
    echo "Usage: $0 <new-module-name>"
    echo "Example: $0 github.com/username/sg-emulator"
    echo ""
    echo "Current module name: $CURRENT_MODULE"
    exit 1
fi

NEW_MODULE="$1"

if [ "$CURRENT_MODULE" = "$NEW_MODULE" ]; then
    echo "Module name is already '$NEW_MODULE'. No changes needed."
    exit 0
fi

echo "Renaming module from '$CURRENT_MODULE' to '$NEW_MODULE'..."

# Update go.mod
echo "Updating go.mod..."
sed -i "s|^module $CURRENT_MODULE|module $NEW_MODULE|" "$PROJECT_ROOT/go.mod"

# Find and replace in all .go files
echo "Updating import statements in .go files..."
find "$PROJECT_ROOT" -name "*.go" -type f | while read -r file; do
    if grep -q "\"$CURRENT_MODULE" "$file"; then
        echo "  Updating: $file"
        sed -i "s|\"$CURRENT_MODULE|\"$NEW_MODULE|g" "$file"
    fi
done

# Run go mod tidy to clean up
echo "Running go mod tidy..."
cd "$PROJECT_ROOT"
go mod tidy

echo ""
echo "Module renamed successfully!"
echo "  Old: $CURRENT_MODULE"
echo "  New: $NEW_MODULE"
echo ""
echo "Don't forget to:"
echo "  1. Update any external references to this module"
echo "  2. Commit the changes"
echo "  3. Create a git tag for versioning (e.g., git tag v0.1.0)"
