"""exmd python harness (SPEC.md §5.1).

Runs the block as __main__ via runpy and dumps the final os.environ —
the environment the block would hand to its children (§4.3) — from an
atexit hook, which fires on sys.exit and uncaught exceptions alike.
os.fsencode round-trips names and values losslessly, including
multi-line values (§4.3).
"""
import atexit
import os
import runpy


def __mdexec_dump():
    with open(os.environ["__MDEXEC_ENVOUT"], "wb") as f:
        for k, v in os.environ.items():
            f.write(os.fsencode(k) + b"=" + os.fsencode(v) + b"\x00")


atexit.register(__mdexec_dump)
runpy.run_path(os.environ["__MDEXEC_BLOCK"], run_name="__main__")
