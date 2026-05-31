import type { VideoSummary } from "@entities/video/model";

export function nextVideo(videos: VideoSummary[], currentID: string, direction: 1 | -1): VideoSummary | undefined {
  if (!videos.length) return undefined;
  const index = videos.findIndex((video) => video.scene === currentID);
  const safeIndex = index >= 0 ? index : 0;
  return videos[(safeIndex + direction + videos.length) % videos.length];
}
