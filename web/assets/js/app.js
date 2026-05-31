import { WorkbenchApp } from "./app/workbench.js";

window.addEventListener("DOMContentLoaded", async () => {
  const app = new WorkbenchApp();
  window.__videoLabelWorkbench = app;
  try {
    await app.init();
  } catch (err) {
    console.error(err);
    app.setStatus("初始化失败");
    alert(err.message);
  }
});

