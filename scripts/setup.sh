#!/usr/bin/env bash

# NetGoat interactive manual setup
# - Copies .env.example -> .env for root, LogDB, CentralMonServer
# - Prompts for required values and writes them to .env files
# - Updates reactbased/next.config.ts API endpoints
# - Installs dependencies via bun install in all packages
# - Optionally builds the frontend (prod)
#
# IMPORTANT: This script does NOT configure systemd. Use your preferred process manager
# (PM2, systemd, etc.) separately. A separate helper for systemd exists at scripts/install-systemd.sh
# but is not invoked here.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
REACT_DIR="$ROOT_DIR/reactbased"
LOGDB_DIR="$ROOT_DIR/LogDB"
CTM_DIR="$ROOT_DIR/CentralMonServer"
FRONTEND_NEXT_CFG="$REACT_DIR/next.config.ts"

# Args / flags
AUTO=0
for arg in "$@"; do
  case "$arg" in
    -y|--yes|--auto)
      AUTO=1
      ;;
    -h|--help)
      cat <<EOF
NetGoat Manual Setup Wizard

Usage: bash scripts/setup.sh [--auto]

Options:
  --auto, -y, --yes   Non-interactive mode. Use defaults, overwrite .env files,
                      and auto-generate all secrets/tokens without prompting.
EOF
      exit 0
      ;;
  esac
done

color() { # $1=color, $2=text
  local c="$1"; shift
  local t="$*"
  case "$c" in
    red) echo -e "\033[31m$t\033[0m" ;;
    green) echo -e "\033[32m$t\033[0m" ;;
    yellow) echo -e "\033[33m$t\033[0m" ;;
    blue) echo -e "\033[34m$t\033[0m" ;;
    *) echo "$t" ;;
  esac
}

prompt() { # $1=question $2=default -> echo answer
  local q="$1"; shift
  local d="${1:-}"
  local ans
  if [[ "$AUTO" == "1" ]]; then
    echo "${d}"
    return 0
  fi
  if [[ -n "$d" ]]; then
    read -r -p "$q [$d]: " ans || true
    echo "${ans:-$d}"
  else
    read -r -p "$q: " ans || true
    echo "$ans"
  fi
}

prompt_secret() { # $1=question (no default)
  local q="$1"
  local ans
  if [[ "$AUTO" == "1" ]]; then
    # Return empty to trigger auto-generation where applicable
    echo
    return 0
  fi
  read -r -s -p "$q: " ans || true
  echo
  echo "$ans"
}

ask_yes_no() { # $1=question $2=default(Y/N) -> return 0 for yes
  local q="$1"; local d="${2:-Y}"; local ans
  if [[ "$AUTO" == "1" ]]; then
    [[ "${d^^}" == "Y" ]] && return 0 || return 1
  fi
  while true; do
    read -r -p "$q [${d^}/$([[ $d == Y ]] && echo n || echo y)]: " ans || true
    ans="${ans:-$d}"
    case "${ans^^}" in
      Y|YES) return 0 ;;
      N|NO) return 1 ;;
      *) echo "Please answer y or n." ;;
    esac
  done
}

ensure_file() { # $1=src $2=dst
  local src="$1" dst="$2"
  if [[ -f "$dst" ]]; then
    if [[ "$AUTO" == "1" ]]; then
      cp -f "$src" "$dst"
    else
      if ask_yes_no "Found $(basename "$dst"). Overwrite with example and re-apply values?" N; then
        cp -f "$src" "$dst"
      else
        return 0
      fi
    fi
  else
    cp "$src" "$dst"
  fi
}

set_kv() { # $1=file $2=KEY $3=value (unquoted)
  local file="$1" key="$2" value="$3"
  # Escape regex meta for key when used in sed/grep
  local key_esc
  key_esc=$(printf '%s' "$key" | sed -e 's/[][\\.^$*+?|(){}]/\\&/g')
  # Remove existing key line(s) if present
  if grep -qE "^${key_esc}=" "$file"; then
    sed -i -E "/^${key_esc}=.*/d" "$file"
  fi
  # Append new key=value (value is written verbatim)
  printf '%s=%s\n' "$key" "$value" >> "$file"
}

