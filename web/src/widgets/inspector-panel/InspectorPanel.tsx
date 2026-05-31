import { useEffect, useMemo, useState } from "react";
import { Button } from "@shared/ui/Button";
import { Panel } from "@shared/ui/Panel";
import { DEFAULT_ANOMALY_TYPES, DEFAULT_CARRYING, DEFAULT_CLOTHING_COLORS, DEFAULT_LOWER_CLOTHING, DEFAULT_UPPER_CLOTHING } from "@shared/config/classCatalog";
import type { ObjectSlot, Segment } from "@entities/anomaly-event/model";
import type { Taxonomy } from "@entities/taxonomy/model";
import type { Track } from "@entities/track/model";
import type { AnnotationRecord } from "@entities/video/model";
import { className, trackKey, trackDisplayName } from "@entities/track/model";
import { createEmptyObjectSlots, intersectTrackWithSegment, normalizeObjectSlots } from "@features/annotate-anomaly-event/model";

interface Props {
  selectedTrack?: Track;
  selectedTrackKey: string;
  segments: Segment[];
  lockedSegment?: Segment;
  pendingDeletes: string[];
  annotations: AnnotationRecord[];
  objectSlots: ObjectSlot[];
  activeSlot: number;
  taxonomy?: Taxonomy;
  onQueueDelete: () => void;
  onClearDeletes: () => void;
  onPurgeDeletes: () => void;
  onSlots: (slots: ObjectSlot[], activeSlot?: number) => void;
  onUseObjectRange: (range: { start: number; end: number }) => void;
  onSaveEvent: (payload: SaveEventPayload) => void;
  onJump: (frame: number, key?: string) => void;
  onDeleteAnnotation: (id: string) => void;
}

export interface SaveEventPayload {
  start: number;
  end: number;
  segmentIds: number[];
  anomalyType: string;
  reason: string;
  severity: string;
  objects: ObjectSlot[];
}

