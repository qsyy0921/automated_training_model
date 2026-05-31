import { className, renderClassOptions } from "../../shared/catalog.js";
import { qs } from "../../shared/dom.js";
import { trackColor, trackKey, trackLabel } from "../../entities/tracking.js";

export class TrackingReviewFeature {
  constructor(app) {
    this.app = app;
  }

  bind() {
    qs("trackClass").innerHTML = renderClassOptions();
    qs("trackSearch").addEventListener("input", () => this.renderTracks());
    qs("trackClass").addEventListener("change", () => this.renderTracks());
    qs("toggleTracks").addEventListener("click", () => this.toggleTracks());
    qs("addReject").addEventListener("click", () => this.addReject());
    qs("clearPending").addEventListener("click", () => this.clearPending());
    qs("savePending").addEventListener("click", () => this.savePending());
    qs("purgePending").addEventListener("click", () => this.purgePending());
  }

  renderTracks() {
    const { tracks, selectedTrackKey, pendingDeletes, tracksCollapsed } = this.app.store.state;
    const root = qs("tracks");
    root.style.display = tracksCollapsed ? "none" : "grid";
    const search = qs("trackSearch").value.toLowerCase();
    const classFilter = qs("trackClass").value;
    root.innerHTML = "";
    tracks
      .filter((track) => !classFilter || String(track.class_id) === classFilter)
      .filter((track) => `${track.track_id} ${className(track.class_id)}`.toLowerCase().includes(search))
      .forEach((track) => {
        const key = trackKey(track);
        const button = document.createElement("button");
        button.className = `track ${selectedTrackKey === key ? "active" : ""} ${pendingDeletes[key] ? "pending" : ""}`;
        button.style.setProperty("--c", trackColor(track));
        const meanConf = track.avg_conf ?? track.mean_conf ?? 0;
        button.innerHTML = `<b>${trackLabel(track)}</b><small>${track.first_frame}-${track.last_frame} · ${track.frames} 次出现 · 平均置信度 ${meanConf.toFixed(2)}</small>`;
        button.addEventListener("click", () => this.app.selectTrack(key, true));
        root.appendChild(button);
      });
  }

  renderPending() {
    const root = qs("pending");
    const rows = Object.entries(this.app.store.state.pendingDeletes);
    root.innerHTML = rows.length ? "" : '<div class="empty">暂无待删除轨迹。</div>';
    for (const [key, row] of rows) {
      const card = document.createElement("div");
      card.className = "card";
      card.style.setProperty("--c", trackColor(row));
      card.innerHTML = `<b>${trackLabel(row)}</b><small>${key} · ${row.start_frame}-${row.end_frame}</small><button>撤销</button>`;
      card.querySelector("button").addEventListener("click", () => {
        delete this.app.store.state.pendingDeletes[key];
        this.renderPending();
        this.renderTracks();
        this.app.viewer.drawBoxes();
      });
      root.appendChild(card);
    }
  }

  addReject() {
    const track = this.app.selectedTrack();
    if (!track) {
      alert("先选择对象");
      return;
    }
    this.app.store.state.pendingDeletes[trackKey(track)] = {
      track_key: trackKey(track),
      track_id: track.track_id,
      class_id: track.class_id,
      object_class: className(track.class_id),
      start_frame: track.first_frame,
      end_frame: track.last_frame,
      label: "正常",
      anomaly_type: "无",
      tracking_status: "删除",
      tracking_issue: "误检",
      bbox_quality: "ok",
    };
    this.renderPending();
    this.renderTracks();
    this.app.viewer.drawBoxes();
  }

  clearPending() {
    this.app.store.patch({ pendingDeletes: {} });
    this.renderPending();
    this.renderTracks();
    this.app.viewer.drawBoxes();
  }

  async savePending() {
    const rows = Object.values(this.app.store.state.pendingDeletes);
    if (!rows.length) {
      alert("没有待删除项");
      return;
    }
    for (const row of rows) {
      await this.app.api.saveAnnotation(this.app.store.state.scene, row);
    }
    this.app.store.patch({ pendingDeletes: {} });
    await this.app.refresh();
  }

  async purgePending() {
    const keys = Object.keys(this.app.store.state.pendingDeletes);
    if (!keys.length) {
      alert("没有待彻底删除的数据");
      return;
    }
    if (!confirm("彻底删除源 tracking 数据？系统会先备份 CSV。")) return;
    await this.savePending();
    await this.app.api.purgeTracks(this.app.store.state.scene, keys);
    await this.app.refresh();
  }

  toggleTracks() {
    const next = !this.app.store.state.tracksCollapsed;
    this.app.store.patch({ tracksCollapsed: next });
    qs("toggleTracks").textContent = next ? "展开" : "收起";
    this.renderTracks();
  }
}

