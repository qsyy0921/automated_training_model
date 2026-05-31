import { qs, show } from "../../shared/dom.js";

export class DatasetPanelFeature {
  constructor(app) {
    this.app = app;
  }

  bind() {
    qs("toggleImport").addEventListener("click", () => show(qs("importBox"), qs("importBox").classList.contains("is-hidden")));
    qs("registerFolder").addEventListener("click", () => this.registerFolder());
    qs("registerManifest").addEventListener("click", () => this.registerManifest());
    qs("uploadArchive").addEventListener("click", () => this.uploadArchive());
  }

  async load() {
    try {
      const data = await this.app.api.listDatasets();
      this.render(data.datasets || []);
    } catch {
      this.render([]);
    }
  }

  render(rows) {
    const root = qs("datasetList");
    root.innerHTML = rows.length ? "" : '<div class="empty">暂无已登记数据集。</div>';
    for (const dataset of rows) {
      const card = document.createElement("div");
      card.className = "card";
      card.innerHTML = `<b>${dataset.name}</b><small>${dataset.source_type} · ${dataset.status || "registered"}</small><button>激活</button>`;
      card.querySelector("button").addEventListener("click", () => this.activate(dataset.id));
      root.appendChild(card);
    }
  }

  async activate(id) {
    await this.app.api.activateDataset(id);
    this.app.store.patch({ activeDataset: id, scene: "" });
    qs("lifecycleDataset").value = id;
    this.app.setStatus("数据集已激活");
    await this.app.loadVideos();
    await this.load();
  }

  async registerFolder() {
    const data = await this.app.api.registerFolderDataset({
      name: qs("dsName").value,
      merge_root: qs("mergeRoot").value,
      frame_root: qs("frameRoot").value,
      mask_root: qs("maskRoot").value,
    });
    await this.activate(data.dataset.id);
  }

  async registerManifest() {
    const data = await this.app.api.registerManifestDataset({
      name: qs("dsName").value || "manifest dataset",
      manifest_path: qs("manifestPath").value,
    });
    await this.activate(data.dataset.id);
  }

  async uploadArchive() {
    const file = qs("archiveFile").files[0];
    if (!file) {
      alert("请选择压缩包");
      return;
    }
    const body = new FormData();
    body.append("file", file);
    body.append("name", qs("dsName").value || file.name);
    const data = await this.app.api.uploadArchiveDataset(body);
    await this.activate(data.dataset.id);
  }
}

