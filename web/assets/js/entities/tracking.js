import { classColor, className } from "../shared/catalog.js";

export function trackKey(item) {
  return item.track_key || `${item.class_id}:${item.track_id}`;
}

export function annotationTrackKey(item) {
  return item.track_key || `${item.class_id}:${item.track_id}`;
}

export function trackLabel(item) {
  return `${className(item.class_id)} 编号:${item.track_id}`;
}

export function trackColor(item) {
  return item.color || classColor(item.class_id);
}

