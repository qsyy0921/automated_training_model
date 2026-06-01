# Rust/WASM Acceleration Layer

The frontend keeps React/TypeScript as the product UI and uses Rust/WASM only for deterministic hot-path math.

Current module:

```text
crates/tracking-math/
  src/lib.rs

web/src/shared/wasm/
  trackingMath.ts
  pkg/tracking_math.wasm
```

Build:

```powershell
cd F:\automated_training_model\web
npm run wasm:build
npm run build
```

The first WASM functions are deliberately small:

- `iou_xywh`: bbox IoU in `[x,y,w,h]`.
- `segment_intersection_start/end`: track range intersected with an anomaly segment.
- `segment_intersects`: cheap segment overlap check.

Design rules:

1. UI, state, forms, API calls, and workflow orchestration stay in TypeScript.
2. WASM is loaded as an optional acceleration layer with TypeScript fallback.
3. Do not move business logic into Rust unless it is CPU-bound, deterministic, and measurable.
4. Future candidates: polygon/mask operations, RLE encode/decode, batch IoU, NMS, dense timeline aggregation.
