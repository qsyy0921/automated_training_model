# Frontend Architecture

版本：v0.2  
日期：2026-06-01

## 1. 技术路线

前端已经从原生 ES Modules 迁移到：

```text
Vite + React + TypeScript
+ TanStack Query
+ Zustand
+ Zod
+ FSD / 前端 DDD 分层
+ Design Tokens
```

选择原因：

- TypeScript 让 API 字段、业务实体和 UI 草稿状态有编译期约束。
- React 适合构建复杂工作台和可复用组件。
- Vite 保持本地开发和构建速度。
- TanStack Query 管理服务端状态、缓存、轮询和任务状态。
- Zustand 管理当前视频、当前帧、选中对象、草稿、播放器等 UI 状态。
- Zod 作为后续 API runtime validation 的边界工具。

## 2. FSD 目录结构

```text
web/
  package.json
  vite.config.ts
  tsconfig.json
  index.html
  src/
    app/
      main.tsx
      providers/
      store/
      styles/
    pages/
      annotation-workbench/
    widgets/
      app-shell/
      dataset-sidebar/
      inspector-panel/
      task-monitor-panel/
      track-list/
      video-review-layout/
    features/
      annotate-anomaly-event/
      register-dataset/
      review-tracking/
      select-video/
      submit-lifecycle-task/
    entities/
      anomaly-event/
      dataset/
      frame/
      task/
      track/
      video/
    shared/
      api/
      assets/
      config/
      lib/
      ui/
```

## 3. 依赖方向

```text
app -> pages -> widgets -> features -> entities -> shared
```

允许：

- page 组合 widgets/features。
- widget 调用 feature 传入的 handler 或共享实体模型。
- feature 使用 entities 和 shared。
- shared 不依赖任何业务层。

禁止：

- entities 依赖 widgets/features/pages。
- shared 依赖业务代码。
- 页面里堆业务算法。
- 组件直接散落 `fetch`，必须经过 `shared/api` 或对应 repository/adapter。

## 4. 状态分层

### Server State

由 TanStack Query 管理：

- videos
- video meta
- boxes
- annotations
- datasets
- task status

### UI State

由 Zustand 管理：

- 当前视频
- 当前帧
- 当前锁定异常片段
- 当前选中 track
- 播放状态
- 播放速度
- 轨迹列表收起状态

### Draft State

仍由 Zustand 管理，但与服务端状态分离：

- tracking 删除预览队列
- 异常事件对象槽位
- 未保存的对象外貌描述

保存前不得直接改写源数据。

## 5. 页面布局

```text
左侧 DatasetSidebar:
  数据集入口、视频搜索、类别过滤、视频列表

中间 Workspace:
  VideoReviewLayout:
    帧级展示、bbox overlay、对象点击选择、播放、倍速、锁定异常片段
  TaskMonitorPanel:
    数据接入、自动标注、训练、评估、部署任务入口
  TrackList:
    轨迹列表、轨迹搜索、类别过滤

右侧 InspectorPanel:
  当前对象
  tracking 删除预览与彻底删除
  异常事件标注
  对象槽位
  已保存标注
```

## 6. 设计系统方向

当前使用 CSS variables 维护设计 tokens：

- 品牌色：蓝 / 青
- 辅助强调色：粉 / 橙
- 状态色：成功 / 危险 / 警告
- 圆角、阴影、边框、面板背景集中定义

后续如果 UI 规模继续扩大，迁移到 Tailwind + Headless UI / Radix 思路的内部组件库。

## 7. 后续演进

- 增加 Zod schema 对 API 响应做运行时校验。
- 把复杂表单拆成 `features/annotate-anomaly-event` 内部子模块。
- 增加 toast/dialog，替换 `alert/confirm`。
- 增加 Playwright UI smoke tests。
- 增加任务中心页面、模型注册页面、部署页面。
- 如果出现多个团队独立维护子系统，再评估微前端或模块联邦。
