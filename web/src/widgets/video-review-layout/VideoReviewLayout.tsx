import { useEffect, useMemo, useRef, type MouseEvent } from "react";
import { Badge } from "@shared/ui/Badge";
import { Button } from "@shared/ui/Button";
import { clamp } from "@shared/lib/format";
import { warmupTrackingMath } from "@shared/wasm/trackingMath";
import type { Segment } from "@entities/anomaly-event/model";
import type { Box, Track } from "@entities/track/model";
import type { ClassCount, VideoMeta } from "@entities/video/model";
import { classColor, className, displayClassName, trackKey } from "@entities/track/model";

type PlaybackMode = "frames" | "video";
type PlaybackRangeMode = "full" | "segment" | "track";

interface Props {
  scene: string;
  meta?: VideoMeta;
  frame: number;
  boxes: Box[];
  selectedTrackKey: string;
  selectedTrack?: Track;
  lockedSegment?: Segment;
  playRate: number;
  playbackMode: PlaybackMode;
  playbackRangeMode: PlaybackRangeMode;
  reviewFPS: number;
  playing: boolean;
  pendingDeletes: string[];
  onFrameChange: (frame: number) => void;
  onSelectTrack: (key: string) => void;
  onSegmentLock: (segment?: Segment) => void;
  onPlaybackRangeMode: (mode: PlaybackRangeMode) => void;
  onPlayRate: (rate: number) => void;
  onPlaybackMode: (mode: PlaybackMode) => void;
  onReviewFPS: (fps: number) => void;
  onPlaying: (playing: boolean) => void;
  onAdjacentVideo: (delta: number) => void;
}

function mediaSize(img: HTMLImageElement | null, video: HTMLVideoElement | null, mode: PlaybackMode) {
  if (mode === "video" && video && video.videoWidth && video.clientWidth) {
    return { el: video, naturalWidth: video.videoWidth, naturalHeight: video.videoHeight, width: video.clientWidth, height: video.clientHeight };
  }
  if (img && img.naturalWidth && img.clientWidth) {
    return { el: img, naturalWidth: img.naturalWidth, naturalHeight: img.naturalHeight, width: img.clientWidth, height: img.clientHeight };
  }
  return undefined;
}

