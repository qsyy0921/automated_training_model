---
name: automated-training-data-lake
description: Use when ingesting raw datasets, copied training inputs, derived labels, model checkpoints, or Hugging Face model artifacts into this project's data_lake without committing large files.
---

# Automated Training Data Lake

Use this skill when the task is to place datasets or models under
`F:\automated_training_model\data_lake` and keep Git clean. The data lake is the
local source of truth for large artifacts; Git only receives code, manifests,
catalog metadata, scripts, and documentation.

## Rules

- Never add raw data, model weights, checkpoints, videos, frames, or copied
  artifacts to Git.
- Put raw datasets under `data_lake/raw/datasets/<dataset-id>/original`.
- Put derived labels or tracking outputs under
  `data_lake/derived/<dataset-id>/<producer>/<run-id>`.
- Put downloaded model repositories under
  `data_lake/models/artifacts/huggingface/<org>/<repo>`.
- Put training checkpoints under
  `data_lake/models/checkpoints/<project>/<run-id>`.
- Put small catalogs and manifests under `data_lake/catalog` only if they do
  not contain large binary payloads.
- If a command may download model weights, confirm the destination and expected
  size before running it.

## Raw Dataset Ingest

Use `scripts/register_raw_dataset.ps1` for a repeatable copy plus catalog:

```powershell
powershell -ExecutionPolicy Bypass -File skills\automated-training-data-lake\scripts\register_raw_dataset.ps1 `
  -DatasetId shanghaitech-original `
  -SourceRoot F:\keyan\token_compression\data\shanghai\data
```

The script copies with `robocopy`, writes a catalog JSON under
`data_lake/catalog/datasets`, and does not touch the app's active dataset
registry.

## Hugging Face Model Ingest

Use `scripts/download_hf_model.ps1` for model repositories:

```powershell
powershell -ExecutionPolicy Bypass -File skills\automated-training-data-lake\scripts\download_hf_model.ps1 `
  -RepoId nvidia/LocateAnything-3B `
  -PullLFS
```

Without `-PullLFS`, the script clones metadata with `GIT_LFS_SKIP_SMUDGE=1`.
With `-PullLFS`, it runs `git lfs pull` after cloning.

## Validation

After ingest:

```powershell
git status --short
Get-ChildItem data_lake\catalog -Recurse -File | Select-Object FullName,Length
```

Expected result: catalogs and scripts may be visible to Git; artifact
directories remain ignored.
