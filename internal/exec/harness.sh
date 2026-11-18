# md-exec bash harness (SPEC.md §5.1).
#
# Sources the block inside a controlled shell and dumps the shell's exported
# environment — the environment the block would hand to its children (§4.3) —
# from an EXIT trap. The trap fires even when user code exits early (`exit`,
# `set -e`), and the shell's exit status passes through unchanged.
#
# `compgen -e` lists exported names without spawning a child (a child would
# inject `_=/usr/bin/env` into the dump). Values are NUL-delimited so
# multi-line values survive losslessly (§4.3). Compatible with bash 3.2.
__mdexec_dump() {
  {
    for __mdexec_v in $(compgen -e); do
      printf '%s=%s\0' "$__mdexec_v" "${!__mdexec_v}"
    done
  } > "$__MDEXEC_ENVOUT"
}
trap __mdexec_dump EXIT
source "$__MDEXEC_BLOCK"
