"""Referenced via @scripts/register.py — executed by the runner's python3 adapter.

os.environ writes are harvested exactly like bash `export` (spec §4.3).
"""
import json
import os

manifest = {
    "job": os.environ["JOB_NAME"],
    "workspace": os.environ["WORK"],
    "lines": os.environ.get("LOG_LINES", "").splitlines(),
}
with open(os.path.join(os.environ["WORK"], "manifest.json"), "w") as f:
    json.dump(manifest, f, indent=2)

os.environ["DEPLOY_ID"] = f"dpl-{len(manifest['lines'])}x-{os.environ['JOB_NAME']}"
print("registered:", os.environ["DEPLOY_ID"])
