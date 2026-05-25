#!/usr/bin/env bash
# tests/notify-me_test.sh — shell test suite for the notify-me wrapper.
#
# Invocation: bash notify-me_test.sh
# Relies on: bash ≥ 4.0, standard POSIX tools.
#
# Each test function is named test_<name>. The harness at the bottom
# runs them all and reports pass/fail counts.

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
NOTIFY_ME="$SCRIPT_DIR/../notify-me"

# ── test harness ──────────────────────────────────────────────────────────────

PASS=0
FAIL=0
FAILURES=()

run_test() {
  local name="$1"
  if "$name"; then
    echo "  PASS  $name"
    ((PASS++))
  else
    echo "  FAIL  $name"
    ((FAIL++))
    FAILURES+=("$name")
  fi
}

assert_eq() {
  local label="$1" expected="$2" actual="$3"
  if [[ "$actual" != "$expected" ]]; then
    echo "    ASSERTION FAILED: $label" >&2
    echo "      expected: $(printf '%q' "$expected")" >&2
    echo "      actual:   $(printf '%q' "$actual")" >&2
    return 1
  fi
}

assert_contains() {
  local label="$1" needle="$2" haystack="$3"
  if [[ "$haystack" != *"$needle"* ]]; then
    echo "    ASSERTION FAILED: $label" >&2
    echo "      needle:   $(printf '%q' "$needle")" >&2
    echo "      haystack: $(printf '%q' "$haystack")" >&2
    return 1
  fi
}

assert_empty() {
  local label="$1" actual="$2"
  if [[ -n "$actual" ]]; then
    echo "    ASSERTION FAILED: $label — expected empty, got: $(printf '%q' "$actual")" >&2
    return 1
  fi
}

assert_exit_nonzero() {
  local label="$1" code="$2"
  if [[ "$code" -eq 0 ]]; then
    echo "    ASSERTION FAILED: $label — expected non-zero exit, got 0" >&2
    return 1
  fi
}

# ── helpers ───────────────────────────────────────────────────────────────────

# make_fake_bin creates an executable script at <dir>/<name> that records
# its arguments to <dir>/<name>.args and exits 0.
make_fake_bin() {
  local dir="$1" name="$2"
  local bin="$dir/$name"
  cat > "$bin" << 'EOF'
#!/usr/bin/env bash
printf '%s\n' "$@" >> "${0}.args"
EOF
  chmod +x "$bin"
}

# make_failing_bin creates an executable that always exits non-zero.
make_failing_bin() {
  local dir="$1" name="$2"
  local bin="$dir/$name"
  cat > "$bin" << 'EOF'
#!/usr/bin/env bash
echo "fake $0 failed" >&2
exit 1
EOF
  chmod +x "$bin"
}

# ── tests ─────────────────────────────────────────────────────────────────────

# Test: terminal-notifier receives correct title and message arguments.
test_terminal_notifier_args() {
  local tmpdir
  tmpdir=$(mktemp -d)
  trap "rm -rf $tmpdir" RETURN

  make_fake_bin "$tmpdir" "terminal-notifier"

  stdout=$(PATH="$tmpdir:$PATH" bash "$NOTIFY_ME" \
    --title "Test Title" --message "Test Message" 2>/dev/null)
  rc=$?

  assert_eq "exit code 0 with terminal-notifier present" 0 "$rc" || return 1
  assert_empty "stdout is silent on success" "$stdout" || return 1

  local args_file="$tmpdir/terminal-notifier.args"
  [[ -f "$args_file" ]] || { echo "    FAIL: terminal-notifier.args not created" >&2; return 1; }
  local args
  args=$(cat "$args_file")
  assert_contains "passes -title flag"   "-title"         "$args" || return 1
  assert_contains "passes title value"   "Test Title"     "$args" || return 1
  assert_contains "passes -message flag" "-message"       "$args" || return 1
  assert_contains "passes message value" "Test Message"   "$args" || return 1
  assert_contains "passes -sound default" "-sound"        "$args" || return 1
}

