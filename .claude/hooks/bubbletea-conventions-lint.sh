#!/usr/bin/env bash
# Hook: lint poplar UI + content files for bubbletea structural-
# conventions violations. PostToolUse on Edit/Write.
#
# Non-blocking: prints warnings to stderr and exits 0. The hook is a
# review prompt, not a gate. Conventions live in
# docs/poplar/bubbletea-conventions.md; the research that grounds them
# lives in docs/poplar/research/2026-04-26-*.md.

input=$(cat)
file=$(echo "$input" | jq -r '.tool_input.file_path // empty')

# Scope: internal/ui/**/*.go and internal/content/*.go (renderers).
# Skip tests.
case "$file" in
    *"/internal/ui/"*.go|*"/internal/content/"*.go) ;;
    *) exit 0 ;;
esac
case "$file" in
    *_test.go) exit 0 ;;
esac

if [[ ! -f "$file" ]]; then
    exit 0
fi

warnings=""

# Rule 1: width math via len() on a string.
# Heuristic: a literal comparison of `len(...)` to a `width`-named
# variable on the same line, or a width assigned from len(...). The
# noisy "len near width" check produces too many collection-len false
# positives in this codebase; this stricter pattern catches the
# actual mistake (treating bytes as cells).
if grep -nE '(len\([^)]+\)\s*[<>=!]+\s*\w*[Ww]idth|[Ww]idth\s*[:=]+\s*len\()' "$file" >/dev/null; then
    warnings+="  Possible width math via len() — use lipgloss.Width or displayCells (§3 Text rendering)\n"
fi

# Rule 2: ansi.Wordwrap without ansi.Hardwrap nearby (renderer files).
# Triggers in internal/content/ files, or in any file that defines a
# function taking a `width int` parameter — those are renderers.
if grep -nE '(ansi\.Wordwrap\(|wordwrap\.String\()' "$file" >/dev/null; then
    if ! grep -nE '(ansi\.Hardwrap\(|Hardwrap\()' "$file" >/dev/null; then
        case "$file" in
            *"/internal/content/"*) is_renderer=1 ;;
            *)
                if grep -nE 'func.*\(.*\bwidth\s+int\b' "$file" >/dev/null; then
                    is_renderer=1
                else
                    is_renderer=0
                fi
                ;;
        esac
        if [[ "$is_renderer" == "1" ]]; then
            warnings+="  ansi.Wordwrap without ansi.Hardwrap — long tokens overflow (§3 Text rendering)\n"
        fi
    fi
fi

# Rule 3: defensive parent-side clipping. A MaxWidth call near a
# child View() call is a sign the child isn't honoring its size
# contract. Match if `MaxWidth(` and `.View()` appear within 3 lines.
if grep -nE -A2 'MaxWidth\(' "$file" 2>/dev/null \
        | grep -E '\.View\(\)' >/dev/null; then
    warnings+="  MaxWidth applied near a child View() call — fix the child's clipPane instead (§8 Anti-patterns)\n"
fi

# Rule 4: deprecated APIs.
if grep -nE '\b(HighPerformanceRendering|tea\.Sequentially|spinner\.Tick\(\)|\.NewModel\()' "$file" >/dev/null; then
    warnings+="  Deprecated bubbletea API — see norms §7 (HighPerformanceRendering, tea.Sequentially, package-level spinner.Tick, NewModel)\n"
fi

# Rule 5: EnterAltScreen in Init (catches the screen.go:29-31 warning).
if awk '/func.*\bInit\(/,/^}/' "$file" | grep -E 'EnterAltScreen|EnableMouseCellMotion|EnableMouseAllMotion' >/dev/null; then
    warnings+="  EnterAltScreen / EnableMouse* in Init — use the WithAltScreen / WithMouse* ProgramOption instead (§6 Program setup)\n"
fi

if [[ -n "$warnings" ]]; then
    echo "BUBBLETEA CONVENTIONS: $(basename "$file") may have issues:" >&2
    echo -e "$warnings" >&2
    echo "  Conventions: docs/poplar/bubbletea-conventions.md" >&2
fi

exit 0
