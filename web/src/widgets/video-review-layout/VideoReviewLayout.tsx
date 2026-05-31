import { useEffect, useMemo, useRef } from "react";
import { Badge } from "@shared/ui/Badge";
import { Button } from "@shared/ui/Button";
import { clamp } from "@shared/lib/format";
import type { Segment } from "@entities/anomaly-event/model";
import type { Box } from "@entities/track/model";
import type { ClassCount, VideoMeta } from "@entities/video/model";
import { classColor, className, trackKey } from "@entities/track/model";

interface Props {
  scene: string;
  meta?: VideoMeta;
  frame: number;
  boxes: Box[];
  selectedTrackKey: string;
  lockedSegment?: Segment;
  playRate: number;
  playing: boolean;
  pendingDeletes: string[];
  onFrameChange: (frame: number) => void;
  onSelectTrack: (key: string) => void;
  onSegmentLock: (segment?: Segment) => void;
  onPlayRate: (rate: number) => void;
  onPlaying: (playing: boolean) => void;
  onAdjacentVideo: (delta: number) => void;
}

export function VideoReviewLayout({
  scene,
  meta,
  frame,
  boxes,
  selectedTrackKey,
  lockedSegment,
  playRate,
  playing,
  pendingDeletes,
  onFrameChange,
  onSelectTrack,
  onSegmentLock,
  onPlayRate,
  onPlaying,
  onAdjacentVideo
}: Props) {
  const imgRef = useRef<HTMLImageElement | null>(null);
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const range = useMemo<[number, number]>(() => {
    if (lockedSegment) return [lockedSegment.start_frame, lockedSegment.end_frame];
    return [1, meta?.frame_count || 1];
  }, [lockedSegment, meta]);

  useEffect(() => {
    if (!playing) return;
    const timer = window.setInterval(() => {
      onFrameChange(frame >= range[1] ? range[0] : frame + 1);
    }, Math.max(60, 480 / playRate));
    return () => window.clearInterval(timer);
  }, [frame, onFrameChange, playRate, playing, range]);

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.target instanceof HTMLInputElement || event.target instanceof HTMLTextAreaElement || event.target instanceof HTMLSelectElement) return;
      if (event.key.toLowerCase() === "a") onAdjacentVideo(-1);
      if (event.key.toLowerCase() === "d") onAdjacentVideo(1);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onAdjacentVideo]);

  const draw = () => {
    const img = imgRef.current;
    const canvas = canvasRef.current;
    if (!img || !canvas || !img.naturalWidth || !img.clientWidth) return;
    canvas.width = img.clientWidth;
    canvas.height = img.clientHeight;
    const ctx = canvas.getContext("2d");
    if (!ctx) return;
    ctx.clearRect(0, 0, canvas.width, canvas.height);
    const sx = canvas.width / img.naturalWidth;
    const sy = canvas.height / img.naturalHeight;
    for (const box of boxes) {
      const key = trackKey(box);
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
      ctx.font = "700 13px Microsoft YaHei, Segoe UI";
      ctx.lineWidth = 4;
      ctx.strokeStyle = "rgba(0,0,0,.9)";
      ctx.strokeText(label, x, labelY);
      ctx.fillStyle = color;
      ctx.fillText(label, x, labelY);
    }
  };

  useEffect(() => {
    draw();
    window.addEventListener("resize", draw);
    return () => window.removeEventListener("resize", draw);
  });

  const pick = (event: React.MouseEvent<HTMLCanvasElement>) => {
    const img = imgRef.current;
    const canvas = canvasRef.current;
    if (!img || !canvas) return;
    const rect = canvas.getBoundingClientRect();
    const x = event.clientX - rect.left;
    const y = event.clientY - rect.top;
    const sx = canvas.width / img.naturalWidth;
    const sy = canvas.height / img.naturalHeight;
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

  const safeFrame = clamp(frame, range[0], range[1]);
  const segmentOptions = meta?.anomaly_segments || [];

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
          <>
            <img ref={imgRef} src={`/api/video/${scene}/frame/${safeFrame}.jpg?ts=${Date.now()}`} onLoad={draw} alt={`${scene} frame ${safeFrame}`} />
            <canvas ref={canvasRef} onClick={pick} />
          </>
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
            value={lockedSegment?.index ?? "full"}
            onChange={(event) => {
              const value = event.target.value;
              if (value === "full") onSegmentLock(undefined);
              else onSegmentLock(segmentOptions.find((item) => String(item.index) === value));
            }}
          >
            <option value="full">整段视频</option>
            {segmentOptions.map((segment) => (
              <option key={segment.index} value={segment.index}>
                异常片段 {segment.index}（{segment.start_frame}-{segment.end_frame}）
              </option>
            ))}
          </select>
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
            {className(item.class_id)} {item.count}
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

