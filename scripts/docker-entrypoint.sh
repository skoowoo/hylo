#!/bin/sh
set -eu

# The built-in defaults listen on loopback, which is unreachable from outside the container.
cfg="$HOME/.hylo/config.toml"
if [ ! -f "$cfg" ]; then
  mkdir -p "$HOME/.hylo"
  cat >"$cfg" <<'EOF'
[server]
host = "0.0.0.0"
port = 54321
write_timeout = 1800

[vault]
path = "~/vault"
EOF
fi

exec "$@"
