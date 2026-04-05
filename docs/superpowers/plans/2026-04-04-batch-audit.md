# Batch Email Audit Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a reusable audit script that pipes cached emails through the beautiful-aerc html filter, producing reviewable output for batch quality assessment.

**Architecture:** A bash script reads aerc's JMAP blob cache (`~/.cache/aerc/907.life/blobs/`), identifies HTML blobs (already extracted by aerc - no MIME parsing needed), selects a diverse sample, pipes each through `beautiful-aerc html`, and writes rendered output to an `audit-output/` directory with an index. Phase 2 is manual review. Phase 3 is pipeline fixes and refactoring informed by the review.

**Tech Stack:** Bash (audit script), Go (pipeline fixes in Phase 3)

**Key discovery:** aerc's JMAP cache stores MIME parts as individual blobs. Of ~320 blobs, ~208 are already-extracted HTML bodies (start with `<!DOCTYPE`, `<html>`, etc.), ~21 are full RFC 2822 messages, ~84 are plain text bodies, and ~7 are binary attachments. The HTML blobs can be piped directly into `beautiful-aerc html` without MIME extraction. The RFC 2822 messages contain From/Subject headers we can use for sender context when they reference the same email.

---

### Task 1: Create the audit script

**Files:**
- Create: `scripts/audit.sh`
- Modify: `.gitignore` (add `audit-output/`)

- [ ] **Step 1: Add audit-output to .gitignore**

Append to `.gitignore`:

```
audit-output/
```

- [ ] **Step 2: Create the audit script skeleton**