export function InspectorPanel({
  selectedTrack,
  segments,
  lockedSegment,
  pendingDeletes,
  annotations,
  objectSlots,
  activeSlot,
  taxonomy,
  onQueueDelete,
  onClearDeletes,
  onPurgeDeletes,
  onSlots,
  onUseObjectRange,
  onSaveEvent,
  onJump,
  onDeleteAnnotation
}: Props) {
  const [start, setStart] = useState(lockedSegment?.start_frame || 1);
  const [end, setEnd] = useState(lockedSegment?.end_frame || 1);
  const [segmentIds, setSegmentIds] = useState<number[]>(lockedSegment ? [lockedSegment.index] : []);
  const [anomalyTypes, setAnomalyTypes] = useState<string[]>(["骑车"]);
  const [reason, setReason] = useState("");
  const [severity, setSeverity] = useState("中");
  const [appearance, setAppearance] = useState({ upper_color: "未填写", upper_clothing: "未填写", lower_color: "未填写", lower_clothing: "未填写", carrying: "未填写" });

  const selectedText = selectedTrack ? `${trackDisplayName(selectedTrack)} · ${selectedTrack.first_frame}-${selectedTrack.last_frame}` : "未选择";

  const activeObject = useMemo(() => objectSlots[activeSlot], [activeSlot, objectSlots]);
  const anomalyTypeOptions = taxonomy?.anomaly_types?.length ? taxonomy.anomaly_types : [...DEFAULT_ANOMALY_TYPES];
  const clothingColors = taxonomy?.clothing_colors?.length ? taxonomy.clothing_colors : [...DEFAULT_CLOTHING_COLORS];
  const upperClothing = taxonomy?.upper_clothing?.length ? taxonomy.upper_clothing : [...DEFAULT_UPPER_CLOTHING];
  const lowerClothing = taxonomy?.lower_clothing?.length ? taxonomy.lower_clothing : [...DEFAULT_LOWER_CLOTHING];
  const carrying = taxonomy?.carrying?.length ? taxonomy.carrying : [...DEFAULT_CARRYING];

  useEffect(() => {
    if (!lockedSegment) return;
    setStart(lockedSegment.start_frame);
    setEnd(lockedSegment.end_frame);
    setSegmentIds((ids) => (ids.includes(lockedSegment.index) ? ids : [lockedSegment.index]));
  }, [lockedSegment]);

  const resetEventDraft = () => {
    setStart(lockedSegment?.start_frame || 1);
    setEnd(lockedSegment?.end_frame || 1);
    setSegmentIds(lockedSegment ? [lockedSegment.index] : []);
    setAnomalyTypes(["骑车"]);
    setSeverity("中");
    setReason("");
    setAppearance({ upper_color: "未填写", upper_clothing: "未填写", lower_color: "未填写", lower_clothing: "未填写", carrying: "未填写" });
    onSlots(createEmptyObjectSlots(), 0);
  };

  const fillCurrentObject = () => {
    if (!selectedTrack) {
      alert("请先在画面或轨迹列表中选择对象");
      return;
    }
    const next = [...objectSlots];
    let index = next.findIndex((slot) => slot.empty);
    if (index < 0) {
      index = next.length;
      next.push({ slot: index + 1, empty: true });
    }
    next[index] = {
      slot: index + 1,
      empty: false,
      track_key: trackKey(selectedTrack),
      track_id: selectedTrack.track_id,
      class_id: selectedTrack.class_id,
      object_class: className(selectedTrack.class_id),
      ...Object.fromEntries(Object.entries(appearance).filter(([, value]) => value && value !== "未填写"))
    };
    onSlots(next, index);
  };

  const addSlot = () => onSlots([...objectSlots, { slot: objectSlots.length + 1, empty: true }], objectSlots.length);

  const removeSlot = (index: number) => {
    const next = normalizeObjectSlots(objectSlots.filter((_, i) => i !== index));
    onSlots(next, Math.max(0, index - 1));
  };

  const setRangeFromObjectAndSegment = () => {
    if (!selectedTrack || !lockedSegment) {
      alert("必须先选择对象并锁定异常片段，才会自动填写对象轨迹与片段的交集");
      return;
    }
    const range = intersectTrackWithSegment(selectedTrack, lockedSegment);
    if (!range) {
      alert("当前对象轨迹与锁定片段没有交集");
      return;
    }
    setStart(range.start);
    setEnd(range.end);
    onUseObjectRange(range);
  };

  const save = () => {
    const objects = objectSlots.filter((slot) => !slot.empty);
    if (!objects.length) {
      alert("至少添加一个异常相关对象");
      return;
    }
    if (!segmentIds.length) {
      alert("请选择异常事件所属的异常时间段");
      return;
    }
    onSaveEvent({ start, end, segmentIds, anomalyType: anomalyTypes.join("|"), reason, severity, objects });
  };

  return (
    <div className="inspectorStack">
      <Panel title="当前对象">
        <p className="selectedText">{selectedText}</p>
        <div className="buttonGrid">
          <Button variant="danger" onClick={onQueueDelete} disabled={!selectedTrack}>
            加入删除预览
          </Button>
          <Button onClick={fillCurrentObject} disabled={!selectedTrack}>
            填入异常对象
          </Button>
        </div>
      </Panel>

      <Panel title="轨迹删除预览">
        {pendingDeletes.length ? (
          <div className="stack">
            {pendingDeletes.map((key) => (
              <span className="deleteChip" key={key}>
                {key}
              </span>
            ))}
          </div>
        ) : (
          <div className="empty">暂无待删除轨迹。</div>
        )}
        <div className="buttonGrid">
          <Button onClick={onClearDeletes}>清空待删</Button>
          <Button variant="danger" onClick={onPurgeDeletes} disabled={!pendingDeletes.length}>
            彻底删除数据
          </Button>
        </div>
      </Panel>

      <Panel
        title="异常事件（一个事件可含多个对象）"
        action={
          <Button type="button" onClick={resetEventDraft}>
            新增异常事件
          </Button>
        }
      >
        <label>
          所属异常时间段（可多选）
          <div className="segmentCheckGrid">
            {segments.map((segment) => (
              <label className="segmentCheck" key={segment.index}>
                <input
                  type="checkbox"
                  checked={segmentIds.includes(segment.index)}
                  onChange={(event) => {
                    setSegmentIds(event.target.checked ? [...segmentIds, segment.index] : segmentIds.filter((id) => id !== segment.index));
                  }}
                />
                #{segment.index} {segment.start_frame}-{segment.end_frame}
              </label>
            ))}
          </div>
        </label>
        <div className="formGrid two">
          <label>
            开始帧
            <input type="number" value={start} onChange={(event) => setStart(Number(event.target.value))} />
          </label>
          <label>
            结束帧
            <input type="number" value={end} onChange={(event) => setEnd(Number(event.target.value))} />
          </label>
        </div>
        <Button onClick={setRangeFromObjectAndSegment}>按当前对象 ∩ 当前锁定片段设置时间段</Button>
        <div className="formGrid two">
          <label className="fullSpan">
            异常类型（可多选）
            <div className="checkGrid">
              {anomalyTypeOptions.map((item) => (
                <label className="segmentCheck" key={item}>
                  <input
                    type="checkbox"
                    checked={anomalyTypes.includes(item)}
                    onChange={(event) => {
                      setAnomalyTypes(event.target.checked ? [...anomalyTypes, item] : anomalyTypes.filter((value) => value !== item));
                    }}
                  />
                  {item}
                </label>
              ))}
            </div>
          </label>
          <label>
            严重程度
            <select value={severity} onChange={(event) => setSeverity(event.target.value)}>
              {["低", "中", "高"].map((item) => (
                <option key={item}>{item}</option>
              ))}
            </select>
          </label>
        </div>
        <label>
          异常原因/描述
          <textarea value={reason} onChange={(event) => setReason(event.target.value)} placeholder="例如：白色上衣黑色裤子的人骑黑色自行车穿过步行区域" />
        </label>

        <div className="appearancePanel">
          <h4>当前对象在该异常中的描述</h4>
          <div className="formGrid two">
            <Select label="上衣颜色" value={appearance.upper_color} options={clothingColors} onChange={(v) => setAppearance({ ...appearance, upper_color: v })} />
            <Select label="上衣类型" value={appearance.upper_clothing} options={upperClothing} onChange={(v) => setAppearance({ ...appearance, upper_clothing: v })} />
            <Select label="下装颜色" value={appearance.lower_color} options={clothingColors} onChange={(v) => setAppearance({ ...appearance, lower_color: v })} />
            <Select label="下装类型" value={appearance.lower_clothing} options={lowerClothing} onChange={(v) => setAppearance({ ...appearance, lower_clothing: v })} />
            <Select label="携带/关联物" value={appearance.carrying} options={carrying} onChange={(v) => setAppearance({ ...appearance, carrying: v })} />
          </div>
        </div>

        <div className="objectSlotHeader">
          <h4>异常相关对象槽位</h4>
          <Button onClick={addSlot}>新增对象槽位</Button>
        </div>
        <div className="objectSlots">
          {objectSlots.map((slot, index) => (
            <div
              key={slot.slot}
              role="button"
              tabIndex={0}
              className={`objectSlot ${activeSlot === index ? "active" : ""}`}
              onClick={() => onSlots(objectSlots, index)}
              onKeyDown={(event) => {
                if (event.key === "Enter" || event.key === " ") onSlots(objectSlots, index);
              }}
            >
              <b>{slot.empty ? `对象槽位 ${slot.slot}: 空` : `${slot.object_class} 编号:${slot.track_id}`}</b>
              <small>
                {slot.empty
                  ? "选择画面对象后点击填入异常对象"
                  : [slot.upper_color, slot.upper_clothing, slot.lower_color, slot.lower_clothing, slot.carrying, slot.appearance].filter(Boolean).join(" / ")}
              </small>
              <span className="inlineActions">
                <Button type="button" onClick={(event) => { event.stopPropagation(); removeSlot(index); }}>
                  删除对象
                </Button>
              </span>
            </div>
          ))}
        </div>
        <div className="buttonGrid">
          <Button onClick={() => onSlots(createEmptyObjectSlots(), 0)}>删除异常事件草稿</Button>
          <Button variant="primary" onClick={save}>
            保存异常事件
          </Button>
        </div>
        <p className="hint">保存时只保留非“未填写”的对象外貌字段。</p>
      </Panel>

      <Panel title="已保存标注">
        {annotations.length ? (
          <div className="savedList">
            {annotations.map((ann) => (
              <article key={ann.id} className="savedCard">
                <b>
                  {className(ann.class_id)} 编号:{ann.track_id}
                </b>
                <small>
                  {ann.start_frame}-{ann.end_frame} · {ann.anomaly_type} · {ann.event_id}
                </small>
                <small>原因/描述: {ann.event_reason || "-"}</small>
                <small>
                  外貌: {[ann.upper_color, ann.upper_clothing, ann.lower_color, ann.lower_clothing, ann.carrying, ann.appearance].filter(Boolean).join(" / ") || "-"}
                </small>
                <div className="inlineActions">
                  <Button onClick={() => onJump(ann.start_frame, `${ann.class_id}:${ann.track_id}`)}>跳转</Button>
                  <Button variant="danger" onClick={() => onDeleteAnnotation(ann.id)}>删除标注</Button>
                </div>
              </article>
            ))}
          </div>
        ) : (
          <div className="empty">暂无已保存标注。</div>
        )}
      </Panel>
      {activeObject && !activeObject.empty && <p className="hint">当前槽位：{activeObject.object_class} 编号:{activeObject.track_id}</p>}
    </div>
  );
}

function Select<T extends readonly string[]>({ label, value, options, onChange }: { label: string; value: string; options: T; onChange: (value: string) => void }) {
  return (
    <label>
      {label}
      <select value={value} onChange={(event) => onChange(event.target.value)}>
        {options.map((item) => (
          <option key={item}>{item}</option>
        ))}
      </select>
    </label>
  );
}
