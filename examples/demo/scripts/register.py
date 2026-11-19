"""Referenced via @scripts/register.py — executed by the runner's python3 adapter.

os.environ writes are harvested exactly like bash `export` (spec §4.3).
During a full run, WORK and LOG_LINES arrive from earlier blocks; when this
block is run on its own, it falls back to a scratch workspace and an empty
line set so it still succeeds.
"""
import json
import os
import tempfile

job = os.environ.get("JOB_NAME", "standalone")
workspace = os.environ.get("WORK") or tempfile.mkdtemp(prefix="mdexec-demo-")
os.makedirs(workspace, exist_ok=True)

manifest = {
    "job": job,
    "workspace": workspace,
    "lines": os.environ.get("LOG_LINES", "").splitlines(),
}
with open(os.path.join(workspace, "manifest.json"), "w") as f:
    json.dump(manifest, f, indent=2)

os.environ["DEPLOY_ID"] = f"dpl-{len(manifest['lines'])}x-{job}"
print("registered:", os.environ["DEPLOY_ID"])
