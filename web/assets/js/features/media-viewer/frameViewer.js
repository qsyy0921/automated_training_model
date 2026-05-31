import { classColor, className } from "../../shared/catalog.js";
import { qs, qsa } from "../../shared/dom.js";
import { trackColor, trackKey } from "../../entities/tracking.js";

export class FrameViewerFeature {
  constructor(app) {
    this.app = app;
  }

  bind() {
    qs("prev").addEventListener("click", () => this.app.loadFrame(this.app.store.state.frame - 1));
    qs("next").addEventListener("click", () => this.app.loadFrame(this.app.store.state.frame + 1));
    qs("slider").addEventListener("input", (event) => this.app.loadFrame(event.target.value));
    qs("frameInput").addEventListener("change", (event) => this.app.loadFrame(event.target.value));
    qs("playMode").addEventListener("change", (event) => this.setPlayMode(event.target.value));
    qs("play").addEventListener("click", () => this.togglePlayback());
    qs("wholeVideo").addEventListener("click", () => this.setPlayMode("full"));
    qs("overlay").addEventListener("click", (event) => this.pickBox(event));
    qs("viewer").addEventListener("keydown", (event) => this.handleKeyboard(event));
    window.addEventListener("resize", () => this.drawBoxes());
  }

  resetForMeta() {
    const { meta } = this.app.store.state;
    qs("slider").max = meta.frame_count;
    qs("frameInput").max = meta.frame_count;
    qs("frameCount").textContent = `/${meta.frame_count}`;
    this.renderLegend();
    this.renderSegments();
  }

  range() {
    const { lockedSegment, meta } = this.app.store.state;
    return lockedSegment ? [lockedSegment.start_frame, lockedSegment.end_frame] : [1, meta?.frame_count || 1];
  }

  normalizeFrame(frame) {
    const [start, end] = this.range();
    return Math.max(start, Math.min(end, Number(frame) || start));
  }

  setPlayMode(value, jump = true) {
    const { meta } = this.app.store.state;
    if (value === "full") {
      this.app.store.patch({ lockedSegment: null });
      qs("playMode").value = "full";
      this.renderSegments();
      return;
    }
    const segment = (meta?.anomaly_segments || []).find((item) => String(item.index) === String(value));
    this.app.store.patch({ lockedSegment: segment || null });
    qs("playMode").value = segment ? String(segment.index) : "full";
    this.renderSegments();
    if (jump && segment) this.app.loadFrame(segment.start_frame);
  }

  renderLegend() {
    const { meta } = this.app.store.state;
    qs("legend").innerHTML = (meta.classes || []).map((item) => `<span class="badge" style="--c:${item.color || classColor(item.class_id)}">${className(item.class_id)} ${item.count}</span>`).join("");
  }

  renderSegments() {
    const { meta, lockedSegment } = this.app.store.state;
    const segments = meta?.anomaly_segments || [];
    qs("segments").innerHTML = segments.length
      ? segments.map((item) => `<button class="segment ${lockedSegment?.index === item.index ? "active" : ""}" data-seg="${item.index}">#${item.index} ${item.start_frame}-${item.end_frame} (${item.length} 帧)</button>`).join("")
      : '<span class="empty">无帧级异常片段</span>';
    qsa("[data-seg]", qs("segments")).forEach((button) => button.addEventListener("click", () => this.setPlayMode(button.dataset.seg)));
    qs("playMode").innerHTML = '<option value="full">整段视频</option>' + segments.map((item) => `<option value="${item.index}">异常片段 ${item.index} (${item.start_frame}-${item.end_frame})</option>`).join("");
    qs("playMode").value = lockedSegment ? String(lockedSegment.index) : "full";
  }

  setFrameUI(frame) {
    qs("slider").value = frame;
    qs("frameInput").value = frame;
  }

  setImage(scene, frame) {
    const image = qs("frameImg");
    image.onload = () => this.drawBoxes();
    image.src = `/api/video/${scene}/frame/${frame}.jpg?ts=${Date.now()}`;
  }

  drawBoxes() {
    const image = qs("frameImg");
    const canvas = qs("overlay");
    if (!image.naturalWidth || !image.clientWidth) return;
    canvas.width = image.clientWidth;
    canvas.height = image.clientHeight;
    const ctx = canvas.getContext("2d");
    ctx.clearRect(0, 0, canvas.width, canvas.height);
    const sx = canvas.width / image.naturalWidth;
    const sy = canvas.height / image.naturalHeight;
    const { boxes, pendingDeletes, selectedTrackKey } = this.app.store.state;
    for (const box of boxes) {
      const key = trackKey(box);
      if (pendingDeletes[key]) continue;
      const x = box.x * sx;
      const y = box.y * sy;
      const w = box.w * sx;
      const h = box.h * sy;
      const color = box.color || classColor(box.class_id);
      ctx.lineWidth = selectedTrackKey === key ? 4 : 2;
      ctx.strokeStyle = color;
      ctx.strokeRect(x, y, w, h);
      ctx.font = "bold 12px Segoe UI";
      ctx.lineWidth = 4;
      ctx.strokeStyle = "rgba(0,0,0,.88)";
      ctx.strokeText(`编号:${box.track_id}`, x, Math.max(13, y - 4));
      ctx.fillStyle = color;
      ctx.fillText(`编号:${box.track_id}`, x, Math.max(13, y - 4));
    }
  }

  pickBox(event) {
    const canvas = qs("overlay");
    const rect = canvas.getBoundingClientRect();
    const image = qs("frameImg");
    const x = event.clientX - rect.left;
    const y = event.clientY - rect.top;
    const sx = canvas.width / image.naturalWidth;
    const sy = canvas.height / image.naturalHeight;
    let hit = null;
    for (const box of this.app.store.state.boxes) {
      const bx = box.x * sx;
      const by = box.y * sy;
      const bw = box.w * sx;
      const bh = box.h * sy;
      if (x >= bx && x <= bx + bw && y >= by && y <= by + bh) hit = box;
    }
    if (hit) this.app.selectTrack(trackKey(hit), false);
  }

  togglePlayback() {
    const state = this.app.store.state;
    if (state.playing) {
      clearInterval(state.timer);
      this.app.store.patch({ playing: false, timer: null });
      qs("play").textContent = "▶";
      return;
    }
    const timer = setInterval(() => {
      const [, end] = this.range();
      const [start] = this.range();
      this.app.loadFrame(state.frame >= end ? start : state.frame + 1);
    }, Math.max(80, 500 / Number(qs("playRate").value || 1)));
    this.app.store.patch({ playing: true, timer });
    qs("play").textContent = "Ⅱ";
  }

  handleKeyboard(event) {
    if (event.ctrlKey && event.key.toLowerCase() === "s") {
      event.preventDefault();
      this.app.tracking.savePending();
      return;
    }
    if (event.key.toLowerCase() === "a") this.app.loadAdjacentVideo(-1);
    if (event.key.toLowerCase() === "d") this.app.loadAdjacentVideo(1);
  }
}

