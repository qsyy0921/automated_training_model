import { api } from "../infrastructure/apiClient.js";
import { createStore } from "../state/store.js";
import { qs } from "../shared/dom.js";
import { className } from "../shared/catalog.js";
import { trackKey } from "../entities/tracking.js";
import { VideoListFeature } from "../features/video-list/videoList.js";
import { DatasetPanelFeature } from "../features/datasets/datasetPanel.js";
import { LifecyclePanelFeature } from "../features/lifecycle/lifecyclePanel.js";
import { FrameViewerFeature } from "../features/media-viewer/frameViewer.js";
import { TrackingReviewFeature } from "../features/tracking-review/trackingReview.js";
import { AnomalyAnnotationFeature } from "../features/anomaly-annotation/anomalyAnnotation.js";

export class WorkbenchApp {
  constructor() {
    this.api = api;
    this.store = createStore();
    this.videoList = new VideoListFeature(this);
    this.datasets = new DatasetPanelFeature(this);
    this.lifecycle = new LifecyclePanelFeature(this);
    this.viewer = new FrameViewerFeature(this);
    this.tracking = new TrackingReviewFeature(this);
    this.annotation = new AnomalyAnnotationFeature(this);
  }

  async init() {
    this.videoList.bind();
    this.datasets.bind();
    this.lifecycle.bind();
    this.viewer.bind();
    this.tracking.bind();
    this.annotation.bind();
    await this.loadVideos();
    await this.datasets.load();
  }

  setStatus(text) {
    qs("status").textContent = text;
  }

  hasUnsavedChanges() {
    return Object.keys(this.store.state.pendingDeletes).length > 0 || this.store.state.objectSlots.some((slot) => !slot.empty);
  }

  async loadVideos() {
    const data = await this.api.listVideos();
    this.store.patch({ videos: data.videos || [] });
    this.videoList.render();
    if (!this.store.state.scene && this.store.state.videos[0]) {
      await this.loadScene(this.store.state.videos[0].scene, true);
    }
  }

  async loadScene(scene, force = false) {
    if (!force && this.hasUnsavedChanges()) {
      alert("当前视频有未保存内容，请先 Ctrl+S 保存。");
      return;
    }
    this.setStatus("加载中");
    this.store.patch({
      scene,
      frame: 1,
      selectedTrackKey: "",
      pendingDeletes: {},
      lockedSegment: null,
      objectSlots: [],
      activeSlot: 0,
    });
    const meta = await this.api.videoMeta(scene);
    this.store.patch({
      meta,
      tracks: meta.tracks || [],
      annotations: meta.annotations || [],
    });
    qs("title").textContent = scene;
    qs("meta").textContent = `${meta.frame_count} 帧 · ${this.store.state.tracks.length} 条轨迹 · ${meta.rows} 个框 · 异常帧 ${meta.anomaly_frame_count || 0}`;
    this.viewer.resetForMeta();
    this.annotation.renderSegmentChecks();
    this.annotation.ensureSlots();
    this.tracking.renderPending();
    this.tracking.renderTracks();
    this.annotation.renderAnnotations();
    this.annotation.renderEventTree();
    this.videoList.render();
    await this.loadFrame(1);
    this.setStatus("就绪");
  }

  async loadFrame(frame) {
    if (!this.store.state.meta) return;
    const safeFrame = this.viewer.normalizeFrame(frame);
    this.store.patch({ frame: safeFrame });
    this.viewer.setFrameUI(safeFrame);
    const data = await this.api.frameBoxes(this.store.state.scene, safeFrame);
    this.store.patch({ boxes: data.boxes || [] });
    this.viewer.setImage(this.store.state.scene, safeFrame);
  }

  async refresh() {
    const scene = this.store.state.scene;
    const meta = await this.api.videoMeta(scene);
    this.store.patch({
      meta,
      tracks: meta.tracks || [],
      annotations: meta.annotations || [],
    });
    this.viewer.resetForMeta();
    this.annotation.renderSegmentChecks();
    this.tracking.renderPending();
    this.tracking.renderTracks();
    this.annotation.renderAnnotations();
    this.annotation.renderEventTree();
    this.videoList.render();
    await this.loadFrame(this.store.state.frame);
    this.setStatus("已刷新");
  }

  selectedTrack() {
    return this.store.state.tracks.find((item) => trackKey(item) === this.store.state.selectedTrackKey);
  }

  selectTrack(key, jumpToTrack) {
    this.store.patch({ selectedTrackKey: key });
    const track = this.selectedTrack();
    qs("selectedText").textContent = track ? `${className(track.class_id)} 编号:${track.track_id} · ${track.first_frame}-${track.last_frame}` : "未选择";
    this.tracking.renderTracks();
    this.viewer.drawBoxes();
    if (track && jumpToTrack) {
      const [start, end] = this.viewer.range();
      this.loadFrame(Math.max(start, Math.min(end, track.first_frame)));
    }
  }

  loadAdjacentVideo(delta) {
    const index = this.store.state.videos.findIndex((item) => item.scene === this.store.state.scene);
    if (index < 0) return;
    const next = this.store.state.videos[Math.max(0, Math.min(this.store.state.videos.length - 1, index + delta))];
    if (next) this.loadScene(next.scene);
  }
}