export function VideoReviewLayout({
  scene,
  meta,
  frame,
  boxes,
  selectedTrackKey,
  selectedTrack,
  lockedSegment,
  playRate,
  playbackMode,
  playbackRangeMode,
  reviewFPS,
  playing,
  pendingDeletes,
  onFrameChange,
  onSelectTrack,
  onSegmentLock,
  onPlaybackRangeMode,
  onPlayRate,
  onPlaybackMode,
  onReviewFPS,
  onPlaying,
  onAdjacentVideo
}: Props) {
  const imgRef = useRef<HTMLImageElement | null>(null);
  const videoRef = useRef<HTMLVideoElement | null>(null);
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const safeFPS = Math.max(1, Math.min(120, reviewFPS || 30));
  const range = useMemo<[number, number]>(() => {
    if (playbackRangeMode === "track" && selectedTrack) return [selectedTrack.first_frame, selectedTrack.last_frame];
    if (playbackRangeMode === "segment" && lockedSegment) return [lockedSegment.start_frame, lockedSegment.end_frame];
    return [1, meta?.frame_count || 1];
  }, [lockedSegment, meta?.frame_count, playbackRangeMode, selectedTrack]);
  const safeFrame = clamp(frame, range[0], range[1]);
  const segmentOptions = meta?.anomaly_segments || [];

  useEffect(() => {
    warmupTrackingMath();
  }, []);

  useEffect(() => {
    if (safeFrame !== frame) onFrameChange(safeFrame);
  }, [frame, onFrameChange, safeFrame]);

  const draw = () => {
    const canvas = canvasRef.current;
    const size = mediaSize(imgRef.current, videoRef.current, playbackMode);
    if (!canvas || !size) return;
    canvas.width = size.width;
    canvas.height = size.height;
    const ctx = canvas.getContext("2d");
    if (!ctx) return;
    ctx.clearRect(0, 0, canvas.width, canvas.height);
    const sx = canvas.width / size.naturalWidth;
    const sy = canvas.height / size.naturalHeight;
    for (const box of boxes) {
      const key = trackKey(box);
      if (playbackRangeMode === "track" && key !== selectedTrackKey) continue;
      if (pendingDeletes.includes(key)) continue;
      const color = box.color || classColor(box.class_id);
      const x = box.x * sx;
      const y = box.y * sy;
      const w = box.w * sx;
      const h = box.h * sy;
      ctx.lineWidth = selectedTrackKey === key ? 4 : 2;
      ctx.strokeStyle = color;
      ctx.strokeRect(x, y, w, h);
      const label = `编号:${box.track_id}`;
      const labelY = Math.max(15, y - 5);
      ctx.font = "700 13px Microsoft YaHei, Segoe UI, sans-serif";
      ctx.lineWidth = 4;
      ctx.strokeStyle = "rgba(0,0,0,.9)";
      ctx.strokeText(label, x, labelY);
      ctx.fillStyle = color;
      ctx.fillText(label, x, labelY);
    }
  };

  useEffect(() => {
    if (!playing || playbackMode !== "frames") return;
    const timer = window.setInterval(() => {
      onFrameChange(safeFrame >= range[1] ? range[0] : safeFrame + 1);
    }, Math.max(16, 1000 / safeFPS / playRate));
    return () => window.clearInterval(timer);
  }, [onFrameChange, playRate, playbackMode, playing, range, safeFPS, safeFrame]);

  useEffect(() => {
    if (!scene || playbackMode !== "frames") return;
    const lookAhead = 14;
    for (let offset = 1; offset <= lookAhead; offset += 1) {
      const nextFrame = safeFrame + offset;
      if (nextFrame > range[1]) break;
      const image = new Image();
      image.decoding = "async";
      image.src = `/api/video/${scene}/frame/${nextFrame}.jpg`;
    }
  }, [playbackMode, range, scene, safeFrame]);

  useEffect(() => {
    const video = videoRef.current;
    if (!video || playbackMode !== "video") return;
    video.playbackRate = playRate;
    if (playing) {
      void video.play().catch(() => onPlaying(false));
    } else {
      video.pause();
    }
  }, [onPlaying, playRate, playbackMode, playing]);

  useEffect(() => {
    const video = videoRef.current;
    if (!video || playbackMode !== "video" || playing) return;
    const target = (safeFrame - 1) / safeFPS;
    if (Number.isFinite(target) && Math.abs(video.currentTime - target) > 0.08) {
      video.currentTime = target;
    }
  }, [playbackMode, playing, safeFPS, safeFrame]);

  useEffect(() => {
    draw();
    window.addEventListener("resize", draw);
    return () => window.removeEventListener("resize", draw);
  });

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.target instanceof HTMLInputElement || event.target instanceof HTMLTextAreaElement || event.target instanceof HTMLSelectElement) return;
      if (event.key.toLowerCase() === "a") onAdjacentVideo(-1);
      if (event.key.toLowerCase() === "d") onAdjacentVideo(1);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onAdjacentVideo]);

  const pick = (event: MouseEvent<HTMLCanvasElement>) => {
    const size = mediaSize(imgRef.current, videoRef.current, playbackMode);
    const canvas = canvasRef.current;
    if (!size || !canvas) return;
    const rect = canvas.getBoundingClientRect();
    const x = event.clientX - rect.left;
    const y = event.clientY - rect.top;
    const sx = canvas.width / size.naturalWidth;
    const sy = canvas.height / size.naturalHeight;
    let hit: Box | undefined;
    for (const box of boxes) {
      const bx = box.x * sx;
      const by = box.y * sy;
      const bw = box.w * sx;
      const bh = box.h * sy;
      if (x >= bx && x <= bx + bw && y >= by && y <= by + bh) hit = box;
    }
    if (hit) onSelectTrack(trackKey(hit));
  };

  const onVideoTime = () => {
    const video = videoRef.current;
    if (!video || playbackMode !== "video") return;
    const nextFrame = clamp(Math.round(video.currentTime * safeFPS) + 1, range[0], range[1]);
    if (nextFrame >= range[1] && video.currentTime >= range[1] / safeFPS) {
      video.currentTime = (range[0] - 1) / safeFPS;
      onFrameChange(range[0]);
      return;
    }
    if (nextFrame < range[0]) {
      video.currentTime = (range[0] - 1) / safeFPS;
      return;
    }
    if (nextFrame !== safeFrame) onFrameChange(nextFrame);
    draw();
  };

  return (
    <section className="viewerCard">
      <div className="sampleHeader">
        <div>
          <h2>{scene || "未选择视频"}</h2>
          <p>
            {meta?.frame_count || 0} 帧 · {meta?.tracks?.length || 0} 条轨迹 · {meta?.rows || 0} 个框 · 异常帧 {meta?.anomaly_frame_count || 0}
          </p>
        </div>
        <Button onClick={() => onSegmentLock(undefined)}>整段视频</Button>
      </div>
      <div className="viewerSurface">
        {scene ? (
          <div className="mediaLayer">
            {playbackMode === "video" ? (
              <video
                ref={videoRef}
                src={`/api/video/${scene}/preview`}
                muted
                playsInline
                preload="auto"
                onLoadedMetadata={() => {
                  const video = videoRef.current;
                  if (video) video.currentTime = (safeFrame - 1) / safeFPS;
                  draw();
                }}
                onTimeUpdate={onVideoTime}
                onSeeked={draw}
              />
            ) : (
              <img ref={imgRef} src={`/api/video/${scene}/frame/${safeFrame}.jpg`} onLoad={draw} alt={`${scene} frame ${safeFrame}`} />
            )}
            <canvas ref={canvasRef} onClick={pick} />
          </div>
        ) : (
          <div className="empty">请选择视频</div>
        )}
      </div>
      <div className="transport">
        <Button onClick={() => onFrameChange(safeFrame - 1)}>←</Button>
        <Button variant="primary" onClick={() => onPlaying(!playing)}>
          {playing ? "暂停" : "播放"}
        </Button>
        <Button onClick={() => onFrameChange(safeFrame + 1)}>→</Button>
        <input type="range" min={range[0]} max={range[1]} value={safeFrame} onChange={(event) => onFrameChange(Number(event.target.value))} />
        <input className="frameInput" type="number" min={range[0]} max={range[1]} value={safeFrame} onChange={(event) => onFrameChange(Number(event.target.value))} />
        <span>/{range[1]}</span>
      </div>
      <div className="reviewControls">
        <label>
          播放范围
          <select
            value={playbackRangeMode === "track" ? "track" : lockedSegment?.index ?? "full"}
            onChange={(event) => {
              const value = event.target.value;
              if (value === "track") {
                if (!selectedTrack) return;
                onPlaybackRangeMode("track");
                onFrameChange(selectedTrack.first_frame);
              } else if (value === "full") {
                onPlaybackRangeMode("full");
                onSegmentLock(undefined);
              } else {
                const segment = segmentOptions.find((item) => String(item.index) === value);
                onPlaybackRangeMode("segment");
                onSegmentLock(segment);
              }
            }}
          >
            <option value="full">整段视频</option>
            <option value="track" disabled={!selectedTrack}>
              {selectedTrack ? `current track ${selectedTrack.first_frame}-${selectedTrack.last_frame}` : "current track"}
            </option>
            {segmentOptions.map((segment) => (
              <option key={segment.index} value={segment.index}>
                异常片段 {segment.index}（{segment.start_frame}-{segment.end_frame}）
              </option>
            ))}
          </select>
        </label>
        {selectedTrack ? (
          <Button
            onClick={() => {
              onPlaybackRangeMode("track");
              onFrameChange(selectedTrack.first_frame);
            }}
          >
            Play track
          </Button>
        ) : null}
        <label>
          播放方式
          <select value={playbackMode} onChange={(event) => onPlaybackMode(event.target.value as PlaybackMode)}>
            <option value="frames">逐帧审核（预取）</option>
            <option value="video">视频预览（流畅）</option>
          </select>
        </label>
        <label>
          FPS
          <input type="number" min={1} max={120} value={safeFPS} onChange={(event) => onReviewFPS(Number(event.target.value) || 30)} />
        </label>
        <label>
          播放速度
          <select value={playRate} onChange={(event) => onPlayRate(Number(event.target.value))}>
            {[0.5, 1, 1.5, 2, 3, 4].map((rate) => (
              <option key={rate} value={rate}>
                {rate.toFixed(1)}x
              </option>
            ))}
          </select>
        </label>
      </div>
      <div className="badgeRow">
        {(meta?.classes || []).map((item: ClassCount) => (
          <Badge key={item.class_id} color={item.color}>
            {displayClassName(item)} {item.count}
          </Badge>
        ))}
      </div>
      <div className="segmentRail">
        <span>帧级异常片段 {segmentOptions.length} 段，点击可锁定播放</span>
        {segmentOptions.map((segment) => (
          <button key={segment.index} className={lockedSegment?.index === segment.index ? "segment active" : "segment"} onClick={() => onSegmentLock(segment)}>
            #{segment.index} {segment.start_frame}-{segment.end_frame}（{segment.length} 帧）
          </button>
        ))}
      </div>
    </section>
  );
}
