#!/usr/bin/env bash
# vale-comments: commit-gate Vale scan of Go comment prose. Vale extracts the
# comment text from each .go file and lints it through the vendored glw907
# overlay, so the em dash and the banned lexicon block the gate. This catches
# what the voice-check.sh T33 grep cannot: an em dash in a comment that also
# carries a slash (a path, a date, an a/b list), where the negated-slash regex
# stops before the dash. Judgment words stay advisory (see .vale.ini), not gate
# blockers, because they recur in legitimate technical comments.
#
# A missing Vale hard-fails: an advisory guard that skips quietly is the
# silent-decay class this gate exists to close, and "installed on the
# workstation" is not portability.
#
# Pass --check-vendor to additionally assert the vendored glw907 overlay
# matches the workstation's canonical copy. Off by default: a fresh clone
# or CI has no workstation dotfiles to compare against, and the committed
# overlay is the source of truth there.
#
# Exit 1 on any error-level finding, a missing Vale, or (with
# --check-vendor) a drifted overlay; 0 otherwise.
#
# The file list unions tracked and untracked-but-not-ignored .go files: a
# brand-new file is invisible to `git ls-files` alone until it is staged, so
# a gate run before `git add` silently skips it and reports a false clean
# (the mechanical cause behind two false green claims in the pass 2 sweep,
# task 5a and task 10).
set -uo pipefail
cd "$(dirname "$0")/.."

check_vendor=0
for arg in "$@"; do
  case "$arg" in
    --check-vendor) check_vendor=1 ;;
    *)
      echo "vale-comments: unknown argument: $arg" >&2
      exit 2
      ;;
  esac
done

if ! command -v vale >/dev/null 2>&1; then
  echo "vale-comments: vale not installed; install it, do not skip the gate" >&2
  exit 1
fi

if [ "$check_vendor" -eq 1 ]; then
  vendor="$HOME/.dotfiles/scripts/glw907-vendor.sh"
  if [ ! -x "$vendor" ]; then
    echo "vale-comments: --check-vendor given but $vendor is not present or not executable" >&2
    exit 1
  fi
  "$vendor" "$PWD" || exit 1
fi

mapfile -t files < <(
  { git ls-files '*.go'; git ls-files --others --exclude-standard '*.go'; } | sort -u
)
[ "${#files[@]}" -gt 0 ] || { echo "vale-comments: no Go files"; exit 0; }

if vale --minAlertLevel=error --output=line "${files[@]}"; then
  echo "vale-comments: clean"
else
  echo "vale-comments: error-level findings in Go comment prose (above)." >&2
  echo "Fix the comment, or demote a false-positive rule in .vale.ini." >&2
  exit 1
fi