# Test: falls back to osascript when terminal-notifier is not on PATH.
test_osascript_fallback() {
  local tmpdir
  tmpdir=$(mktemp -d)
  trap "rm -rf $tmpdir" RETURN

  make_fake_bin "$tmpdir" "osascript"

  # Put tmpdir first so our fake osascript shadows any real one.
  # Keep the system PATH suffix so tools like bash, sed, command are available.
  # terminal-notifier is NOT in tmpdir so command -v will not find it.
  stdout=$(PATH="$tmpdir:$PATH" bash "$NOTIFY_ME" \
    --title "FB Title" --message "FB Message" 2>/dev/null)
  rc=$?

  assert_eq "exit code 0 with osascript fallback" 0 "$rc" || return 1
  assert_empty "stdout is silent on osascript fallback" "$stdout" || return 1

  local args_file="$tmpdir/osascript.args"
  [[ -f "$args_file" ]] || { echo "    FAIL: osascript.args not created" >&2; return 1; }
  local args
  args=$(cat "$args_file")
  assert_contains "osascript receives -e flag" "-e" "$args" || return 1
  assert_contains "osascript receives title" "FB Title" "$args" || return 1
  assert_contains "osascript receives message" "FB Message" "$args" || return 1
}

# Test: terminal-notifier is preferred over osascript when both are on PATH.
test_terminal_notifier_preferred_over_osascript() {
  local tmpdir
  tmpdir=$(mktemp -d)
  trap "rm -rf $tmpdir" RETURN

  make_fake_bin "$tmpdir" "terminal-notifier"
  make_fake_bin "$tmpdir" "osascript"

  PATH="$tmpdir:$PATH" bash "$NOTIFY_ME" \
    --title "Pref" --message "Pref Msg" 2>/dev/null

  local tn_args="$tmpdir/terminal-notifier.args"
  local os_args="$tmpdir/osascript.args"
  [[ -f "$tn_args" ]] || { echo "    FAIL: terminal-notifier not called" >&2; return 1; }
  [[ ! -f "$os_args" ]] || { echo "    FAIL: osascript was called when terminal-notifier present" >&2; return 1; }
}

# Test: exits non-zero with stderr message when no notifier available.
test_exits_nonzero_when_no_notifier() {
  local tmpdir
  tmpdir=$(mktemp -d)
  trap "rm -rf $tmpdir" RETURN

  # Empty PATH: no binaries available.
  stdout=$(PATH="$tmpdir" bash "$NOTIFY_ME" \
    --title "Fail" --message "Should fail" 2>/tmp/notify_me_stderr_$$)
  rc=$?
  stderr=$(cat /tmp/notify_me_stderr_$$ 2>/dev/null); rm -f /tmp/notify_me_stderr_$$

  assert_exit_nonzero "exits non-zero when no notifier" "$rc" || return 1
  assert_empty "stdout empty on failure" "$stdout" || return 1
  [[ -n "$stderr" ]] || { echo "    FAIL: expected error on stderr, got nothing" >&2; return 1; }
}

# Test: ntfy curl POST is made when AI_NTFY_TOPIC is set and --level urgent.
test_ntfy_curl_called_on_urgent() {
  local tmpdir
  tmpdir=$(mktemp -d)
  trap "rm -rf $tmpdir" RETURN

  # Provide a working terminal-notifier so the local notification succeeds.
  make_fake_bin "$tmpdir" "terminal-notifier"

  # Fake curl that records its arguments.
  make_fake_bin "$tmpdir" "curl"

  AI_NTFY_TOPIC="test-topic-abc" PATH="$tmpdir:$PATH" bash "$NOTIFY_ME" \
    --title "Urgent" --message "Fire!" --level urgent 2>/dev/null

  local curl_args="$tmpdir/curl.args"
  [[ -f "$curl_args" ]] || { echo "    FAIL: curl not called for urgent ntfy push" >&2; return 1; }
  local args
  args=$(cat "$curl_args")
  assert_contains "curl targets ntfy.sh/test-topic-abc" "ntfy.sh/test-topic-abc" "$args" || return 1
  assert_contains "curl sends Title header" "Title"   "$args" || return 1
  assert_contains "curl sends Priority header" "urgent" "$args" || return 1
}

