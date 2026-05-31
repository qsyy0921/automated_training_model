import type { Segment } from "@entities/anomaly-event/model";
import type { Track } from "@entities/track/model";

export interface ClassCount {
  class_id: number;
  class_name: string;
  color: string;
  count: number;
}

export interface VideoSummary {
  scene: string;
  frame_count: number;
  rows: number;
  track_count: number;
  annotation_count: number;
  classes: ClassCount[];
  has_preview: boolean;
  anomaly_segments: Segment[];
  anomaly_frame_count: number;
}

export interface VideoMeta {
  scene: string;
  frame_count: number;
  rows: number;
  tracks: Track[];
  classes: ClassCount[];
  anomaly_frame_count: number;
  anomaly_segments: Segment[];
  annotations: AnnotationRecord[];
}

export interface AnnotationRecord {
  id: string;
  scene: string;
  track_key: string;
  track_id: number;
  class_id: number;
  object_class: string;
  start_frame: number;
  end_frame: number;
  label: string;
  anomaly_type: string;
  tracking_status: string;
  tracking_issue: string;
  bbox_quality: string;
  event_id: string;
  event_title: string;
  event_reason: string;
  severity: string;
  upper_color?: string;
  lower_color?: string;
  upper_clothing?: string;
  lower_clothing?: string;
  carrying?: string;
  appearance?: string;
  related_track_ids: string;
  notes: string;
  created_at: string;
  updated_at: string;
}

