---
name: hook-author
description: >
  Guided workflow for authoring and installing governance hooks. Covers the
  full lifecycle: describe behavior → write code → validate → install → test.
  Triggers on: /hook-author, "author a hook", "create a hook",
  "write a governance hook", "add a hook", "propose a hook".
---

# Hook Author — Guided Workflow

Walks the user through all five phases of adding a governance hook to the
`~/.ai/hooks/` enforcement plane. Each phase ends with a Did / Next / Blocked
/ Artifact summary.

---

## Supported Event Types

| Event | When it fires |
|---|---|
| `PreToolUse` | Before any tool call executes |
| `PostToolUse` | After a tool call returns |
| `Stop` | When the main agent finishes a turn |
| `SubagentStop` | When a sub-agent finishes a turn |
| `PreCompact` | Before context compaction runs |
| `UserPromptSubmit` | When the user submits a prompt |

---

## Step 1: Define the Hook Behavior

Ask the user (batch yes/no; serialize open-ended):

1. **Event type** — Which event fires the hook? (see table above)
2. **Purpose** — What should it do? (block / log / redact / warn / validate)
3. **Trigger conditions** — Tool name, command pattern, or input content that
   activates the check (e.g., `tool == "Bash"`, command matches `git push`)

When done:

```
Did:    Collected event type, purpose, and trigger conditions.
Next:   Draft the hook script.
Blocked: —
Artifact: Hook specification in memory.
```

---

## Step 2: Write the Hook Code

Draft a hook script based on the Step 1 specification.

### Contract for all hooks

- Hooks receive a JSON payload on **stdin** describing the event.
- Exit **0** → allow / continue.
- Exit **1** → block / abort (message on stderr is shown to the user).
- The hook MUST NOT modify stdout in a way that corrupts the tool pipeline.

### Python template

```python
#!/usr/bin/env python3
"""
Hook: <name>
Event: <event-type>
Purpose: <one-line description>
"""
import json, sys

payload = json.load(sys.stdin)

# Example: block Bash commands that touch .env files
tool_name = payload.get("tool_name", "")
tool_input = payload.get("tool_input", {})

if tool_name == "Bash":
    command = tool_input.get("command", "")
    if ".env" in command:
        print("Blocked: direct access to .env files is not allowed.", file=sys.stderr)
        sys.exit(1)

sys.exit(0)
```

### Bash template

```bash
#!/usr/bin/env bash
# Hook: <name> | Event: <event-type>
payload=$(cat)
tool=$(echo "$payload" | python3 -c "import sys,json; print(json.load(sys.stdin).get('tool_name',''))")
if [[ "$tool" == "Bash" ]]; then
  cmd=$(echo "$payload" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('tool_input',{}).get('command',''))")
  if echo "$cmd" | grep -q "\.env"; then
    echo "Blocked: .env access not allowed." >&2; exit 1
  fi
fi
exit 0
```

Show the draft to the user and ask for approval or revisions before proceeding.

```
Did:    Drafted hook script.
Next:   Validate with `ai hooks validate`.
Blocked: Waiting for user review of the draft.
Artifact: <hook-file-path> (draft, not yet installed)
```

---

## Step 3: Validate the Hook

Once the user approves the draft, save it to a file and validate:

```bash
ai hooks validate <hook-file-path>
```

- If **validation passes**: proceed to Step 4.
- If **validation fails**: show the specific error output, offer to fix the draft,
  and re-run `ai hooks validate` after each fix. Do not proceed to install until
  validation is clean.

```
Did:    Ran `ai hooks validate <hook-file-path>`.
Next:   Install the hook (validation passed) OR fix errors and re-validate.
Blocked: Validation errors (show output).
Artifact: Validated hook file at <hook-file-path>.
```

---

## Step 4: Install the Hook

Install the validated hook:

```bash
ai hooks install <hook-file-path> --event <event-type>
```

Verify the hook appears in the active list:

```bash
ai hooks list
```

Confirm the new hook appears in the list. If `--event` is not accepted as a
flag, run `ai hooks install --help` and adjust to the actual syntax.

- Install conflict: offer `--force` to overwrite, or review the existing hook
  with `ai hooks list` first.

```
Did:    Installed hook; confirmed with `ai hooks list`.
Next:   Test the hook.
Blocked: Install conflict (describe resolution options).
Artifact: Hook active in ~/.ai/hooks/ for event <event-type>.
```

---

## Step 5: Test the Hook

Guide the user through a live test:

1. Describe a trigger action that SHOULD fire the hook (based on Step 1 conditions).
2. Ask the user to perform that action in their current session.
3. Observe the hook's behavior:
   - Did it fire? (check stderr output or audit log)
   - Did it block when it should have blocked?
   - Did it allow when it should have allowed?

If behavior is unexpected:

| Symptom | Diagnosis |
|---|---|
| Hook never fires | Wrong event type — re-check `ai hooks list` and the registered event |
| Hook fires on wrong actions | Trigger condition too broad — tighten the condition in the script |
| Hook blocks everything | Exit code or condition logic error — re-run `ai hooks validate` |
| JSON parse error on stdin | stdin format mismatch — check `~/.ai/hooks/_lib.py` for the payload schema |

To re-validate after a fix:

```bash
ai hooks validate <hook-file-path>
ai hooks install <hook-file-path> --event <event-type>
```

```
Did:    Tested hook against a trigger scenario.
Next:   Hook is operational, or iterate on the script and re-install.
Blocked: Unexpected behavior (describe symptom and diagnosis).
Artifact: Confirmed operational hook at <event-type> / <hook-file-path>.
```

