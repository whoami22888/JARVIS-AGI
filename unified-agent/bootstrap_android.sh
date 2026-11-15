#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SOURCE="$ROOT/templates/android"
TARGET="${1:-$ROOT/android}"

if [[ ! -d "$SOURCE" ]]; then
  echo "Android templates are missing: $SOURCE" >&2
  exit 1
fi

# Never delete runtime data, secrets, local modifications, or build caches.
# Check every conflicting template file before copying any file to avoid partial updates.
while IFS= read -r -d '' source_file; do
  relative_path="${source_file#"$SOURCE/"}"
  target_file="$TARGET/$relative_path"
  if [[ -e "$target_file" ]] && ! cmp -s -- "$source_file" "$target_file"; then
    printf 'Refusing to overwrite modified Android file: %s\n' "$relative_path" >&2
    echo "Update the source template or reconcile the generated file before bootstrapping." >&2
    exit 1
  fi
done < <(find "$SOURCE" -type f -print0)

mkdir -p "$TARGET"
cp -a "$SOURCE/." "$TARGET/"
printf 'Synchronized android templates into %s (existing local files preserved)\n' "$TARGET"
