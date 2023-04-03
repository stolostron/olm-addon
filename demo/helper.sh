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

