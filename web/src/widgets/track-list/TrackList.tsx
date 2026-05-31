import { Badge } from "@shared/ui/Badge";
import { Button } from "@shared/ui/Button";
import { fixed } from "@shared/lib/format";
import type { Track } from "@entities/track/model";
import { classColor, className, trackKey } from "@entities/track/model";

interface Props {
  tracks: Track[];
  selectedTrackKey: string;
  collapsed: boolean;
  search: string;
  classFilter: string;
  onToggle: () => void;
  onSearch: (value: string) => void;
  onClassFilter: (value: string) => void;
  onSelect: (key: string) => void;
}

export function TrackList({ tracks, selectedTrackKey, collapsed, search, classFilter, onToggle, onSearch, onClassFilter, onSelect }: Props) {
  const classIds = Array.from(new Set(tracks.map((item) => item.class_id))).sort((a, b) => a - b);
  const filtered = tracks.filter((track) => {
    const text = search.trim();
    const hitText = !text || `${track.track_id}`.includes(text) || className(track.class_id).includes(text);
    const hitClass = !classFilter || String(track.class_id) === classFilter;
    return hitText && hitClass;
  });

  return (
    <section className={`trackDock ${collapsed ? "collapsed" : ""}`}>
      <div className="dockHeader">
        <h3>轨迹列表</h3>
        <div className="dockActions">
          <Button onClick={onToggle}>{collapsed ? "展开轨迹列表" : "收起轨迹列表"}</Button>
          <input value={search} onChange={(event) => onSearch(event.target.value)} placeholder="轨迹 ID / 类别" />
          <select value={classFilter} onChange={(event) => onClassFilter(event.target.value)}>
            <option value="">全部类别</option>
            {classIds.map((id) => (
              <option key={id} value={id}>
                {className(id)}
              </option>
            ))}
          </select>
        </div>
      </div>
      {!collapsed && (
        <div className="trackGrid">
          {filtered.map((track) => {
            const key = trackKey(track);
            return (
              <button key={key} className={`trackCard ${selectedTrackKey === key ? "active" : ""}`} onClick={() => onSelect(key)}>
                <Badge color={track.color || classColor(track.class_id)}>
                  {className(track.class_id)} 编号:{track.track_id}
                </Badge>
                <small>
                  {track.first_frame}-{track.last_frame} · {track.frames} 次出现 · 平均置信度 {fixed(track.mean_conf || track.avg_conf, 2)}
                </small>
              </button>
            );
          })}
        </div>
      )}
    </section>
  );
}

