export interface Segment {
  index: number;
  start_frame: number;
  end_frame: number;
  length: number;
}

export interface ObjectSlot {
  slot: number;
  empty: boolean;
  track_key?: string;
  track_id?: number;
  class_id?: number;
  object_class?: string;
  upper_color?: string;
  upper_clothing?: string;
  lower_color?: string;
  lower_clothing?: string;
  carrying?: string;
  appearance?: string;
}