Create `scripts/audit.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

BLOB_DIR="${BLOB_DIR:-$HOME/.cache/aerc/907.life/blobs}"
AUDIT_DIR="audit-output"
BINARY="./beautiful-aerc"
MAX_PER_SOURCE=2

usage() {
    echo "Usage: audit.sh [-b blob-dir] [-n max-per-source] [-o output-dir]"
    echo ""
    echo "Pipe cached aerc HTML blobs through beautiful-aerc html filter"
    echo "and write rendered output for manual review."
    echo ""
    echo "Options:"
    echo "  -b DIR   Blob cache directory (default: ~/.cache/aerc/907.life/blobs)"
    echo "  -n NUM   Max emails per source signature (default: 2)"
    echo "  -o DIR   Output directory (default: audit-output)"
    exit 1
}

while getopts "b:n:o:h" opt; do
    case $opt in
        b) BLOB_DIR="$OPTARG" ;;
        n) MAX_PER_SOURCE="$OPTARG" ;;
        o) AUDIT_DIR="$OPTARG" ;;
        h) usage ;;
        *) usage ;;
    esac
done

if [[ ! -d "$BLOB_DIR" ]]; then
    echo "Error: blob directory not found: $BLOB_DIR" >&2
    exit 1
fi

if [[ ! -x "$BINARY" ]]; then
    echo "Error: beautiful-aerc binary not found at $BINARY" >&2
    echo "Run 'make build' first." >&2
    exit 1
fi

rm -rf "$AUDIT_DIR"
mkdir -p "$AUDIT_DIR"

# Step 1: Find all HTML blobs and extract a source signature from each.
# A "source signature" is derived from the HTML content itself - the DOCTYPE,
# generator meta tag, or first structural element. This clusters emails by
# the tool/platform that generated them, which is what determines HTML patterns.
declare -A source_counts
processed=0
skipped=0
total_html=0

index_file="$AUDIT_DIR/index.txt"
printf "%-60s  %s\n" "SOURCE SIGNATURE" "OUTPUT FILE" > "$index_file"
printf "%-60s  %s\n" "$(printf '%0.s-' {1..60})" "$(printf '%0.s-' {1..30})" >> "$index_file"

for blob in $(find "$BLOB_DIR" -type f); do
    # Quick check: is this an HTML blob?
    first_bytes=$(head -c 500 "$blob" 2>/dev/null || true)
    if ! echo "$first_bytes" | grep -qi '<!doctype\|<html\|<head\|<body\|<table\|<div'; then
        continue
    fi
    total_html=$((total_html + 1))

    # Extract a source signature from the HTML content.
    # Priority: meta generator tag > DOCTYPE variant > first structural element
    sig=""

    # Check for generator meta tag (e.g., Constant Contact, Mailchimp)
    gen=$(grep -oi 'content="[^"]*"' "$blob" 2>/dev/null | head -1 || true)
    meta_gen=$(grep -oi '<meta[^>]*name="generator"[^>]*>' "$blob" 2>/dev/null | head -1 || true)
    if [[ -n "$meta_gen" ]]; then
        sig="generator:$(echo "$meta_gen" | grep -oi 'content="[^"]*"' | head -1 | sed 's/content="//;s/"//')"
    fi

    # Check for known platform markers in the HTML
    if [[ -z "$sig" ]]; then
        if grep -qi 'mc:edit\|mc:variant\|mailchimp' "$blob" 2>/dev/null; then
            sig="platform:mailchimp"
        elif grep -qi 'data-hs-cos\|hubspot' "$blob" 2>/dev/null; then
            sig="platform:hubspot"
        elif grep -qi 'constantcontact\|roving' "$blob" 2>/dev/null; then
            sig="platform:constantcontact"
        elif grep -qi 'sendgrid' "$blob" 2>/dev/null; then
            sig="platform:sendgrid"
        elif grep -qi 'mso-\|urn:schemas-microsoft' "$blob" 2>/dev/null; then
            sig="platform:microsoft-outlook"
        elif grep -qi 'gmail_default\|gmail' "$blob" 2>/dev/null; then
            sig="platform:gmail"
        elif grep -qi 'yahoo' "$blob" 2>/dev/null; then
            sig="platform:yahoo"
        elif grep -qi 'class="moz-' "$blob" 2>/dev/null; then
            sig="platform:thunderbird"
        fi
    fi

    # Fallback: use DOCTYPE variant
    if [[ -z "$sig" ]]; then
        doctype=$(head -5 "$blob" 2>/dev/null | grep -oi '<!doctype[^>]*>' | head -1 || true)
        if [[ -n "$doctype" ]]; then
            sig="doctype:$(echo "$doctype" | tr '[:upper:]' '[:lower:]' | sed 's/[[:space:]]\+/ /g' | cut -c1-80)"
        fi
    fi

    # Final fallback: first HTML tag
    if [[ -z "$sig" ]]; then
        sig="unknown:$(head -1 "$blob" | cut -c1-60)"
    fi

    # Check diversity limit
    count="${source_counts[$sig]:-0}"
    if [[ "$count" -ge "$MAX_PER_SOURCE" ]]; then
        skipped=$((skipped + 1))
        continue
    fi
    source_counts[$sig]=$((count + 1))

    # Generate output filename from blob hash
    hash=$(basename "$blob")
    outfile="$AUDIT_DIR/${hash}.txt"

    # Pipe through the filter
    export AERC_COLUMNS=80
    if "$BINARY" html < "$blob" > "$outfile" 2>/dev/null; then
        printf "%-60s  %s\n" "${sig:0:60}" "${hash}.txt" >> "$index_file"
        processed=$((processed + 1))
    else
        echo "FAILED: $blob ($sig)" >> "$AUDIT_DIR/errors.txt"
        rm -f "$outfile"
    fi
done

echo "" >> "$index_file"
echo "Total HTML blobs found: $total_html" >> "$index_file"
echo "Processed: $processed" >> "$index_file"
echo "Skipped (diversity limit): $skipped" >> "$index_file"
echo "Unique sources: ${#source_counts[@]}" >> "$index_file"

echo "Done. Processed $processed emails from ${#source_counts[@]} unique sources."
echo "Skipped $skipped (diversity limit of $MAX_PER_SOURCE per source)."
echo "Output in $AUDIT_DIR/"
```

