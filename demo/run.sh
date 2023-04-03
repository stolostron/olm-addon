#!/usr/bin/env bash

export DEMO_DIR="$( dirname "${BASH_SOURCE[0]}" )"

source ${DEMO_DIR}/demo-magic

export TERM=xterm-256color

cols=100
if command -v tput &> /dev/null; then
  output=$(echo -e cols | tput -S)
  if [[ -n "${output}" ]]; then
    cols=$((output - 10))
  fi
fi
export cols

TYPE_SPEED=30
DEMO_PROMPT="olm-demo $ "

echo "Starting demo"

pe "echo 'This is a command'"
