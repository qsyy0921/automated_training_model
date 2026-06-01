export type ClassID = 0 | 1 | 2 | 3 | 5 | 7 | 36 | 80 | number;

export interface Track {
  track_key: string;
  track_id: number;
  class_id: ClassID;
  class_name: string;
  color: string;
  first_frame: number;
  last_frame: number;
  frames: number;
  mean_conf: number;
  avg_conf: number;
  mean_area: number;
  avg_area: number;
  max_area: number;
  review_status?: string;
}

export interface Box {
  frame: number;
  frame_name: string;
  track_id: number;
  class_id: ClassID;
  class_name: string;
  track_key: string;
  confidence: number;
  x: number;
  y: number;
  w: number;
  h: number;
  x2: number;
  y2: number;
  color: string;
  source: string;
}

export function trackKey(input: Pick<Track | Box, "class_id" | "track_id">): string {
  return `${input.class_id}:${input.track_id}`;
}

export function trackDisplayName(input: Pick<Track | Box, "class_id" | "track_id">): string {
  return `${className(input.class_id)} 编号:${input.track_id}`;
}

export function className(classID: ClassID): string {
  const names: Record<number, string> = {
    0: "人",
    1: "自行车",
    2: "汽车",
    3: "摩托车",
    5: "公交车",
    7: "卡车",
    36: "滑板",
    80: "婴儿车"
  };
  return names[Number(classID)] ?? `类别 ${classID}`;
}

export function displayClassName(input: Pick<Track | Box, "class_id"> & { class_name?: string }): string {
  return input.class_name || className(input.class_id);
}

export function classColor(classID: ClassID): string {
  const colors: Record<number, string> = {
    0: "#00B8D9",
    1: "#F5C400",
    2: "#FF4D6D",
    3: "#8B5CF6",
    5: "#2F6FED",
    7: "#21B573",
    36: "#D946EF",
    80: "#F97316"
  };
  return colors[Number(classID)] ?? "#7A8CA8";
}