- [ ] **Step 3: Make the script executable**

Run: `chmod +x scripts/audit.sh`

- [ ] **Step 4: Build the binary and run the script**

Run:
```bash
cd ~/Projects/beautiful-aerc
make build
./scripts/audit.sh
```

Expected: Script processes HTML blobs, writes rendered output to `audit-output/`, prints summary showing how many emails were processed and from how many unique sources.

- [ ] **Step 5: Verify the output**

Run:
```bash
cat audit-output/index.txt
ls audit-output/*.txt | wc -l
head -50 audit-output/$(ls audit-output/*.txt | head -1 | xargs basename)
```

Expected: Index file lists processed emails with source signatures. Individual output files contain ANSI-styled markdown-like text. Verify diversity - should see multiple different source signatures.

- [ ] **Step 6: Commit**

```bash
git add scripts/audit.sh .gitignore
git commit -m "Add batch audit script for filter quality review"
```

---

### Task 2: Run the audit and review output

This task is manual - the implementing agent reads all output files and catalogs issues.

- [ ] **Step 1: Read the index file**

Read `audit-output/index.txt` to understand the diversity of sources processed.

- [ ] **Step 2: Read each output file**

For each file in `audit-output/`, read it and evaluate:

1. Does this look like well-formatted markdown?
2. Structural problems: broken lists, orphaned markup characters, stray ANSI artifacts, bleeding emphasis, excessive whitespace, unstripped HTML entities, mangled links/footnotes
3. Aesthetic problems: technically correct but ugly output that a well-formatted markdown document wouldn't have

- [ ] **Step 3: Write the issue catalog**

Write findings to `audit-output/issues.md` with this structure:

```markdown
# Audit Issue Catalog

## Summary
- X emails reviewed
- Y issues found across Z categories

## Issues by Category

### Category: [name]
- **Affected emails:** [list of source signatures]
- **Pipeline stage:** [which stage causes this]
- **Description:** [what's wrong]
- **Example:** [snippet from affected output]
- **Root cause:** [why the current code doesn't handle this]

### Category: [name]
...
```

Group related issues. For each, identify which pipeline stage in `internal/filter/html.go` is responsible and what HTML input pattern triggers it.

- [ ] **Step 4: Identify refactoring opportunities**

After cataloging all issues, look for:
- Multiple issues with the same root cause
- Cleanup stages doing overlapping work
- Regex-based fixes that could be replaced with structural approaches
- Stages that have accumulated band-aid patches from the one-at-a-time workflow

Add a "Refactoring Opportunities" section to `issues.md`.

---

### Task 3: Fix issues and refactor the pipeline

This task depends entirely on what Task 2 finds. The implementing agent should:

- [ ] **Step 1: Read the issue catalog**

Read `audit-output/issues.md` and prioritize: structural issues first, then aesthetic, then refactoring.

- [ ] **Step 2: Fix issues at root cause**

For each issue category, fix the root cause in the pipeline. Prefer fixes that handle the general pattern over fixes that match specific senders. After each fix:

Run: `make check`
Expected: All tests pass.

- [ ] **Step 3: Refactor overlapping stages**

Based on the refactoring opportunities identified in Task 2, consolidate or simplify pipeline stages. Each refactor should be a separate commit:

Run: `make check`
Expected: All tests pass after each refactor.

- [ ] **Step 4: Re-run the audit**

Run:
```bash
make build
./scripts/audit.sh
```

Review the new output. Verify fixed issues are resolved and no regressions were introduced.

- [ ] **Step 5: Update golden files if pipeline behavior changed**

Run:
```bash
cd e2e && go test -update-golden && cd ..
make check
```

Expected: Golden files updated, all tests pass.

- [ ] **Step 6: Commit all changes**

Commit each logical change separately with descriptive messages. The audit script commit from Task 1 should already be done. Pipeline fixes and refactors get their own commits.
