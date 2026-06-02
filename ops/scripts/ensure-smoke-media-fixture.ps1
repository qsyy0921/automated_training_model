function Ensure-SmokeMediaFixture {
  param(
    [string]$RepoRoot,
    [string]$MergeRoot,
    [string]$FrameRoot,
    [string]$MaskRoot,
    [string]$AnnotationRoot
  )

  $csvDir = Join-Path $MergeRoot "csv"
  $hasCSV = (Test-Path -LiteralPath $csvDir) -and (@(Get-ChildItem -LiteralPath $csvDir -Filter "*.csv" -File -ErrorAction SilentlyContinue).Count -gt 0)
  if ($hasCSV) {
    return [pscustomobject]@{
      MergeRoot = $MergeRoot
      FrameRoot = $FrameRoot
      MaskRoot = $MaskRoot
      AnnotationRoot = $AnnotationRoot
      Fixture = $false
    }
  }

  $root = Join-Path $RepoRoot "tmp\smoke-media"
  $merge = Join-Path $root "merge"
  $frames = Join-Path $root "frames"
  $masks = Join-Path $root "masks"
  $annotations = Join-Path $root "annotations_review"
  $scene = "smoke_scene"

  New-Item -ItemType Directory -Force -Path (Join-Path $merge "csv") | Out-Null
  New-Item -ItemType Directory -Force -Path (Join-Path $frames $scene) | Out-Null
  New-Item -ItemType Directory -Force -Path $masks | Out-Null
  New-Item -ItemType Directory -Force -Path $annotations | Out-Null

  $csv = @(
    "frame_index,frame_name,track_id,class_id,class_name,x1,y1,x2,y2,confidence,source",
    "1,000001.jpg,1,0,person,10,20,60,120,0.95,smoke",
    "2,000002.jpg,1,0,person,12,22,62,122,0.96,smoke"
  )
  Set-Content -LiteralPath (Join-Path (Join-Path $merge "csv") "$scene.csv") -Value $csv -Encoding UTF8

  return [pscustomobject]@{
    MergeRoot = $merge
    FrameRoot = $frames
    MaskRoot = $masks
    AnnotationRoot = $annotations
    Fixture = $true
  }
}
