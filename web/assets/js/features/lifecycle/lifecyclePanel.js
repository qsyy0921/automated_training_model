import { qs } from "../../shared/dom.js";

export class LifecyclePanelFeature {
  constructor(app) {
    this.app = app;
  }

  bind() {
    qs("autoLabelJob").addEventListener("click", () => this.submit("/api/autolabel/jobs", { dataset_id: this.datasetID(), task_types: ["detection", "tracking", "caption"], require_review: true }));
    qs("trainJob").addEventListener("click", () => this.submit("/api/training/runs", { dataset_id: this.datasetID(), target_task: "object_detection", model_family: "yolo", training_config: { epochs: "100" } }));
    qs("evalJob").addEventListener("click", () => this.submit("/api/evaluation/runs", { dataset_id: this.datasetID(), model_id: "candidate-model", metrics: ["recall50", "mean_iou"], save_visuals: true, failure_mining: true }));
    qs("deployJob").addEventListener("click", () => this.submit("/api/deployments", { model_id: "candidate-model", target: "local", runtime: "onnxruntime", strategy: "manual" }));
    qs("checkTask").addEventListener("click", () => this.checkTask());
  }

  datasetID() {
    return qs("lifecycleDataset").value || this.app.store.state.activeDataset;
  }

  async submit(path, payload) {
    try {
      const data = await this.app.api.submitTask(path, payload);
      const taskID = data.job?.task_id || data.run?.task_id || data.model?.task_id || data.deployment?.task_id || "";
      qs("taskID").value = taskID;
      this.app.setStatus(`任务已提交 ${taskID}`);
    } catch (err) {
      alert(err.message);
    }
  }

  async checkTask() {
    const id = qs("taskID").value;
    if (!id) {
      alert("请输入 task id");
      return;
    }
    const data = await this.app.api.taskStatus(id);
    alert(JSON.stringify(data.task, null, 2));
  }
}

