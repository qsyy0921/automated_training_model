# Frontend Architecture

版本：v0.1  
日期：2026-05-31

## 1. 设计目标

前端不能因为当前项目小就写成单页脚本。当前阶段采用无构建链路的原生 ES Modules，但按大型平台前端的方式提前拆分边界：

- 设计系统集中管理。
- API client 独立。
- UI state 与 draft state 独立。
- 复杂 viewer 专门化。
- 标注业务、tracking 审核、数据集、生命周期任务分别作为 feature。
- `index.html` 只负责页面骨架，不承载业务逻辑。

## 2. 目录结构

```text
web/
  index.html
  assets/
    css/
      app.css
    js/
      app/
        workbench.js
      app.js
      entities/
        tracking.js
      features/
        anomaly-annotation/
          anomalyAnnotation.js
        datasets/
          datasetPanel.js
        lifecycle/
          lifecyclePanel.js
        media-viewer/
          frameViewer.js
        tracking-review/
          trackingReview.js
        video-list/
          videoList.js
      infrastructure/
        apiClient.js
      shared/
        catalog.js
        dom.js
      state/
        store.js
```

## 3. 依赖方向

```text
app/workbench
  -> features
  -> infrastructure/apiClient
  -> state/store
  -> shared

features
  -> entities
  -> shared
  -> app orchestration only through injected app object

infrastructure
  -> browser API

shared
  -> no business feature dependency
```

禁止：

```text
feature A -> feature B
infrastructure -> feature
shared -> feature
index.html -> business logic
```

跨 feature 的协作由 `WorkbenchApp` 编排。例如：

```text
FrameViewer 选择对象
-> WorkbenchApp.selectTrack
-> TrackingReview 和 AnomalyAnnotation 读取统一 state
```

## 4. 状态分类

### Server State

来自 Go API：

- videos
- video meta
- tracks
- boxes
- annotations
- datasets
- task status

### UI State

只影响界面：

- 当前视频
- 当前帧
- 当前锁定异常片段
- 当前选中 track
- 播放状态
- 轨迹列表是否收起

### Draft State

用户尚未保存的编辑：

- tracking 删除预览队列
- 异常事件对象槽位
- 表单字段

保存前不应直接改写源数据。

## 5. 当前页面布局

```text
左侧 Sidebar：
  数据接入、视频列表、类别过滤

中间 Workspace：
  顶部样本状态
  帧级审核主画布
  播放范围/倍速/异常片段
  底部轨迹列表

右侧 Inspector：
  当前对象
  删除预览
  异常事件标注
  已保存标注
  视频-片段-事件-对象结构
```

## 6. 迁移路线

短期保持原生 ES Modules：

- 零 npm 依赖。
- Go 静态文件服务即可运行。
- 方便研究机器、内网服务器、Docker 环境启动。

当出现以下情况时迁移到 Vite + TypeScript + React：

- feature 数量超过 10 个。
- 表单状态和校验明显复杂。
- 需要复用组件库和端到端测试。
- 需要多页面路由、权限、任务中心、模型 registry 复杂表格。

迁移时保留：

- `features` 边界。
- `entities` 模型。
- `infrastructure/apiClient`。
- `state` 的状态分类。

