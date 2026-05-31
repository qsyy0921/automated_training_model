import type { ObjectSlot, Segment } from "@entities/anomaly-event/model";
import type { Track } from "@entities/track/model";

export function createEmptyObjectSlots(count = 5): ObjectSlot[] {
  return Array.from({ length: count }, (_, index) => ({
    slot: index + 1,
    empty: true
  }));
}

export function normalizeObjectSlots(slots: ObjectSlot[]): ObjectSlot[] {
  const next = slots.map((slot, index) => ({ ...slot, slot: index + 1 }));
  return next.length ? next : createEmptyObjectSlots(1);
}

export function compactObjectSlot(slot: ObjectSlot): ObjectSlot {
  const unknownValues = new Set(["", "unknown", "未填写"]);
  return Object.fromEntries(
    Object.entries(slot).filter(([, value]) => !unknownValues.has(String(value ?? "")))
  ) as ObjectSlot;
}

export function intersectTrackWithSegment(track: Track, segment: Segment): { start: number; end: number } | null {
  const start = Math.max(track.first_frame, segment.start_frame);
  const end = Math.min(track.last_frame, segment.end_frame);
  if (start > end) return null;
  return { start, end };
}

