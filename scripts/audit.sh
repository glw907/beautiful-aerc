#!/usr/bin/env bash
# audit.sh - Batch audit of HTML email blobs through mailrender html filter
# Reads blobs from aerc's JMAP cache, clusters by source signature,
# renders up to N samples per cluster, and writes results to audit-output/.

set -euo pipefail

# Defaults
BLOB_DIR="$HOME/.cache/aerc/blobs"
MAX_PER_SOURCE=2
OUTPUT_DIR="audit-output"
BINARY="./mailrender"

usage() {
  cat <<EOF
Usage: $(basename "$0") [OPTIONS]

Batch audit HTML email blobs through the mailrender html filter.

Options:
  -b DIR   Blob directory (default: $BLOB_DIR)
  -n NUM   Max samples per source signature (default: $MAX_PER_SOURCE)
  -o DIR   Output directory (default: $OUTPUT_DIR)
  -h       Show this help

Output:
  audit-output/index.txt   source signature + output filename, plus summary stats
  audit-output/<hash>.txt  rendered output for each selected blob
  audit-output/errors.txt  rendering failures
EOF
  exit 0
}

while getopts "b:n:o:h" opt; do
  case "$opt" in
    b) BLOB_DIR="$OPTARG" ;;
    n) MAX_PER_SOURCE="$OPTARG" ;;
    o) OUTPUT_DIR="$OPTARG" ;;
    h) usage ;;
    *) echo "Unknown option: -$OPTARG" >&2; exit 1 ;;
  esac
done

# Validate prerequisites
if [[ ! -d "$BLOB_DIR" ]]; then
  echo "error: blob directory not found: $BLOB_DIR" >&2
  exit 1
fi

if [[ ! -x "$BINARY" ]]; then
  echo "error: binary not executable: $BINARY" >&2
  echo "Run 'make build' first." >&2
  exit 1
fi

# Clean and recreate output directory
rm -rf "$OUTPUT_DIR"
mkdir -p "$OUTPUT_DIR"

INDEX="$OUTPUT_DIR/index.txt"
ERRORS="$OUTPUT_DIR/errors.txt"

# Write index header
printf "%-60s  %s\n" "SOURCE_SIGNATURE" "OUTPUT_FILE" > "$INDEX"
printf "%s\n" "$(printf '%.0s-' {1..80})" >> "$INDEX"

# Associative array tracking count per source signature
declare -A source_counts

total_html=0
total_rendered=0
total_errors=0
total_skipped=0

# Extract source signature from HTML content
get_source_signature() {
  local content="$1"

  # Priority 1: meta generator tag
  local generator
  generator=$(echo "$content" | grep -oi '<meta[^>]*name=["\047]generator["\047][^>]*>' | grep -oi 'content=["\047][^"'\'']*' | sed 's/content=["'\'']//' | head -1 | tr '[:upper:]' '[:lower:]' | tr -s ' ' '-' | cut -c1-50)
  if [[ -n "$generator" ]]; then
    echo "generator:${generator}"
    return
  fi

  # Priority 2: known platform markers in content (first 4000 bytes)
  local snippet
  snippet=$(echo "$content" | head -c 4000 | tr '[:upper:]' '[:lower:]')

  if echo "$snippet" | grep -q 'mailchimp'; then
    echo "platform:mailchimp"; return
  fi
  if echo "$snippet" | grep -q 'hubspot'; then
    echo "platform:hubspot"; return
  fi
  if echo "$snippet" | grep -q 'constantcontact'; then
    echo "platform:constantcontact"; return
  fi
  if echo "$snippet" | grep -q 'sendgrid'; then
    echo "platform:sendgrid"; return
  fi
  if echo "$snippet" | grep -q 'microsoft.*outlook\|outlook.*microsoft\|mso-'; then
    echo "platform:microsoft-outlook"; return
  fi
  if echo "$snippet" | grep -q 'gmail'; then
    echo "platform:gmail"; return
  fi
  if echo "$snippet" | grep -q 'yahoo'; then
    echo "platform:yahoo"; return
  fi
  if echo "$snippet" | grep -q 'thunderbird'; then
    echo "platform:thunderbird"; return
  fi

  # Priority 3: DOCTYPE variant
  local doctype
  doctype=$(echo "$content" | grep -oi '<!DOCTYPE[^>]*>' | head -1 | tr '[:upper:]' '[:lower:]' | tr -s ' ' '-' | cut -c1-60)
  if [[ -n "$doctype" ]]; then
    echo "doctype:${doctype}"
    return
  fi

  # Priority 4: first HTML tag
  local first_tag
  first_tag=$(echo "$content" | grep -oi '<[a-zA-Z][^>]*>' | head -1 | tr '[:upper:]' '[:lower:]' | tr -s ' ' '-' | cut -c1-50)
  if [[ -n "$first_tag" ]]; then
    echo "tag:${first_tag}"
    return
  fi

  echo "unknown"
}

is_html_blob() {
  local f="$1"
  local header
  header=$(head -c 200 "$f" 2>/dev/null | tr -d '\000')
  echo "$header" | grep -qiE '^\s*(<!DOCTYPE|<html|<head|<body|<table|<div)'
}

echo "Scanning blobs in $BLOB_DIR ..."

while IFS= read -r -d '' blob_file; do
  # Skip non-files
  [[ -f "$blob_file" ]] || continue

  # Check if HTML
  if ! is_html_blob "$blob_file"; then
    continue
  fi

  total_html=$((total_html + 1))

  # Get source signature
  content=$(cat "$blob_file" 2>/dev/null | tr -d '\000')
  sig=$(get_source_signature "$content")

  # Check count limit
  current_count="${source_counts[$sig]:-0}"
  if [[ "$current_count" -ge "$MAX_PER_SOURCE" ]]; then
    total_skipped=$((total_skipped + 1))
    continue
  fi

  source_counts[$sig]=$((current_count + 1))

  # Derive output filename from blob basename
  hash=$(basename "$blob_file")
  out_file="$OUTPUT_DIR/${hash}.txt"

  # Render through filter
  if AERC_COLUMNS=80 "$BINARY" html < "$blob_file" > "$out_file" 2>>"$ERRORS"; then
    total_rendered=$((total_rendered + 1))
    printf "%-60s  %s\n" "$sig" "${hash}.txt" >> "$INDEX"
  else
    total_errors=$((total_errors + 1))
    echo "$blob_file: render failed (sig: $sig)" >> "$ERRORS"
    rm -f "$out_file"
  fi

done < <(find "$BLOB_DIR" -type f -print0 | sort -z)

# Summary stats
echo "" >> "$INDEX"
echo "--- Summary ---" >> "$INDEX"
echo "HTML blobs found:    $total_html" >> "$INDEX"
echo "Rendered:            $total_rendered" >> "$INDEX"
echo "Skipped (>max/sig):  $total_skipped" >> "$INDEX"
echo "Errors:              $total_errors" >> "$INDEX"
echo "Source signatures:   ${#source_counts[@]}" >> "$INDEX"

echo ""
echo "Done."
echo "  HTML blobs found:    $total_html"
echo "  Rendered:            $total_rendered"
echo "  Skipped (>max/sig):  $total_skipped"
echo "  Errors:              $total_errors"
echo "  Source signatures:   ${#source_counts[@]}"
echo "  Output:              $OUTPUT_DIR/"
