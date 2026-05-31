import { className, renderClassOptions } from "../../shared/catalog.js";
import { qs } from "../../shared/dom.js";

export class VideoListFeature {
  constructor(app) {
    this.app = app;
  }

  bind() {
    qs("videoSearch").addEventListener("input", () => this.render());
    qs("videoClass").addEventListener("change", () => this.render());
    qs("videoClass").innerHTML = renderClassOptions();
  }

  render() {
    const { state } = this.app.store;
    const search = qs("videoSearch").value.toLowerCase();
    const classFilter = qs("videoClass").value;
    const root = qs("videoList");
    root.innerHTML = "";
    state.videos
      .filter((video) => video.scene.toLowerCase().includes(search))
      .filter((video) => !classFilter || (video.classes || []).some((item) => String(item.class_id) === classFilter))
      .forEach((video) => {
        const button = document.createElement("button");
        button.className = `videoItem ${state.scene === video.scene ? "active" : ""}`;
        button.innerHTML = `
          <div class="videoName">${video.scene}</div>
          <div class="muted">${video.frame_count} 帧 · ${video.track_count} 条轨迹 · ${video.annotation_count || 0} 条标注</div>
          <div class="badges">${(video.classes || []).map((item) => `<span class="badge" style="--c:${item.color}">${className(item.class_id)}:${item.count}</span>`).join("")}</div>
        `;
        button.addEventListener("click", () => this.app.loadScene(video.scene));
        root.appendChild(button);
      });
  }
}

