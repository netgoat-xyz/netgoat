#!/usr/bin/env bash
set -euo pipefail

DEFAULT_DOMAIN="netgoat.test"
DEFAULT_IP="127.0.0.1"
DEFAULT_PORT="8080"
DEFAULT_TARGET="http://127.0.0.1:8888"
HOSTS_FILE="${HOSTS_FILE:-/etc/hosts}"
MARKER_NAME="NETGOAT TEST DOMAIN"

domain="${NETGOAT_TEST_DOMAIN:-$DEFAULT_DOMAIN}"
ip="${NETGOAT_TEST_IP:-$DEFAULT_IP}"
port="${NETGOAT_TEST_PORT:-$DEFAULT_PORT}"
target="${NETGOAT_TEST_TARGET:-$DEFAULT_TARGET}"
aliases=()

usage() {
  cat <<EOF
Usage:
  $0 add [options]
  $0 remove [options]
  $0 status [options]
  $0 print-config [options]
  $0 test-url [options]

Options:
  --domain <name>      Domain to map. Default: ${DEFAULT_DOMAIN}
  --ip <address>       IP to map to. Default: ${DEFAULT_IP}
  --port <port>        Agent port used by test-url. Default: ${DEFAULT_PORT}
  --target <url>       Upstream target shown in print-config. Default: ${DEFAULT_TARGET}
  --alias <name>       Extra host alias. Can be repeated.
  --hosts-file <path>  Hosts file path. Default: /etc/hosts
  -h, --help           Show this help.

Examples:
  sudo $0 add --domain app.netgoat.test --alias api.app.netgoat.test
  $0 print-config --domain app.netgoat.test --target http://127.0.0.1:8888
  $0 test-url --domain app.netgoat.test --port 8080
  sudo $0 remove

Notes:
  /etc/hosts does not support wildcard domains. Add each hostname you need as
  the base domain or via repeated --alias flags.
EOF
}

die() {
  echo "error: $*" >&2
  exit 1
}

need_command() {
  command -v "$1" >/dev/null 2>&1 || die "missing required command: $1"
}

is_ipv4() {
  [[ "$1" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}$ ]]
}

begin_marker() {
  echo "# BEGIN ${MARKER_NAME}"
}

end_marker() {
  echo "# END ${MARKER_NAME}"
}

all_hosts() {
  printf "%s" "$domain"
  for alias in "${aliases[@]}"; do
    printf " %s" "$alias"
  done
  printf "\n"
}

require_hosts_write() {
  if [[ ! -w "$HOSTS_FILE" ]]; then
    die "${HOSTS_FILE} is not writable. Re-run with sudo or set HOSTS_FILE to a writable test file."
  fi
}

backup_hosts() {
  local backup="${HOSTS_FILE}.netgoat.$(date +%Y%m%d%H%M%S).bak"
  cp "$HOSTS_FILE" "$backup"
  echo "backup: $backup"
}

strip_block() {
  awk -v begin="$(begin_marker)" -v end="$(end_marker)" '
    $0 == begin { skip = 1; next }
    $0 == end { skip = 0; next }
    skip != 1 { print }
  ' "$HOSTS_FILE"
}

flush_dns() {
  case "$(uname -s)" in
    Darwin)
      dscacheutil -flushcache >/dev/null 2>&1 || true
      killall -HUP mDNSResponder >/dev/null 2>&1 || true
      ;;
    Linux)
      if command -v resolvectl >/dev/null 2>&1; then
        resolvectl flush-caches >/dev/null 2>&1 || true
      elif command -v systemd-resolve >/dev/null 2>&1; then
        systemd-resolve --flush-caches >/dev/null 2>&1 || true
      fi
      ;;
  esac
}

add_hosts() {
  require_hosts_write
  backup_hosts

  local tmp
  tmp="$(mktemp)"
  strip_block > "$tmp"
  {
    echo
    begin_marker
    printf "%s %s\n" "$ip" "$(all_hosts)"
    end_marker
  } >> "$tmp"

  cat "$tmp" > "$HOSTS_FILE"
  rm -f "$tmp"
  flush_dns

  echo "added: $(all_hosts | tr '\n' ' ') -> ${ip}"
  echo "test:  http://${domain}:${port}/"
}

remove_hosts() {
  require_hosts_write
  backup_hosts

  local tmp
  tmp="$(mktemp)"
  strip_block > "$tmp"
  cat "$tmp" > "$HOSTS_FILE"
  rm -f "$tmp"
  flush_dns

  echo "removed ${MARKER_NAME} block from ${HOSTS_FILE}"
}

status_hosts() {
  echo "hosts file: ${HOSTS_FILE}"
  if grep -Fq "$(begin_marker)" "$HOSTS_FILE"; then
    sed -n "/$(begin_marker)/,/$(end_marker)/p" "$HOSTS_FILE"
  else
    echo "no ${MARKER_NAME} block found"
  fi

  echo
  echo "resolution:"
  if command -v getent >/dev/null 2>&1; then
    getent hosts "$domain" || true
  elif command -v dscacheutil >/dev/null 2>&1; then
    dscacheutil -q host -a name "$domain" || true
  else
    ping -c 1 "$domain" 2>/dev/null | sed -n '1p' || true
  fi
}

print_config() {
  cat <<EOF
Add this route to config.yml when testing the local agent:

routes:
  ${domain}:
    type: "domain"
    targets:
      - url: "${target}"
        health_check: "http"
EOF

  for alias in "${aliases[@]}"; do
    cat <<EOF
  ${alias}:
    type: "domain"
    targets:
      - url: "${target}"
        health_check: "http"
EOF
  done
}

test_url() {
  need_command curl
  echo "curling http://${domain}:${port}/"
  curl -v "http://${domain}:${port}/"
}

action="${1:-}"
[[ -n "$action" ]] || { usage; exit 2; }
shift || true

while [[ $# -gt 0 ]]; do
  case "$1" in
    --domain)
      domain="${2:-}"
      shift 2
      ;;
    --ip)
      ip="${2:-}"
      shift 2
      ;;
    --port)
      port="${2:-}"
      shift 2
      ;;
    --target)
      target="${2:-}"
      shift 2
      ;;
    --alias)
      aliases+=("${2:-}")
      shift 2
      ;;
    --hosts-file)
      HOSTS_FILE="${2:-}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      die "unknown option: $1"
      ;;
  esac
done

[[ -n "$domain" ]] || die "--domain cannot be empty"
[[ -n "$ip" ]] || die "--ip cannot be empty"
[[ -n "$port" ]] || die "--port cannot be empty"
[[ -f "$HOSTS_FILE" ]] || die "hosts file does not exist: $HOSTS_FILE"
is_ipv4 "$ip" || die "--ip must be an IPv4 address"

case "$action" in
  add)
    add_hosts
    ;;
  remove)
    remove_hosts
    ;;
  status)
    status_hosts
    ;;
  print-config)
    print_config
    ;;
  test-url)
    test_url
    ;;
  -h|--help)
    usage
    ;;
  *)
    die "unknown action: $action"
    ;;
esac
