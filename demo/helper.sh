#!/usr/bin/env bash

function pause() {
  if [[ "${NO_WAIT}" = "true" ]]; then
    sleep 2
  else
    if [[ -n "${1-}" ]]; then
      sleep "$1"
    else
      wait
    fi
  fi
}

function c() {
  local comment="$*"
  if command -v fold &> /dev/null; then
    comment=$(echo "$comment" | fold -w "${cols:-100}")
  fi
  p "# $comment"
}

function wait_command() {
  local command="$1";
  local wait_seconds="${2:-40}"; # 40 seconds as default timeout
  until [[ $((wait_seconds--)) -eq 0 ]] || eval "$command 2>/dev/null" ; do sleep 1 && echo -n "."; done
  echo ""
  ((++wait_seconds))
}
