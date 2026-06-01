type TrackingMathExports = WebAssembly.Exports & {
  iou_xywh: (ax: number, ay: number, aw: number, ah: number, bx: number, by: number, bw: number, bh: number) => number;
  segment_intersection_start: (trackStart: number, trackEnd: number, segmentStart: number, segmentEnd: number) => number;
  segment_intersection_end: (trackStart: number, trackEnd: number, segmentStart: number, segmentEnd: number) => number;
  segment_intersects: (trackStart: number, trackEnd: number, segmentStart: number, segmentEnd: number) => number;
};

let exportsPromise: Promise<TrackingMathExports | undefined> | undefined;

function fallbackIoU(ax: number, ay: number, aw: number, ah: number, bx: number, by: number, bw: number, bh: number): number {
  if (aw <= 0 || ah <= 0 || bw <= 0 || bh <= 0) return 0;
  const ix1 = Math.max(ax, bx);
  const iy1 = Math.max(ay, by);
  const ix2 = Math.min(ax + aw, bx + bw);
  const iy2 = Math.min(ay + ah, by + bh);
  const inter = Math.max(0, ix2 - ix1) * Math.max(0, iy2 - iy1);
  if (inter <= 0) return 0;
  const union = aw * ah + bw * bh - inter;
  return union > 0 ? inter / union : 0;
}

async function loadTrackingMath(): Promise<TrackingMathExports | undefined> {
  if (!exportsPromise) {
    exportsPromise = (async () => {
      try {
        const url = new URL("./pkg/tracking_math.wasm", import.meta.url);
        const response = await fetch(url);
        const bytes = await response.arrayBuffer();
        const instance = await WebAssembly.instantiate(bytes, {});
        return instance.instance.exports as TrackingMathExports;
      } catch (error) {
        console.warn("tracking-math wasm unavailable, using TypeScript fallback", error);
        return undefined;
      }
    })();
  }
  return exportsPromise;
}

export async function iouXYWH(ax: number, ay: number, aw: number, ah: number, bx: number, by: number, bw: number, bh: number): Promise<number> {
  const wasm = await loadTrackingMath();
  return wasm?.iou_xywh(ax, ay, aw, ah, bx, by, bw, bh) ?? fallbackIoU(ax, ay, aw, ah, bx, by, bw, bh);
}

export function iouXYWHSync(ax: number, ay: number, aw: number, ah: number, bx: number, by: number, bw: number, bh: number): number {
  return fallbackIoU(ax, ay, aw, ah, bx, by, bw, bh);
}

export async function segmentIntersection(
  trackStart: number,
  trackEnd: number,
  segmentStart: number,
  segmentEnd: number
): Promise<{ start: number; end: number } | undefined> {
  const wasm = await loadTrackingMath();
  if (wasm) {
    const start = wasm.segment_intersection_start(trackStart, trackEnd, segmentStart, segmentEnd);
    const end = wasm.segment_intersection_end(trackStart, trackEnd, segmentStart, segmentEnd);
    return start > 0 && end >= start ? { start, end } : undefined;
  }
  const start = Math.max(trackStart, segmentStart);
  const end = Math.min(trackEnd, segmentEnd);
  return end >= start ? { start, end } : undefined;
}

export function warmupTrackingMath(): void {
  void loadTrackingMath();
}
