#!/usr/bin/env bash
# vale-comments: commit-gate Vale scan of Go comment prose. Vale extracts the
# comment text from each .go file and lints it through the vendored glw907
# overlay, so the em dash and the banned lexicon block the gate. This catches
# what the voice-check.sh T33 grep cannot: an em dash in a comment that also
# carries a slash (a path, a date, an a/b list), where the negated-slash regex
# stops before the dash. Judgment words stay advisory (see .vale.ini), not gate
# blockers, because they recur in legitimate technical comments.
#
# Exit 1 on any error-level finding; 0 otherwise.
set -uo pipefail
cd "$(dirname "$0")/.."

if ! command -v vale >/dev/null 2>&1; then
  echo "vale-comments: vale not installed, skipping (see CLAUDE.md, Authoring)"
  exit 0
fi

# Workstation hygiene: when the canonical glw907 source is present, assert the
# vendored copy has not drifted from it. Skips on CI or a fresh clone, where the
# committed copy is itself the source of truth.
vendor="$HOME/.dotfiles/scripts/glw907-vendor.sh"
if [ -x "$vendor" ] && [ -d "$HOME/.dotfiles/vale/.config/vale/styles/glw907" ]; then
  "$vendor" "$PWD" || exit 1
fi

mapfile -t files < <(git ls-files '*.go')
[ "${#files[@]}" -gt 0 ] || { echo "vale-comments: no Go files"; exit 0; }

if vale --minAlertLevel=error --output=line "${files[@]}"; then
  echo "vale-comments: clean"
else
  echo "vale-comments: error-level findings in Go comment prose (above)." >&2
  echo "Fix the comment, or demote a false-positive rule in .vale.ini." >&2
  exit 1
fi
