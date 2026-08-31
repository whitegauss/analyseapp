#!/usr/bin/env bash
#
# Run the test suites for all three stacks with compact output.
#
#   scripts/test.sh [all|frontend|api|worker] [--full] [--cov]
#
# On success each suite prints a single line. On failure only the failing
# suite's output is shown, capped at MAX_FAIL_LINES; the full log is always
# kept under .test-logs/ so nothing is lost.
#
# Environment:
#   WORKER_PYTHON   python interpreter used for the worker suite
#   MAX_FAIL_LINES  lines of failure output to show per suite (default 80)

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LOG_DIR="$REPO_ROOT/.test-logs"
MAX_FAIL_LINES="${MAX_FAIL_LINES:-80}"
case "$MAX_FAIL_LINES" in
  '' | *[!0-9]*)
    echo "MAX_FAIL_LINES must be a non-negative integer; got '$MAX_FAIL_LINES'. Using 80." >&2
    MAX_FAIL_LINES=80
    ;;
esac

suite="all"
full=0
cov=0

for arg in "$@"; do
  case "$arg" in
    all | frontend | api | worker) suite="$arg" ;;
    --full) full=1 ;;
    --cov) cov=1 ;;
    -h | --help)
      sed -n '3,14p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    *)
      echo "unknown argument: $arg" >&2
      echo "usage: scripts/test.sh [all|frontend|api|worker] [--full] [--cov]" >&2
      exit 2
      ;;
  esac
done

mkdir -p "$LOG_DIR"

if [ -t 1 ]; then
  C_OK=$'\033[32m'; C_FAIL=$'\033[31m'; C_DIM=$'\033[2m'; C_OFF=$'\033[0m'
else
  C_OK=""; C_FAIL=""; C_DIM=""; C_OFF=""
fi

failed_suites=()

# BSD/macOS date has no %N and emits a literal "N", so probe once and fall
# back to whole seconds rather than producing garbage arithmetic.
if date +%s%N 2>/dev/null | grep -qE '^[0-9]+$'; then
  now_ms() { echo $(($(date +%s%N) / 1000000)); }
else
  now_ms() { echo $(($(date +%s) * 1000)); }
fi
elapsed_since() { local ms=$(($(now_ms) - $1)); printf '%d.%d' "$((ms / 1000))" "$(((ms % 1000) / 100))"; }

# report <name> <exit_code> <summary> <elapsed> <full_log> [display_log]
#
# display_log lets a suite strip boilerplate from what is shown on failure
# while the untouched full log stays on disk.
report() {
  local name="$1" code="$2" summary="$3" elapsed="$4" log="$5" shown_log="${6:-$5}"
  if [ "$code" -eq 0 ]; then
    printf '%s✓%s %-9s %-22s %ss\n' "$C_OK" "$C_OFF" "$name" "$summary" "$elapsed"
    return 0
  fi

  printf '%s✗%s %-9s %-22s %ss\n' "$C_FAIL" "$C_OFF" "$name" "$summary" "$elapsed"
  failed_suites+=("$name")

  local total shown
  total=$(wc -l < "$shown_log")
  if [ "$full" -eq 1 ] || [ "$total" -le "$MAX_FAIL_LINES" ]; then
    shown="$total"
    sed 's/^/  /' "$shown_log"
  else
    shown="$MAX_FAIL_LINES"
    printf '  %s… %s lines omitted, showing the last %s …%s\n' \
      "$C_DIM" "$((total - MAX_FAIL_LINES))" "$MAX_FAIL_LINES" "$C_OFF"
    tail -n "$MAX_FAIL_LINES" "$shown_log" | sed 's/^/  /'
  fi
  printf '  %sfull log: %s%s\n' "$C_DIM" "${log#"$REPO_ROOT"/}" "$C_OFF"
  [ "$shown" -lt "$total" ] && printf '  %sre-run with --full to see all %s lines%s\n' "$C_DIM" "$total" "$C_OFF"
  return 0
}

