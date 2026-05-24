# Notification System Spec

**Purpose:** Provide a uniform `notify-me` interface that AI coding agents (Claude Code, Copilot CLI, and others) can invoke to alert the developer when they are blocked, waiting on input, or have completed a long-running task. Must work on macOS and Windows 11 with equivalent behavior.

**Audience:** An AI doctor agent that will install dependencies, write files, and configure agent hooks. Steps must be deterministic and idempotent.

---

## 1. Design goals

1. **Single interface.** Same command name (`notify-me`), same positional arguments, same exit codes on both platforms.
2. **No silent failures.** If the local OS notification cannot fire, fall back to a push channel (ntfy) so the developer always gets pinged.
3. **Audible by default.** Visual-only banners get ignored; sound is the primary signal that something is waiting.
4. **Secrets via 1Password.** Push tokens and topics resolved from `op://Developer/...` references — never plaintext on disk.
5. **Idempotent install.** Doctor can re-run setup safely; nothing duplicates, nothing breaks.
6. **Zero coupling to a specific agent.** Claude Code, Copilot, future tools, or a shell loop can all call it the same way.

## 2. Public interface

```
notify-me <title> <message> [--level <info|warn|urgent>] [--url <link>] [--quiet]
```

| Arg / flag | Required | Default | Behavior |
|---|---|---|---|
| `<title>` | yes | — | Short label, shown bold. Max 64 chars. |
| `<message>` | yes | — | Body text. Max 256 chars; truncate with ellipsis if longer. |
| `--level` | no | `info` | `info` = default sound, `warn` = attention sound, `urgent` = persistent + push fallback always fires. |
| `--url` | no | — | When clicked, opens this URL. Optional. |
| `--quiet` | no | false | Suppress sound; visual only. |

**Exit codes:** `0` success on at least one channel, `1` all channels failed, `2` bad arguments. Stderr carries human-readable error; stdout stays empty so the command is safe to chain.

**Example calls:**
```bash
notify-me "Claude" "awaiting approval to write file"
notify-me "Copilot" "tests failed, see terminal" --level warn
notify-me "Deploy" "production rollback needed" --level urgent --url "https://github.com/.../actions"
```

## 3. Platform implementations

Both implementations expose the identical CLI above. Internal differences are hidden.

### 3.1 macOS

- **Primary channel:** `terminal-notifier` (Homebrew). Chosen over raw `osascript` because it supports `--open <url>`, grouping, and reliable sound selection without AppleScript quoting hazards.
- **Sounds:** `info` → `Glass`, `warn` → `Ping`, `urgent` → `Sosumi`.
- **Grouping:** `-group claude-agent` so repeated notifications collapse instead of stacking.
- **Install:** `brew install terminal-notifier`.
- **Permissions:** first invocation requires the user to allow notifications for terminal-notifier in System Settings → Notifications. Doctor must surface this as a manual step and verify with a test ping.

### 3.2 Windows 11

- **Primary channel:** BurntToast PowerShell module. Standard, maintained, supports sound + click-through URL.
- **Sounds:** `info` → `Default`, `warn` → `Alarm2`, `urgent` → `Alarm` (looping).
- **Install:** `Install-Module BurntToast -Scope CurrentUser -Force`.
- **Focus Assist caveat:** BurntToast respects Do Not Disturb. Doctor must add `Claude` and `Copilot` to the priority list in Settings → System → Focus, or document that urgent notifications may be suppressed.
- **Script form:** ship as both `notify-me.ps1` and a `notify-me.cmd` shim so it's callable from any shell (cmd, PowerShell, Git Bash, WSL).

### 3.3 Push fallback (both platforms)

