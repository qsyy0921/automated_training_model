import { useMutation, useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { apiClient } from "@shared/api/client";
import { Button } from "@shared/ui/Button";
import { Panel } from "@shared/ui/Panel";
import type { DatasetRecord } from "@entities/dataset/model";
import { canRegisterFolderDataset, canRegisterManifestDataset } from "@features/register-dataset/model";
import { lifecycleTaskEndpoint, type LifecycleTaskKind } from "@features/submit-lifecycle-task/model";

interface Props {
  visible: boolean;
  onDatasetActivated?: () => void;
}

export function TaskMonitorPanel({ visible, onDatasetActivated }: Props) {
  const [datasetID, setDatasetID] = useState("");
  const [folderPayload, setFolderPayload] = useState({ name: "", merge_root: "", frame_root: "", mask_root: "" });
  const [manifestPayload, setManifestPayload] = useState({ name: "", manifest_path: "" });
  const [taskID, setTaskID] = useState("");

  const datasets = useQuery({ queryKey: ["datasets"], queryFn: () => apiClient.listDatasets(), enabled: visible });
  const task = useQuery({ queryKey: ["task", taskID], queryFn: () => apiClient.taskStatus(taskID), enabled: Boolean(taskID), refetchInterval: taskID ? 3000 : false });
  const taskLogs = useQuery({ queryKey: ["task-logs", taskID], queryFn: () => apiClient.taskLogs(taskID, 12), enabled: Boolean(taskID), refetchInterval: taskID ? 3000 : false });
  const taskManifest = useQuery({ queryKey: ["task-manifest", taskID], queryFn: () => apiClient.taskManifest(taskID), enabled: Boolean(taskID), refetchInterval: taskID ? 3000 : false });
  const taskLineage = useQuery({ queryKey: ["task-lineage", taskID], queryFn: () => apiClient.taskLineage(taskID), enabled: Boolean(taskID), refetchInterval: taskID ? 3000 : false });
  const resumeTask = useMutation({
    mutationFn: (id: string) => apiClient.resumeTask(id),
    onSuccess: (res) => setTaskID(res.task.id)
  });

  const registerFolder = useMutation({
    mutationFn: () => apiClient.registerFolderDataset(folderPayload),
    onSuccess: (res) => {
      setDatasetID(res.dataset.id);
      datasets.refetch();
    }
  });
  const registerManifest = useMutation({
    mutationFn: () => apiClient.registerManifestDataset(manifestPayload),
    onSuccess: (res) => {
      setDatasetID(res.dataset.id);
      datasets.refetch();
    }
  });
  const activate = useMutation({
    mutationFn: (id: string) => apiClient.activateDataset(id),
    onSuccess: () => {
      onDatasetActivated?.();
      datasets.refetch();
    }
  });

  const submitTask = useMutation({
    mutationFn: (path: string) => apiClient.submitTask(path, { dataset_id: datasetID, profile: "default" }),
    onSuccess: (res) => setTaskID(res.task.id)
  });

  if (!visible) return null;

  return (
    <section className="taskPanel">
      <Panel title="数据接入">
        <div className="formGrid two">
          <input placeholder="数据集名称" value={folderPayload.name} onChange={(event) => setFolderPayload({ ...folderPayload, name: event.target.value })} />
          <input placeholder="merge/csv 根目录" value={folderPayload.merge_root} onChange={(event) => setFolderPayload({ ...folderPayload, merge_root: event.target.value })} />
          <input placeholder="frames 根目录" value={folderPayload.frame_root} onChange={(event) => setFolderPayload({ ...folderPayload, frame_root: event.target.value })} />
          <input placeholder="可选：帧级 mask 目录" value={folderPayload.mask_root} onChange={(event) => setFolderPayload({ ...folderPayload, mask_root: event.target.value })} />
        </div>
        <Button variant="primary" onClick={() => registerFolder.mutate()} disabled={!canRegisterFolderDataset(folderPayload)}>
          注册本地目录
        </Button>
        <div className="formGrid two">
          <input placeholder="manifest 名称" value={manifestPayload.name} onChange={(event) => setManifestPayload({ ...manifestPayload, name: event.target.value })} />
          <input placeholder="manifest.json / parquet 索引路径" value={manifestPayload.manifest_path} onChange={(event) => setManifestPayload({ ...manifestPayload, manifest_path: event.target.value })} />
        </div>
        <Button onClick={() => registerManifest.mutate()} disabled={!canRegisterManifestDataset(manifestPayload)}>
          注册 Manifest
        </Button>
      </Panel>

      <Panel title="已登记数据集">
        <div className="datasetGrid">
          {(datasets.data?.datasets || []).map((ds: DatasetRecord) => (
            <div
              key={ds.id}
              role="button"
              tabIndex={0}
              className={`datasetCard ${ds.active ? "active" : ""}`}
              onClick={() => setDatasetID(ds.id)}
              onKeyDown={(event) => {
                if (event.key === "Enter" || event.key === " ") setDatasetID(ds.id);
              }}
            >
              <b>{ds.name || ds.id}</b>
              <small>{ds.source_type}</small>
              <small>{ds.id}</small>
              <Button type="button" onClick={(event) => { event.stopPropagation(); activate.mutate(ds.id); }}>
                激活
              </Button>
            </div>
          ))}
        </div>
      </Panel>

      <Panel title="自动训练生命周期任务">
        <input placeholder="dataset id" value={datasetID} onChange={(event) => setDatasetID(event.target.value)} />
        <div className="buttonGrid">
          {([
            ["autolabel", "自动标注"],
            ["training", "训练任务"],
            ["evaluation", "评估任务"],
            ["deployment", "部署任务"]
          ] as [LifecycleTaskKind, string][]).map(([kind, label]) => (
            <Button key={kind} onClick={() => submitTask.mutate(lifecycleTaskEndpoint[kind])} disabled={!datasetID}>
              {label}
            </Button>
          ))}
        </div>
        <input placeholder="task id" value={taskID} onChange={(event) => setTaskID(event.target.value)} />
        {task.data && (
          <div className="taskStatus">
            <b>{task.data.id}</b>
            <small>{task.data.status}</small>
            <small>{task.data.message}</small>
            {task.data.resumable ? (
              <Button type="button" onClick={() => resumeTask.mutate(task.data.id)} disabled={resumeTask.isPending}>
                重新排队
              </Button>
            ) : null}
            {taskLogs.data?.worker_heartbeat ? (
              <small>
                heartbeat {taskLogs.data.worker_heartbeat.status} · {taskLogs.data.worker_heartbeat.message || taskLogs.data.worker_heartbeat.at}
              </small>
            ) : null}
            {taskLogs.data?.metadata?.artifact_manifest ? <small>{taskLogs.data.metadata.artifact_manifest}</small> : null}
            {taskManifest.data?.manifest?.artifact_summary ? (
              <small>
                artifacts {taskManifest.data.manifest.artifact_summary.artifact_count} ·
                {" "}primary {taskManifest.data.manifest.artifact_summary.primary_artifact?.role || "-"}
              </small>
            ) : null}
            {taskLineage.data?.lineage?.length ? (
              <small>
                lineage {taskLineage.data.root_id} · {(taskLineage.data.lineage || []).map((item) => `${item.id}:${item.status}`).join(" -> ")}
              </small>
            ) : null}
            {taskLogs.data?.artifacts?.length ? <small>artifacts {taskLogs.data.artifacts.length}</small> : null}
            {taskLogs.data?.stdout ? <pre className="taskLogPre">{taskLogs.data.stdout}</pre> : null}
            {taskLogs.data?.stderr ? <pre className="taskLogPre taskLogPreWarn">{taskLogs.data.stderr}</pre> : null}
            {taskLogs.data?.logs?.length ? (
              <div className="taskLogList">
                {taskLogs.data.logs.map((log) => (
                  <div key={`${log.at}-${log.message}`} className="taskLogRow">
                    <small>{log.level}</small>
                    <span>{log.message}</span>
                  </div>
                ))}
              </div>
            ) : null}
          </div>
        )}
      </Panel>
    </section>
  );
}
