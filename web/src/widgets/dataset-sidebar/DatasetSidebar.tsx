import { useMemo } from "react";
import { Badge } from "@shared/ui/Badge";
import { Button } from "@shared/ui/Button";
import type { VideoSummary } from "@entities/video/model";
import { classColor, className } from "@entities/track/model";

interface Props {
  videos: VideoSummary[];
  currentScene: string;
  searchText: string;
  classFilter: string;
  onSearch: (value: string) => void;
  onClassFilter: (value: string) => void;
  onSelect: (scene: string) => void;
  onToggleDataPanel: () => void;
}

export function DatasetSidebar({ videos, currentScene, searchText, classFilter, onSearch, onClassFilter, onSelect, onToggleDataPanel }: Props) {
  const classOptions = useMemo(() => {
    const ids = new Set<number>();
    videos.forEach((video) => video.classes?.forEach((item) => ids.add(item.class_id)));
    return Array.from(ids).sort((a, b) => a - b);
  }, [videos]);

  const filtered = videos.filter((video) => {
    const hitText = !searchText || video.scene.includes(searchText.trim());
    const hitClass = !classFilter || video.classes?.some((item) => String(item.class_id) === classFilter);
    return hitText && hitClass;
  });

  return (
    <>
      <section className="brandBlock">
        <p className="eyebrow">Automated Training Model</p>
        <h1>视频标注与训练工作台</h1>
        <p>轨迹审核、对象级异常标注、自动训练闭环</p>
      </section>
      <section className="sidebarTools">
        <input value={searchText} onChange={(event) => onSearch(event.target.value)} placeholder="搜索视频，如 04_0012" />
        <select value={classFilter} onChange={(event) => onClassFilter(event.target.value)}>
          <option value="">全部类别</option>
          {classOptions.map((id) => (
            <option key={id} value={id}>
              {className(id)}
            </option>
          ))}
        </select>
        <Button className="wide" onClick={onToggleDataPanel}>
          数据接入 / 训练任务
        </Button>
      </section>
      <section className="videoList">
        {filtered.map((video) => (
          <button key={video.scene} className={`videoItem ${video.scene === currentScene ? "active" : ""}`} onClick={() => onSelect(video.scene)}>
            <b>{video.scene}</b>
            <small>
              {video.frame_count} 帧 · {video.track_count} 条轨迹 · {video.annotation_count} 条标注
            </small>
            <span className="badgeRow">
              {video.classes?.slice(0, 5).map((item) => (
                <Badge key={item.class_id} color={item.color || classColor(item.class_id)}>
                  {className(item.class_id)} {item.count}
                </Badge>
              ))}
            </span>
          </button>
        ))}
      </section>
      <a className="exportLink" href="/api/videos" target="_blank" rel="noreferrer">
        导出/检查当前数据 JSON
      </a>
    </>
  );
}

