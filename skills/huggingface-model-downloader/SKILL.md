---
name: huggingface-model-downloader
description: Use when downloading HuggingFace model repositories into this project's data_lake, especially large LFS-backed model weights that must not be committed to Git.
---

# HuggingFace Model Downloader

Use this skill to download external HuggingFace model repositories into
`F:\automated_training_model\data_lake\models\artifacts\huggingface` while
keeping Git clean.

## Rules

- Never commit model weights, checkpoints, tokenizer binaries, safetensors, or
  HuggingFace cache files.
- Download repositories under
  `data_lake/models/artifacts/huggingface/<org>/<repo>`.
- Use environment variables for credentials:
  - `HF_TOKEN` or `HUGGINGFACE_HUB_TOKEN` for gated/private models.
  - `HF_HOME` for global HuggingFace cache location if needed.
- Prefer resumable snapshot download through `huggingface_hub`.
- Record only small manifests or verification notes in Git.
- In local development, Agent Runtime has download permission by default. To
  require approval before real downloads, set
  `AGENT_RUNTIME_REQUIRE_MODEL_DOWNLOAD_APPROVAL=true`; then tool-call params
  must include `approved=true`.

## Download

For `nvidia/LocateAnything-3B`:

```powershell
python skills\huggingface-model-downloader\scripts\download_hf_snapshot.py `
  --repo-id nvidia/LocateAnything-3B `
  --local-dir data_lake\models\artifacts\huggingface\nvidia\LocateAnything-3B `
  --manifest data_lake\catalog\models\nvidia_LocateAnything-3B.download.json
```

The script uses `huggingface_hub.snapshot_download`, supports resume behavior,
and writes a small JSON manifest with file counts and total bytes. Dry-run and
verify-only both query HuggingFace remote file metadata first, so the manifest
can record expected file count and expected total bytes before downloading.

## Validation

Dry-run without downloading weights:

```powershell
python skills\huggingface-model-downloader\scripts\download_hf_snapshot.py `
  --repo-id nvidia/LocateAnything-3B `
  --local-dir data_lake\models\artifacts\huggingface\nvidia\LocateAnything-3B `
  --manifest data_lake\catalog\models\nvidia_LocateAnything-3B.download.json `
  --dry-run
```

After download:

```powershell
python skills\huggingface-model-downloader\scripts\download_hf_snapshot.py `
  --repo-id nvidia/LocateAnything-3B `
  --local-dir data_lake\models\artifacts\huggingface\nvidia\LocateAnything-3B `
  --manifest data_lake\catalog\models\nvidia_LocateAnything-3B.download.json `
  --verify-only

git status --short
```

Expected result: the model directory stays ignored by Git; only code, scripts,
docs, and small catalog manifests can appear in Git.

For `nvidia/LocateAnything-3B`, the current public remote manifest reports 38
files and 7,795,875,224 bytes. The largest files are two safetensors shards, so
real download and verify-only should be treated as a long-running Agent Runtime
model job.
