import { useEffect, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { AppShell } from "@widgets/app-shell/AppShell";
import { DatasetSidebar } from "@widgets/dataset-sidebar/DatasetSidebar";
import { InspectorPanel, type SaveEventPayload } from "@widgets/inspector-panel/InspectorPanel";
import { TaskMonitorPanel } from "@widgets/task-monitor-panel/TaskMonitorPanel";
import { TrackList } from "@widgets/track-list/TrackList";
import { VideoReviewLayout } from "@widgets/video-review-layout/VideoReviewLayout";
import { useWorkbenchStore } from "@app/store/workbenchStore";
import { apiClient } from "@shared/api/client";
import { clamp } from "@shared/lib/format";
import { trackKey, type Box } from "@entities/track/model";
import type { ObjectSlot, Segment } from "@entities/anomaly-event/model";
import type { VideoSummary } from "@entities/video/model";
import { addTrackToDeleteQueue } from "@features/review-tracking/model";
import { compactObjectSlot, createEmptyObjectSlots } from "@features/annotate-anomaly-event/model";
import { nextVideo } from "@features/select-video/model";

const EMPTY_VIDEOS: VideoSummary[] = [];
const EMPTY_BOXES: Box[] = [];

export function AnnotationWorkbenchPage() {
  const queryClient = useQueryClient();
  const [showDataPanel, setShowDataPanel] = useState(false);
  const state = useWorkbenchStore();
  const setState = useWorkbenchStore((s) => s.setState);

  const videosQuery = useQuery({ queryKey: ["videos"], queryFn: () => apiClient.listVideos() });
  const taxonomyQuery = useQuery({ queryKey: ["taxonomy"], queryFn: () => apiClient.taxonomy() });
  const videos = videosQuery.data?.videos || EMPTY_VIDEOS;
  const scene = state.currentScene || videos[0]?.scene || "";
  const metaQuery = useQuery({ queryKey: ["video-meta", scene], queryFn: () => apiClient.videoMeta(scene), enabled: Boolean(scene) });
  const meta = metaQuery.data;
  const range = state.lockedSegment ? [state.lockedSegment.start_frame, state.lockedSegment.end_frame] : [1, meta?.frame_count || 1];
  const frame = clamp(state.currentFrame, range[0], range[1]);
  const boxesQuery = useQuery({ queryKey: ["boxes", scene, frame], queryFn: () => apiClient.frameBoxes(scene, frame), enabled: Boolean(scene) });

  useEffect(() => {
    if (!scene) return;
    const ahead = state.playbackMode === "frames" ? 12 : 4;
    for (let offset = 1; offset <= ahead; offset += 1) {
      const nextFrame = frame + offset;
      if (nextFrame > range[1]) break;
      queryClient.prefetchQuery({
        queryKey: ["boxes", scene, nextFrame],
        queryFn: () => apiClient.frameBoxes(scene, nextFrame),
        staleTime: 60_000
      });
    }
  }, [frame, queryClient, range, scene, state.playbackMode]);

  useEffect(() => {
    if (!videosQuery.data) return;
    if (videos.length && !state.currentScene) setState({ videos, currentScene: videos[0].scene });
    else setState({ videos });
  }, [setState, state.currentScene, videos, videosQuery.data]);

  useEffect(() => {
    if (meta) {
      setState({ meta, tracks: meta.tracks || [], boxes: boxesQuery.data?.boxes || EMPTY_BOXES });
    }
  }, [boxesQuery.data?.boxes, meta, setState]);

  const selectedTrack = useMemo(() => state.tracks.find((track) => trackKey(track) === state.selectedTrackKey), [state.selectedTrackKey, state.tracks]);

  const refreshVideo = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ["videos"] }),
      queryClient.invalidateQueries({ queryKey: ["video-meta", scene] }),
      queryClient.invalidateQueries({ queryKey: ["boxes", scene] })
    ]);
  };

  const saveAnnotation = useMutation({
    mutationFn: async (payload: SaveEventPayload) => {
      const eventID = `${scene}-event-${Date.now()}`;
      const cleanObjects = payload.objects.map(compactObjectSlot);
      for (const object of cleanObjects) {
        await apiClient.saveAnnotation(scene, {
          ...object,
          start_frame: payload.start,
          end_frame: payload.end,
          label: "异常",
          anomaly_type: payload.anomalyType,
          event_id: eventID,
          event_title: "异常事件",
          event_reason: payload.reason,
          severity: payload.severity,
          tracking_status: "通过",
          tracking_issue: "正常",
          bbox_quality: "ok",
          notes: `segment_ids=${payload.segmentIds.join("|")}`
        });
      }
    },
    onSuccess: async () => {
      setState({ objectSlots: createEmptyObjectSlots(), activeObjectSlot: 0, statusText: "已保存标注" });
      await refreshVideo();
    }
  });

  const deleteAnnotation = useMutation({
    mutationFn: (id: string) => apiClient.deleteAnnotation(scene, id),
    onSuccess: refreshVideo
  });

  const purgeTracks = useMutation({
    mutationFn: () => apiClient.purgeTracks(scene, state.pendingDeleteKeys),
    onSuccess: async () => {
      setState({ pendingDeleteKeys: [], selectedTrackKey: "", statusText: "已彻底删除轨迹数据" });
      await refreshVideo();
    }
  });

  const selectScene = (nextScene: string) => {
    if (state.pendingDeleteKeys.length || state.objectSlots.some((slot) => !slot.empty)) {
      const ok = confirm("当前视频有未保存草稿，切换视频会丢弃草稿。是否继续？");
      if (!ok) return;
    }
    setState({
      currentScene: nextScene,
      currentFrame: 1,
      selectedTrackKey: "",
      pendingDeleteKeys: [],
      lockedSegment: undefined,
      objectSlots: createEmptyObjectSlots(),
      activeObjectSlot: 0
    });
  };

  const adjacentVideo = (delta: number) => {
    const next = nextVideo(videos, scene, delta > 0 ? 1 : -1);
    if (next) selectScene(next.scene);
  };

  const queueDelete = () => {
    if (!state.selectedTrackKey) return;
    setState({ pendingDeleteKeys: addTrackToDeleteQueue(state.pendingDeleteKeys, state.selectedTrackKey) });
  };

  const setLockedSegment = (segment?: Segment) => {
    setState({ lockedSegment: segment, currentFrame: segment ? segment.start_frame : state.currentFrame, playing: false });
  };

  return (
    <AppShell
      status={state.statusText}
      sidebar={
        <DatasetSidebar
          videos={videos}
          currentScene={scene}
          searchText={state.searchText}
          classFilter={state.classFilter}
          onSearch={(searchText) => setState({ searchText })}
          onClassFilter={(classFilter) => setState({ classFilter })}
          onSelect={selectScene}
          onToggleDataPanel={() => setShowDataPanel((v) => !v)}
        />
      }
      inspector={
        <InspectorPanel
          selectedTrack={selectedTrack}
          selectedTrackKey={state.selectedTrackKey}
          segments={meta?.anomaly_segments || []}
          lockedSegment={state.lockedSegment}
          pendingDeletes={state.pendingDeleteKeys}
          annotations={meta?.annotations || []}
          objectSlots={state.objectSlots}
          activeSlot={state.activeObjectSlot}
          taxonomy={taxonomyQuery.data}
          onQueueDelete={queueDelete}
          onClearDeletes={() => setState({ pendingDeleteKeys: [] })}
          onPurgeDeletes={() => purgeTracks.mutate()}
          onSlots={(objectSlots: ObjectSlot[], activeObjectSlot = state.activeObjectSlot) => setState({ objectSlots, activeObjectSlot })}
          onUseObjectRange={({ start }) => setState({ currentFrame: start })}
          onSaveEvent={(payload) => saveAnnotation.mutate(payload)}
          onJump={(jumpFrame, key) => setState({ currentFrame: jumpFrame, selectedTrackKey: key || state.selectedTrackKey })}
          onDeleteAnnotation={(id) => deleteAnnotation.mutate(id)}
        />
      }
    >
      <VideoReviewLayout
        scene={scene}
        meta={meta}
        frame={frame}
        boxes={boxesQuery.data?.boxes || []}
        selectedTrackKey={state.selectedTrackKey}
        lockedSegment={state.lockedSegment}
        playRate={state.playRate}
        playbackMode={state.playbackMode}
        reviewFPS={state.reviewFPS}
        playing={state.playing}
        pendingDeletes={state.pendingDeleteKeys}
        onFrameChange={(currentFrame) => setState({ currentFrame: clamp(currentFrame, range[0], range[1]) })}
        onSelectTrack={(selectedTrackKey) => setState({ selectedTrackKey })}
        onSegmentLock={setLockedSegment}
        onPlayRate={(playRate) => setState({ playRate })}
        onPlaybackMode={(playbackMode) => setState({ playbackMode, playing: false })}
        onReviewFPS={(reviewFPS) => setState({ reviewFPS })}
        onPlaying={(playing) => setState({ playing })}
        onAdjacentVideo={adjacentVideo}
      />
      <TaskMonitorPanel visible={showDataPanel} onDatasetActivated={refreshVideo} />
      <TrackList
        tracks={state.tracks}
        selectedTrackKey={state.selectedTrackKey}
        collapsed={state.trackListCollapsed}
        search={state.searchText}
        classFilter={state.classFilter}
        onToggle={() => setState({ trackListCollapsed: !state.trackListCollapsed })}
        onSearch={(searchText) => setState({ searchText })}
        onClassFilter={(classFilter) => setState({ classFilter })}
        onSelect={(selectedTrackKey) => setState({ selectedTrackKey })}
      />
    </AppShell>
  );
}