# Test: ntfy curl NOT called when --level is not urgent (default info).
test_ntfy_not_called_on_info() {
  local tmpdir
  tmpdir=$(mktemp -d)
  trap "rm -rf $tmpdir" RETURN

  make_fake_bin "$tmpdir" "terminal-notifier"
  make_fake_bin "$tmpdir" "curl"

  AI_NTFY_TOPIC="test-topic-abc" PATH="$tmpdir:$PATH" bash "$NOTIFY_ME" \
    --title "Info" --message "Non-urgent" --level info 2>/dev/null

  local curl_args="$tmpdir/curl.args"
  [[ ! -f "$curl_args" ]] || { echo "    FAIL: curl called for non-urgent level" >&2; return 1; }
}

# Test: ntfy is silent (no error) when AI_NTFY_TOPIC is not set.
test_ntfy_silent_when_no_topic() {
  local tmpdir
  tmpdir=$(mktemp -d)
  trap "rm -rf $tmpdir" RETURN

  make_fake_bin "$tmpdir" "terminal-notifier"
  make_fake_bin "$tmpdir" "curl"

  # Explicitly unset the env var.
  stdout=$(unset AI_NTFY_TOPIC; PATH="$tmpdir:$PATH" bash "$NOTIFY_ME" \
    --title "No ntfy" --message "No topic" --level urgent 2>/dev/null)
  rc=$?

  assert_eq "exit 0 when ntfy topic not set" 0 "$rc" || return 1
  # curl should NOT be called since no topic.
  local curl_args="$tmpdir/curl.args"
  [[ ! -f "$curl_args" ]] || { echo "    FAIL: curl called when AI_NTFY_TOPIC unset" >&2; return 1; }
}

# Test: missing required --title arg produces non-zero exit.
test_missing_title_arg() {
  local tmpdir
  tmpdir=$(mktemp -d)
  trap "rm -rf $tmpdir" RETURN

  make_fake_bin "$tmpdir" "terminal-notifier"

  PATH="$tmpdir:$PATH" bash "$NOTIFY_ME" --message "No title" 2>/dev/null
  rc=$?
  assert_exit_nonzero "exits non-zero when --title missing" "$rc"
}

# Test: missing required --message arg produces non-zero exit.
test_missing_message_arg() {
  local tmpdir
  tmpdir=$(mktemp -d)
  trap "rm -rf $tmpdir" RETURN

  make_fake_bin "$tmpdir" "terminal-notifier"

  PATH="$tmpdir:$PATH" bash "$NOTIFY_ME" --title "No message" 2>/dev/null
  rc=$?
  assert_exit_nonzero "exits non-zero when --message missing" "$rc"
}

# ── main ──────────────────────────────────────────────────────────────────────

main() {
  # Guard: notify-me must exist before tests can run.
  if [[ ! -f "$NOTIFY_ME" ]]; then
    echo "SKIP: notify-me script not yet created at $NOTIFY_ME" >&2
    echo "      (Tests will be RED until Coder A creates the script)" >&2
    exit 1
  fi

  echo "Running notify-me shell tests..."
  run_test test_terminal_notifier_args
  run_test test_osascript_fallback
  run_test test_terminal_notifier_preferred_over_osascript
  run_test test_exits_nonzero_when_no_notifier
  run_test test_ntfy_curl_called_on_urgent
  run_test test_ntfy_not_called_on_info
  run_test test_ntfy_silent_when_no_topic
  run_test test_missing_title_arg
  run_test test_missing_message_arg

  echo ""
  echo "Results: $PASS passed, $FAIL failed"
  if [[ ${#FAILURES[@]} -gt 0 ]]; then
    echo "Failed tests:"
    for f in "${FAILURES[@]}"; do
      echo "  - $f"
    done
    exit 1
  fi
}

main
