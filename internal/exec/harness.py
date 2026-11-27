"""exmd python harness (SPEC.md §5.1).

Runs the block as __main__ via runpy and dumps the final os.environ —
the environment the block would hand to its children (§4.3) — from an
atexit hook, which fires on sys.exit and uncaught exceptions alike.
os.fsencode round-trips names and values losslessly, including
multi-line values (§4.3). The dump ends with a `__MDEXEC_END=` record; a
dump without it was cut short and is ignored.

The block runs as if started with `python3 FILE` (§5.1.4): sys.argv[0] is
its file and sys.path[0] its directory — the script's own directory for a
script reference, the document's directory (the working directory) for an
embedded block — so modules beside it import.
"""
import atexit
import os
import runpy
import sys

__mdexec_out = os.environ["__MDEXEC_ENVOUT"]
__mdexec_block = os.environ["__MDEXEC_BLOCK"]


def __mdexec_dump():
    with open(__mdexec_out, "wb") as f:
        for k, v in os.environ.items():
            f.write(os.fsencode(k) + b"=" + os.fsencode(v) + b"\x00")
        f.write(b"__MDEXEC_END=\x00")


def __mdexec_source_dir():
    block_dir = os.path.dirname(os.path.abspath(__mdexec_block))
    if block_dir == os.path.dirname(os.path.abspath(__file__)):
        return os.getcwd()  # staged embedded block: the document's directory
    return block_dir


atexit.register(__mdexec_dump)
sys.argv[0] = __mdexec_block
sys.path[0] = __mdexec_source_dir()
runpy.run_path(__mdexec_block, run_name="__main__")
