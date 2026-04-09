# Filter Testing via tmux

Render emails through the filter and verify output without requiring
human visual inspection.

## Rendering a corpus file

```bash
# HTML email
cat corpus/20260404-143022.html | mailrender html

# Plain text email
cat corpus/20260404-143022.txt | mailrender plain
```

## Preview in tmux (simulates aerc viewer)

```bash
tmux kill-session -t test 2>/dev/null
tmux new-session -d -s test -x 80 -y 40

# Render and display
cat corpus/file.html \
  | AERC_COLUMNS=80 AERC_CONFIG=~/.config/aerc mailrender html \
  | tmux load-buffer - \
  && tmux paste-buffer -t test

# Capture for inspection
tmux capture-pane -t test -p

# Clean up
tmux kill-session -t test
```

## Batch audit

The `scripts/audit.sh` script samples HTML from the JMAP blob cache,
runs each through the filter, and writes rendered output to a
directory for review.

```bash
bash scripts/audit.sh -o audit-output/
```

## Comparing output

Strip ANSI codes for text comparison:

```bash
cat corpus/file.html \
  | AERC_COLUMNS=80 AERC_CONFIG=~/.config/aerc mailrender html \
  | sed 's/\x1b\[[0-9;]*m//g' > /tmp/rendered.txt
```
