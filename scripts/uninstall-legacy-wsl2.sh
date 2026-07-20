#!/bin/sh

set -eu

DRY_RUN=0
case "${1-}" in
  "")
    ;;
  --dry-run)
    DRY_RUN=1
    ;;
  *)
    echo "Usage: $0 [--dry-run]" >&2
    exit 2
    ;;
esac

NAME="wsl2-ssh-agent-proxy"
HOME_REAL=$(CDPATH= cd "$HOME" && pwd -P)
PROXY_DIR="$HOME_REAL/$NAME"
PROXY_EXE="$PROXY_DIR/$NAME"
SOCKET_DIR="$HOME_REAL/.ssh/$NAME"
SOCKET_PATH="$SOCKET_DIR/$NAME.sock"
TIMESTAMP=$(date -u "+%Y%m%dT%H%M%SZ")

say() {
  printf '%s\n' "$*"
}

find_proxy_pids() {
  for process_dir in /proc/[0-9]*; do
    [ -d "$process_dir" ] || continue
    executable=$(readlink "$process_dir/exe" 2>/dev/null || true)
    case "$executable" in
      "$PROXY_EXE"|"$PROXY_EXE (deleted)")
        basename "$process_dir"
        ;;
    esac
  done
  return 0
}

stop_proxy() {
  pids=$(find_proxy_pids)
  if [ -z "$pids" ]; then
    say "Legacy $NAME process: not running"
    return
  fi

  say "Legacy $NAME process IDs: $(printf '%s' "$pids" | tr '\n' ' ')"
  if [ "$DRY_RUN" -eq 1 ]; then
    say "[dry-run] Would send TERM, wait up to 5 seconds, then send KILL if required."
    return
  fi

  for pid in $pids; do
    kill -TERM "$pid" 2>/dev/null || true
  done

  remaining="$pids"
  attempts=0
  while [ "$attempts" -lt 5 ]; do
    next=""
    for pid in $remaining; do
      if kill -0 "$pid" 2>/dev/null; then
        next="$next $pid"
      fi
    done
    remaining=${next# }
    [ -z "$remaining" ] && break
    attempts=$((attempts + 1))
    sleep 1
  done

  for pid in $remaining; do
    say "Process $pid did not stop after TERM; sending KILL."
    kill -KILL "$pid" 2>/dev/null || true
  done

  [ -z "$remaining" ] || sleep 1
  still_running=$(find_proxy_pids)
  if [ -n "$still_running" ]; then
    say "Could not stop legacy process IDs: $(printf '%s' "$still_running" | tr '\n' ' ')" >&2
    exit 1
  fi
  say "Legacy $NAME process stopped."
}

remove_profile_reference() {
  profile=$1
  [ -f "$profile" ] || return 0
  grep -q "$NAME" "$profile" 2>/dev/null || return 0

  backup="${profile}.omnisshagent-backup-${TIMESTAMP}"
  if [ "$DRY_RUN" -eq 1 ]; then
    say "[dry-run] Would back up $profile to $backup"
    say "[dry-run] Would remove lines containing $NAME from $profile"
    return
  fi

  cp -p "$profile" "$backup"
  temporary=$(mktemp "${profile}.omnisshagent.XXXXXX")
  if ! awk -v name="$NAME" 'index($0, name) == 0' "$profile" > "$temporary"; then
    rm -f "$temporary"
    say "Could not update $profile; backup retained at $backup" >&2
    exit 1
  fi
  cat "$temporary" > "$profile"
  rm -f "$temporary"
  say "Updated $profile (backup: $backup)"
}

remove_legacy_files() {
  case "$PROXY_DIR" in
    "$HOME_REAL/$NAME")
      ;;
    *)
      say "Refusing unsafe proxy directory: $PROXY_DIR" >&2
      exit 1
      ;;
  esac
  case "$SOCKET_DIR" in
    "$HOME_REAL/.ssh/$NAME")
      ;;
    *)
      say "Refusing unsafe socket directory: $SOCKET_DIR" >&2
      exit 1
      ;;
  esac

  if [ -e "$PROXY_DIR" ]; then
    if [ "$DRY_RUN" -eq 1 ]; then
      say "[dry-run] Would remove $PROXY_DIR"
    else
      rm -rf "$PROXY_DIR"
      say "Removed $PROXY_DIR"
    fi
  else
    say "Legacy proxy directory: not found"
  fi

  if [ -e "$SOCKET_PATH" ] || [ -L "$SOCKET_PATH" ]; then
    if [ "$DRY_RUN" -eq 1 ]; then
      say "[dry-run] Would remove $SOCKET_PATH"
    else
      rm -f "$SOCKET_PATH"
      say "Removed $SOCKET_PATH"
    fi
  fi

  if [ -d "$SOCKET_DIR" ]; then
    if [ "$DRY_RUN" -eq 1 ]; then
      say "[dry-run] Would remove $SOCKET_DIR if it is empty"
    elif rmdir "$SOCKET_DIR" 2>/dev/null; then
      say "Removed empty $SOCKET_DIR"
    else
      say "Retained non-empty $SOCKET_DIR"
    fi
  fi
}

say "Legacy OmniSSHAgent WSL2 proxy uninstaller"
if [ "$DRY_RUN" -eq 1 ]; then
  say "Dry-run mode: no process or file changes will be made."
fi

stop_proxy

remove_profile_reference "$HOME_REAL/.bashrc"
remove_profile_reference "$HOME_REAL/.zshrc"
remove_profile_reference "$HOME_REAL/.profile"
remove_profile_reference "$HOME_REAL/.config/fish/config.fish"

remove_legacy_files

say "Legacy $NAME uninstall completed."
say "Open a new shell before configuring the replacement SSH_AUTH_SOCK."