- **Service:** [ntfy.sh](https://ntfy.sh), self-hostable, no account needed for the public instance.
- **Trigger conditions:**
  - `--level urgent` → always send.
  - `info` / `warn` → send only if local notification command exited non-zero, OR if `NOTIFY_FORCE_PUSH=1`.
- **Topic + token** resolved at runtime from env vars `NTFY_TOPIC` and `NTFY_TOKEN`, which are populated by `op run --env-file=.env -- ...`. Topic should be a long random string (treat as a secret).
- **Payload:** title in `Title` header, level mapped to ntfy `Priority` (info=3, warn=4, urgent=5), optional `Click` header for the URL, optional `Tags` (`hourglass` for info, `warning` for warn, `rotating_light` for urgent).

## 4. File layout

```
~/.local/share/notify-me/          (macOS)
%LOCALAPPDATA%\notify-me\          (Windows)
├── notify-me                       # bash script (macOS) — chmod +x, symlinked to ~/.local/bin
├── notify-me.ps1                   # PowerShell script (Windows)
├── notify-me.cmd                   # shim that invokes pwsh -File notify-me.ps1 (Windows)
├── config.env                      # NTFY_TOPIC / NTFY_TOKEN as op:// references
└── version                         # plain text, semver, used for upgrade checks
```

`config.env` contents (same on both platforms):
```
NTFY_TOPIC=op://Developer/ntfy/topic
NTFY_TOKEN=op://Developer/ntfy/token
```

## 5. Agent integration

### 5.1 Claude Code

Append to `~/.claude/settings.json` (merge, don't overwrite existing hooks):

```json
{
  "hooks": {
    "Notification": [
      { "matcher": "", "hooks": [
        { "type": "command", "command": "notify-me 'Claude' 'awaiting input' --level warn" }
      ]}
    ],
    "Stop": [
      { "matcher": "", "hooks": [
        { "type": "command", "command": "notify-me 'Claude' 'task finished' --level info" }
      ]}
    ]
  }
}
```

### 5.2 GitHub Copilot CLI / VS Code

VS Code `settings.json` (user scope):
```json
{
  "chat.notifyWindowOnConfirmation": true
}
```

For Copilot CLI specifically, wrap invocations so a non-zero exit or an interactive prompt timer triggers `notify-me`. Doctor should add a shell alias:

```bash
copilot() { command copilot "$@"; rc=$?; [ $rc -ne 0 ] && notify-me "Copilot" "exited with $rc" --level warn; return $rc; }
```

PowerShell equivalent goes in `$PROFILE`.

### 5.3 Generic / future agents

Any tool that can run a shell command on a blocking event can use `notify-me`. Document this as the contract: emit `notify-me "<your-agent>" "<reason>"` whenever the agent is about to wait on the human for more than 30 seconds.

## 6. Doctor install procedure

The AI doctor runs these steps in order. Each step is idempotent.

**Both platforms — preflight:**
1. Verify `op` CLI is installed and signed in (`op whoami`). If not, install via Homebrew (`brew install --cask 1password-cli`) or winget (`winget install AgileBits.1Password.CLI`) and prompt the user to sign in.
2. Verify `op item list --vault Developer --tags ntfy` returns a topic and token item. If missing, create one with random topic name and prompt the user to save it.

**macOS:**
3. `brew install terminal-notifier` (skip if already present).
4. Create `~/.local/share/notify-me/`, write `notify-me` and `config.env`.
5. Symlink `~/.local/bin/notify-me` → script. Verify `~/.local/bin` is on `PATH`.
6. Run a test: `notify-me "Doctor" "install complete"`. Prompt user to confirm they saw + heard it. If not, walk through System Settings → Notifications permissions.

**Windows 11:**
3. `Install-Module BurntToast -Scope CurrentUser -Force` (check first with `Get-Module -ListAvailable BurntToast`).
4. Create `%LOCALAPPDATA%\notify-me\`, write `notify-me.ps1`, `notify-me.cmd`, `config.env`.
5. Add `%LOCALAPPDATA%\notify-me` to user `PATH` if not already there.
6. Run a test: `notify-me "Doctor" "install complete"`. If suppressed by Focus Assist, prompt user to whitelist.

**Both platforms — agent wiring:**
7. Merge the hook block into `~/.claude/settings.json`. Preserve existing hooks; deduplicate by command string.
8. Add VS Code setting `chat.notifyWindowOnConfirmation: true`.
9. Add the `copilot()` wrapper to the user's shell rc / PowerShell profile, guarded by an `# notify-me v1` marker so re-runs don't duplicate.

**Verification:**
10. Fire one ping at each level (`info`, `warn`, `urgent`) and confirm the urgent one reaches the push channel too. Tear down with `op run --env-file=$NOTIFY_CONFIG -- bash -c '...'` style invocation to prove secret resolution works end-to-end.

## 7. Testing checklist

The doctor should run and report:

- `notify-me "Test" "info"` → local banner + sound, no push.
- `notify-me "Test" "warn" --level warn` → local banner + attention sound, no push.
- `notify-me "Test" "urgent" --level urgent` → local banner + persistent sound + push to phone.
- `notify-me "Test" "with link" --url https://example.com` → clicking the banner opens the URL.
- Disconnect network, run `--level urgent` → exits non-zero, stderr explains push failed but local fired.
- Disable local notifications, run any level → push fires as fallback, exits zero.

## 8. Out of scope (intentionally)

- Email / SMS gateways. ntfy covers mobile push; layering more channels invites noise.
- Cross-machine fan-out (notify all of my devices at once). ntfy already does this if multiple devices subscribe to the same topic.
- Rate limiting. Agents should self-throttle; the spec assumes a notification per blocking event is fine.
- Server / headless support. Both target platforms have an interactive session by definition.

## 9. Open questions for the developer

1. Self-hosted ntfy or public `ntfy.sh`? Self-hosting is one Docker container but adds an op item and a domain.
2. Should `urgent` also flash the screen / take focus? macOS supports it via `terminal-notifier -activate`; Windows is harder. Default: no, but easy to enable.
3. Quiet hours? Doctor could read a config and route to push-only between, e.g., 22:00–07:00. Useful or annoying?
