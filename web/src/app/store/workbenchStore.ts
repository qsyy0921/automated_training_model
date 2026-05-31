import { create } from "zustand";
import type { ObjectSlot, Segment } from "@entities/anomaly-event/model";
import type { Box, Track } from "@entities/track/model";
import type { VideoMeta, VideoSummary } from "@entities/video/model";

export interface WorkbenchState {
  videos: VideoSummary[];
  currentScene: string;
  currentFrame: number;
  meta?: VideoMeta;
  tracks: Track[];
  boxes: Box[];
  selectedTrackKey: string;
  pendingDeleteKeys: string[];
  lockedSegment?: Segment;
  objectSlots: ObjectSlot[];
  activeObjectSlot: number;
  playing: boolean;
  playRate: number;
  playbackMode: "frames" | "video";
  reviewFPS: number;
  trackListCollapsed: boolean;
  searchText: string;
  classFilter: string;
  statusText: string;
  setState: (patch: Partial<WorkbenchState>) => void;
  resetDrafts: () => void;
}

export const useWorkbenchStore = create<WorkbenchState>((set) => ({
  videos: [],
  currentScene: "",
  currentFrame: 1,
  tracks: [],
  boxes: [],
  selectedTrackKey: "",
  pendingDeleteKeys: [],
  objectSlots: Array.from({ length: 5 }, (_, index) => ({ slot: index + 1, empty: true })),
  activeObjectSlot: 0,
  playing: false,
  playRate: 1,
  playbackMode: "frames",
  reviewFPS: 30,
  trackListCollapsed: false,
  searchText: "",
  classFilter: "",
  statusText: "就绪",
  setState: (patch) => set(patch),
  resetDrafts: () =>
    set({
      selectedTrackKey: "",
      pendingDeleteKeys: [],
      objectSlots: Array.from({ length: 5 }, (_, index) => ({ slot: index + 1, empty: true })),
      activeObjectSlot: 0
    })
}));
