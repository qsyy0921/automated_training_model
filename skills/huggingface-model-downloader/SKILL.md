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
- In Agent Runtime, `model.download_hf` is protected by an approval gate. The
  default response is a preflight plan; real downloads require `approved=true`
  in the tool-call params or `AGENT_RUNTIME_ALLOW_MODEL_DOWNLOAD=true` on the
  server.

## Download

For `nvidia/LocateAnything-3B`:

```powershell
python skills\huggingface-model-downloader\scripts\download_hf_snapshot.py `
  --repo-id nvidia/LocateAnything-3B `
  --local-dir data_lake\models\artifacts\huggingface\nvidia\LocateAnything-3B `
  --manifest data_lake\catalog\models\nvidia_LocateAnything-3B.download.json
```

The script uses `huggingface_hub.snapshot_download`, supports resume behavior,
and writes a small JSON manifest with file counts and total bytes.

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
