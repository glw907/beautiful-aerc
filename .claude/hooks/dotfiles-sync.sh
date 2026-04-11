#!/usr/bin/env bash
# Hook: remind to sync aerc config changes to dotfiles
# PostToolUse on Edit/Write targeting .config/aerc/ or .config/nvim-mail/

input=$(cat)
file=$(echo "$input" | jq -r '.tool_input.file_path // empty')

if [[ "$file" == *"/.config/aerc/"* || "$file" == *"/.config/nvim-mail/"* || "$file" == *"/.config/kitty/"* ]]; then
    echo "REMINDER: Apply this change to ~/.dotfiles/beautiful-aerc/ as well." >&2
fi
exit 0
