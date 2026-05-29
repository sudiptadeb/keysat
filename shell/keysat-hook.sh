#!/bin/bash
# Keysat shell hook - reports current directory via Unix socket (instant, no HTTP)
# Add to your .bashrc or .zshrc:
#   source /path/to/keysat-hook.sh

KEYSAT_SOCK="$HOME/.keysat/hook.sock"

_keysat_report_dir() {
    local rc=$?
    (printf '%s' "$PWD" | nc -w1 -U "$KEYSAT_SOCK") 2>/dev/null & disown
    return $rc
}

# Bash support
if [ -n "$BASH_VERSION" ]; then
    PROMPT_COMMAND="_keysat_report_dir${PROMPT_COMMAND:+;$PROMPT_COMMAND}"
fi

# Zsh support
if [ -n "$ZSH_VERSION" ]; then
    autoload -Uz add-zsh-hook
    add-zsh-hook precmd _keysat_report_dir
fi