run_api() {
  local log="$LOG_DIR/api.log" start elapsed code summary
  local -a cmd=(go test ./...)
  # -coverpkg=./... credits coverage to every package, so packages without
  # their own tests are not silently dropped from the denominator.
  [ "$cov" -eq 1 ] && cmd+=(-covermode=atomic -coverpkg=./... -coverprofile="$LOG_DIR/api.coverprofile")

  start=$(now_ms)
  (cd "$REPO_ROOT/backend/api" && "${cmd[@]}") >"$log" 2>&1
  code=$?
  elapsed="$(elapsed_since "$start")"

  if [ "$code" -eq 0 ]; then
    summary="$(grep -c '^ok' "$log") packages"
    if [ "$cov" -eq 1 ] && [ -f "$LOG_DIR/api.coverprofile" ]; then
      summary="$summary  $( (cd "$REPO_ROOT/backend/api" && go tool cover -func="$LOG_DIR/api.coverprofile") | awk '/^total:/ {print $3}')"
    fi
  else
    summary="$(grep -c '^--- FAIL' "$log") failed"
    grep -vE '^(\?|ok) ' "$log" > "$LOG_DIR/api.failures.log"
  fi
  report api "$code" "$summary" "$elapsed" "$log" "$LOG_DIR/api.failures.log"
}

run_frontend() {
  local log="$LOG_DIR/frontend.log" start elapsed code summary
  local -a cmd=(npx --no-install vitest run --reporter=dot --silent)
  [ "$cov" -eq 1 ] && cmd+=(--coverage --coverage.reporter=text-summary)

  start=$(now_ms)
  (cd "$REPO_ROOT/frontend" && "${cmd[@]}") >"$log" 2>&1
  code=$?
  elapsed="$(elapsed_since "$start")"

  if [ "$code" -eq 0 ]; then
    summary="$(awk '/^ *Tests +[0-9]/ {print $2; exit}' "$log") tests"
    if [ "$cov" -eq 1 ]; then
      summary="$summary  $(awk '/^Statements/ {print $3; exit}' "$log")"
    fi
  else
    summary="$(awk '/^ *Tests +/ {for (i = 1; i <= NF; i++) if ($i == "failed") {print $(i-1); exit}}' "$log") failed"
    [ "$summary" = " failed" ] && summary="failed"
  fi
  report frontend "$code" "$summary" "$elapsed" "$log"
}

run_worker() {
  local log="$LOG_DIR/worker.log" start elapsed code summary py
  local worker_dir="$REPO_ROOT/backend/worker"

  if [ -n "${WORKER_PYTHON:-}" ]; then
    py="$WORKER_PYTHON"
  elif [ -x "$worker_dir/.venv/bin/python" ]; then
    py="$worker_dir/.venv/bin/python"
  else
    py="python3"
  fi

  local -a cmd=("$py" -m pytest -q --no-header --tb=short -p no:cacheprovider)
  [ "$cov" -eq 1 ] && cmd+=(--cov=app --cov-report=term-missing:skip-covered)

  start=$(now_ms)
  (cd "$worker_dir" && "${cmd[@]}") >"$log" 2>&1
  code=$?
  elapsed="$(elapsed_since "$start")"

  if [ "$code" -eq 0 ]; then
    summary="$(awk '/[0-9]+ passed/ {for (i = 1; i <= NF; i++) if ($i ~ /^passed,?$/) {print $(i-1); exit}}' "$log") tests"
    if [ "$cov" -eq 1 ]; then
      summary="$summary  $(awk '/^TOTAL/ {print $NF; exit}' "$log")"
    fi
  else
    summary="$(awk '/[0-9]+ failed/ {for (i = 1; i <= NF; i++) if ($i ~ /^failed,?$/) {print $(i-1); exit}}' "$log") failed"
    [ "$summary" = " failed" ] && summary="failed"
  fi
  report worker "$code" "$summary" "$elapsed" "$log"
}

case "$suite" in
  all) run_api; run_frontend; run_worker ;;
  api) run_api ;;
  frontend) run_frontend ;;
  worker) run_worker ;;
esac

if [ "${#failed_suites[@]}" -gt 0 ]; then
  printf '\n%s%s suite(s) failed: %s%s\n' "$C_FAIL" "${#failed_suites[@]}" "${failed_suites[*]}" "$C_OFF"
  exit 1
fi
