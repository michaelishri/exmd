# exmd bash harness (SPEC.md §5.1).
#
# Sources the block inside a controlled shell and dumps the shell's exported
# environment — the environment the block would hand to its children (§4.3) —
# from an EXIT trap. The trap fires even when user code exits early (`exit`,
# `set -e`), and the shell's exit status passes through unchanged.
#
# `compgen -e` lists exported names without spawning a child (a child would
# inject `_=/usr/bin/env` into the dump). Values are NUL-delimited so
# multi-line values survive losslessly (§4.3). The dump ends with a
# `__MDEXEC_END=` record; a dump without it was cut short and is ignored.
#
# Blocks commonly install their own EXIT trap. `trap` is wrapped so such a
# handler runs alongside the dump instead of replacing it, and `trap -p`
# still reports the block's own handler (§5.1.4). Compatible with bash 3.2.
__mdexec_out=$__MDEXEC_ENVOUT
__mdexec_user_exit=

__mdexec_dump() {
  local IFS=$' \t\n' __mdexec_v
  {
    for __mdexec_v in $(builtin compgen -e); do
      builtin printf '%s=%s\0' "$__mdexec_v" "${!__mdexec_v-}"
    done
    builtin printf '__MDEXEC_END=\0'
  } >| "$__mdexec_out"
}

# __mdexec_status sets $? so the block's EXIT handler sees the exit status.
__mdexec_status() { return "$1"; }

__mdexec_on_exit() {
  local __mdexec_rc=$?
  __mdexec_dump
  if [ -n "$__mdexec_user_exit" ]; then
    __mdexec_status "$__mdexec_rc"
    eval "$__mdexec_user_exit"
    # The handler may have exported more; record the final environment.
    __mdexec_dump
  fi
}

__mdexec_is_exit() {
  case $1 in
    0 | [Ee][Xx][Ii][Tt] | [Ss][Ii][Gg][Ee][Xx][Ii][Tt]) return 0 ;;
  esac
  return 1
}

# __mdexec_print_exit prints the block's EXIT handler the way `trap -p` does.
__mdexec_print_exit() {
  if [ -n "$__mdexec_user_exit" ]; then
    # Quote as `trap -p` does: ' becomes '\''. The pattern and replacement
    # live in variables because bash 3.2 and 4+ treat quotes inside
    # ${var//pat/rep} differently.
    local __mdexec_sq="'" __mdexec_esc="'\\''"
    builtin printf "trap -- '%s' EXIT\n" "${__mdexec_user_exit//$__mdexec_sq/$__mdexec_esc}"
  fi
}

trap() {
  # Subshells do not run the harness dump: their traps are the block's own.
  if [ "${BASH_SUBSHELL:-0}" -gt 0 ] && [ "${1-}" != -p ] && [ $# -gt 0 ]; then
    builtin trap "$@"
    return
  fi
  [ $# -eq 0 ] && set -- -p
  case $1 in
    -l) builtin trap "$@"; return ;;
    -p)
      shift
      local __mdexec_sig
      if [ $# -eq 0 ]; then
        __mdexec_print_exit
        for __mdexec_sig in $(builtin compgen -A signal); do
          builtin trap -p "$__mdexec_sig"
        done
        return
      fi
      for __mdexec_sig in "$@"; do
        if __mdexec_is_exit "$__mdexec_sig"; then
          __mdexec_print_exit
        else
          builtin trap -p "$__mdexec_sig" || return
        fi
      done
      return
      ;;
    --) shift ;;
  esac
  # A lone signal resets it, as `trap - SIG` does.
  [ $# -eq 1 ] && set -- - "$1"
  local __mdexec_action=$1 __mdexec_sig __mdexec_rc=0
  shift
  for __mdexec_sig in "$@"; do
    if __mdexec_is_exit "$__mdexec_sig"; then
      if [ "$__mdexec_action" = - ]; then
        __mdexec_user_exit=
      else
        __mdexec_user_exit=$__mdexec_action
      fi
    else
      builtin trap -- "$__mdexec_action" "$__mdexec_sig" || __mdexec_rc=$?
    fi
  done
  return "$__mdexec_rc"
}

builtin trap __mdexec_on_exit EXIT
source "$__MDEXEC_BLOCK"
