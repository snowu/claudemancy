---
description: List the sigils claudemancy knows and the skills they cast
allowed-tools: Bash
---
!`"${CLAUDE_PLUGIN_ROOT}/build.sh" --if-missing >/dev/null 2>&1; "${CLAUDE_PLUGIN_ROOT}/bin/claudemancy" --list 2>&1`

Show the grimoire above to the user exactly as printed, inside a single code block, with no commentary before or after it.
