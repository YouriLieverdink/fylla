#!/usr/bin/env bash
# PreToolUse/Bash guard: hard-block destructive Laravel DB commands that wipe
# local data (migrate:fresh/refresh/reset, db:wipe). The local sqlite holds
# Fylla-owned data (capacity, billable flags, timer history) with no provider to
# re-pull from — see ADR-0003. Use plain `php artisan migrate` instead.
set -euo pipefail

cmd=$(jq -r '.tool_input.command // empty')

# Require an artisan invocation so innocent mentions (greps, commit messages)
# aren't blocked — every real destructive path runs through artisan.
if printf '%s' "$cmd" | grep -qE 'artisan' \
  && printf '%s' "$cmd" | grep -qE 'migrate:(fresh|refresh|reset)|db:wipe'; then
  cat <<'JSON'
{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"Blocked: destructive DB command (migrate:fresh / migrate:refresh / migrate:reset / db:wipe) drops all local data, which has no provider to re-pull from. Use plain `php artisan migrate` for pending migrations. If a rebuild is genuinely intended, the user must run it themselves in the terminal."}}
JSON
fi