escape_sed() { # escape replacement for sed
  echo "$1" | sed -e 's/[\/&]/\\&/g'
}

update_next_config_urls() {
  local backend_url="$1" logdb_url="$2" file="$FRONTEND_NEXT_CFG"
  if [[ ! -f "$file" ]]; then
    color yellow "[WARN] $file not found; skipping frontend URL update"
    return 0
  fi
  BACKEND="$backend_url" LOGDB="$logdb_url" bun -e '
    const fs = require("fs");
    const f = process.argv[1];
    let s = fs.readFileSync(f, "utf8");
    const be = process.env.BACKEND || "";
    const lg = process.env.LOGDB || "";
    s = s.replace(/(backendapi:\s*")[^"]*(")/, `$1${be}$2`);
    s = s.replace(/(logdb:\s*")[^"]*(")/, `$1${lg}$2`);
    fs.writeFileSync(f, s);
  ' "$file"
  color green "[OK] Updated frontend API URLs in $(basename "$file")"
}

run_bun_install() {
  local dir="$1"
  if [[ ! -d "$dir" ]]; then return 0; fi
  (cd "$dir" && bun install)
}

clear
echo
echo "$(color blue "NetGoat Manual Setup Wizard")"
echo "Root: $ROOT_DIR"
echo

# Prereq check: bun presence
if ! command -v bun >/dev/null 2>&1; then
  color red "[ERROR] Bun is not installed or not in PATH."
  echo "Install Bun and re-run this script. See https://bun.sh"
  exit 1
fi

echo "Step 1/6: Copying .env.example -> .env"
ensure_file "$ROOT_DIR/.env.example" "$ROOT_DIR/.env"
ensure_file "$LOGDB_DIR/.env.example" "$LOGDB_DIR/.env"
ensure_file "$CTM_DIR/.env.example" "$CTM_DIR/.env"
echo

echo "Step 2/6: Collecting configuration values"

# Defaults
DEFAULT_REGION="MM1"
DEFAULT_MONGO_ROOT="mongodb://localhost/netgoat"
DEFAULT_CTM_URL="http://localhost:1933"
DEFAULT_LOGDB_URL="http://localhost:3010"
DEFAULT_BACKEND_URL="http://localhost:3001"
DEFAULT_CTM_PORT="1933"

REGION_ID=$(prompt "Region ID (e.g., MM1)" "$DEFAULT_REGION")
MONGO_ROOT=$(prompt "MongoDB URI for NetGoat (root .env)" "$DEFAULT_MONGO_ROOT")
CTM_URL=$(prompt "CentralMonServer URL" "$DEFAULT_CTM_URL")
SHARED_JWT=$(prompt_secret "Shared JWT Secret (SHARED_JWT_SECRET) [leave blank to auto-generate]")
CENTRAL_JWT=$(prompt_secret "Central Server JWT (Central_JWT) [leave blank to auto-generate]")

echo
if ask_yes_no "Use the same MongoDB URI for CentralMonServer as root (.env)?" Y; then
  CTM_MONGO="$MONGO_ROOT"
else
  CTM_MONGO=$(prompt "MongoDB URI for CentralMonServer" "$DEFAULT_MONGO_ROOT")
fi
CTM_PORT=$(prompt "CentralMonServer Port" "$DEFAULT_CTM_PORT")
CTM_SHARED_JWT=${SHARED_JWT}
CTM_DYNAMIC_JWT=$(prompt_secret "CTM Dynamic Secret Key JWT (DYNAMIC_SECRET_KEY_JWT_SECRET) [leave blank to auto-generate]")

# Auto-generate secrets/tokens if left blank
GEN_NOTES=()
if [[ -z "$SHARED_JWT" ]]; then
  SHARED_JWT=$(bun -e 'console.log(require("crypto").randomBytes(32).toString("hex"))')
  CTM_SHARED_JWT="$SHARED_JWT"
  GEN_NOTES+=("Generated SHARED_JWT_SECRET")
fi
if [[ -z "$CTM_DYNAMIC_JWT" ]]; then
  CTM_DYNAMIC_JWT=$(bun -e 'console.log(require("crypto").randomBytes(32).toString("hex"))')
  GEN_NOTES+=("Generated DYNAMIC_SECRET_KEY_JWT_SECRET")
fi
if [[ -z "$CENTRAL_JWT" ]]; then
  CENTRAL_JWT=$(SHARED="$SHARED_JWT" bun -e '
    const jwt = require("jsonwebtoken");
    const secret = process.env.SHARED;
    const token = jwt.sign({ role: "agent" }, secret, { algorithm: "HS256", expiresIn: "90d" });
    console.log(token);
  ')
  GEN_NOTES+=("Generated Central_JWT signed with SHARED_JWT_SECRET (expires in 90d)")
fi

echo
BACKEND_URL=$(prompt "Frontend Backend API URL" "$DEFAULT_BACKEND_URL")
LOGDB_URL=$(prompt "Frontend LogDB API URL" "$DEFAULT_LOGDB_URL")

echo
echo "Step 3/6: Writing .env files"
# Root .env
set_kv "$ROOT_DIR/.env" regionID "$REGION_ID"
set_kv "$ROOT_DIR/.env" mongodb "$MONGO_ROOT"
set_kv "$ROOT_DIR/.env" Central_server "$CTM_URL"
set_kv "$ROOT_DIR/.env" Central_JWT "${CENTRAL_JWT}"
set_kv "$ROOT_DIR/.env" SHARED_JWT_SECRET "$SHARED_JWT"

# LogDB .env (reuse values)
set_kv "$LOGDB_DIR/.env" regionID "$REGION_ID"
set_kv "$LOGDB_DIR/.env" Central_server "$CTM_URL"
set_kv "$LOGDB_DIR/.env" Central_JWT "${CENTRAL_JWT}"
set_kv "$LOGDB_DIR/.env" SHARED_JWT_SECRET "$SHARED_JWT"

# CentralMonServer .env
set_kv "$CTM_DIR/.env" MONGODB_URI "$CTM_MONGO"
set_kv "$CTM_DIR/.env" SHARED_JWT_SECRET "$CTM_SHARED_JWT"
set_kv "$CTM_DIR/.env" DYNAMIC_SECRET_KEY_JWT_SECRET "$CTM_DYNAMIC_JWT"
set_kv "$CTM_DIR/.env" PORT "$CTM_PORT"

echo "Step 4/6: Updating frontend API endpoints"
update_next_config_urls "$BACKEND_URL" "$LOGDB_URL"

echo "Step 5/6: Installing dependencies with bun"
run_bun_install "$ROOT_DIR"
run_bun_install "$LOGDB_DIR"
run_bun_install "$CTM_DIR"
run_bun_install "$REACT_DIR"

echo
echo "$(color green "All done!")"
if ((${#GEN_NOTES[@]})); then
  echo "$(color blue "Auto-generated:") ${GEN_NOTES[*]}"
fi
echo
echo "Next steps (manual run examples):"
echo "  - Start NetGoat core (from repo root):   bun ."
echo "  - Start LogDB:                            (cd $LOGDB_DIR && bun .)"
echo "  - Start CentralMonServer:                 (cd $CTM_DIR && bun .)"
echo "  - Start Frontend (dev):                   (cd $REACT_DIR && bun run dev)"
echo "  - Start Frontend (prod):                  (cd $REACT_DIR && bun run start)"
echo "  - Start everything together:              bun run stack  # or: bun stack"
echo
echo "You can manage processes with PM2 or start them manually as shown above."

echo
echo "Documentation: https://docs.netgoat.xyz (check back for updates!)"
