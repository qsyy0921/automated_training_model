import { classColor, className } from "../../shared/catalog.js";
import { escapeHTML, qs, qsa } from "../../shared/dom.js";
import { annotationTrackKey, trackKey, trackLabel } from "../../entities/tracking.js";

export class AnomalyAnnotationFeature {
  constructor(app) {
    this.app = app;
  }

  bind() {
    qs("useObjectRange").addEventListener("click", () => this.useObjectRange());
    qs("fillObject").addEventListener("click", () => this.fillObject());
    qs("newSlot").addEventListener("click", () => this.addEmptySlot());
    qs("saveEvent").addEventListener("click", () => this.saveEvent());
    qs("objectSlots").addEventListener("keydown", (event) => this.handleSlotKeyboard(event));
  }

  ensureSlots(count = 5) {
    const slots = this.app.store.state.objectSlots;
    while (slots.length < count) slots.push({ empty: true, slot: slots.length + 1 });
    this.renderSlots();
  }

  addEmptySlot() {
    const slots = this.app.store.state.objectSlots;
    slots.push({ empty: true, slot: slots.length + 1 });
    this.renderSlots();
  }

  renderSegmentChecks() {
    const segments = this.app.store.state.meta?.anomaly_segments || [];
    qs("segmentChecks").innerHTML = segments.map((item) => `<label class="segment"><input type="checkbox" value="${item.index}"> #${item.index} ${item.start_frame}-${item.end_frame}</label>`).join("");
  }

  selectedSegments() {
    return qsa("input:checked", qs("segmentChecks")).map((item) => Number(item.value));
  }

  renderSlots() {
    const root = qs("objectSlots");
    const { objectSlots, activeSlot } = this.app.store.state;
    root.innerHTML = "";
    objectSlots.forEach((slot, index) => {
      const card = document.createElement("div");
      card.className = `slot ${slot.empty ? "empty" : ""} ${activeSlot === index ? "active" : ""}`;
      card.style.setProperty("--c", classColor(slot.class_id));
      card.innerHTML = slot.empty
        ? `<b>对象槽位 ${index + 1}: 空</b><small>选择对象后填入</small><button>删除槽位</button>`
        : `<b>${trackLabel(slot)}</b><small>${[slot.upper_color, slot.upper_clothing, slot.lower_color, slot.lower_clothing, slot.appearance].filter(Boolean).join(" / ") || "未填写描述"}</small><button>清空槽位</button>`;
      card.addEventListener("click", () => {
        this.app.store.patch({ activeSlot: index });
        this.renderSlots();
      });
      card.querySelector("button").addEventListener("click", (event) => {
        event.stopPropagation();
        objectSlots[index] = { empty: true, slot: index + 1 };
        this.renderSlots();
      });
      root.appendChild(card);
    });
  }

  fillObject() {
    const track = this.app.selectedTrack();
    if (!track) {
      alert("先选择对象");
      return;
    }
    this.ensureSlots();
    const slots = this.app.store.state.objectSlots;
    let index = slots.findIndex((slot) => slot.empty);
    if (index < 0) {
      slots.push({ empty: true, slot: slots.length + 1 });
      index = slots.length - 1;
    }
    slots[index] = {
      empty: false,
      slot: index + 1,
      track_key: trackKey(track),
      track_id: track.track_id,
      class_id: track.class_id,
      object_class: className(track.class_id),
      upper_color: qs("upperColor").value,
      lower_color: qs("lowerColor").value,
      upper_clothing: qs("upperClothing").value,
      lower_clothing: qs("lowerClothing").value,
      appearance: qs("appearance").value,
    };
    this.app.store.patch({ activeSlot: index });
    this.renderSlots();
  }

  useObjectRange() {
    const track = this.app.selectedTrack();
    const segment = this.app.store.state.lockedSegment;
    if (!track || !segment) {
      alert("必须先选择对象并锁定异常片段");
      return;
    }
    qs("start").value = Math.max(track.first_frame, segment.start_frame);
    qs("end").value = Math.min(track.last_frame, segment.end_frame);
  }

  async saveEvent() {
    const objects = this.app.store.state.objectSlots.filter((slot) => !slot.empty);
    if (!objects.length) {
      alert("至少填入一个异常对象");
      return;
    }
    const segments = this.selectedSegments();
    if (!segments.length) {
      alert("请选择所属异常时间段");
      return;
    }
    const eventID = `${this.app.store.state.scene}-event-${Date.now()}`;
    const eventReason = qs("eventReason").value;
    for (const object of objects) {
      await this.app.api.saveAnnotation(this.app.store.state.scene, {
        ...object,
        start_frame: Number(qs("start").value),
        end_frame: Number(qs("end").value),
        label: "异常",
        anomaly_type: qs("type").value,
        event_id: eventID,
        event_title: "异常事件",
        event_reason: eventReason,
        severity: qs("severity").value,
        tracking_status: "通过",
        tracking_issue: "正常",
        bbox_quality: "ok",
        notes: `segment_ids=${segments.join("|")}`,
      });
    }
    this.app.store.patch({ objectSlots: [] });
    this.ensureSlots();
    await this.app.refresh();
  }

  renderAnnotations() {
    const root = qs("annList");
    const annotations = this.app.store.state.annotations || [];
    root.innerHTML = annotations.length ? "" : '<div class="empty">暂无已保存标注。</div>';
    for (const ann of annotations) {
      const card = document.createElement("div");
      card.className = "card";
      card.style.setProperty("--c", classColor(ann.class_id));
      card.innerHTML = `
        <b>${className(ann.class_id)} 编号:${ann.track_id}</b>
        <small>${ann.start_frame}-${ann.end_frame} · ${ann.label || ""} · ${ann.anomaly_type || ""}</small>
        <small>原因/描述: ${escapeHTML(ann.event_reason || "")}</small>
        <button>跳转</button> <button class="danger">删除标注</button>
      `;
      const buttons = qsa("button", card);
      buttons[0].addEventListener("click", () => {
        this.app.selectTrack(annotationTrackKey(ann), false);
        this.app.loadFrame(ann.start_frame);
      });
      buttons[1].addEventListener("click", async () => {
        await this.app.api.deleteAnnotation(this.app.store.state.scene, ann.id);
        await this.app.refresh();
      });
      root.appendChild(card);
    }
  }

  renderEventTree() {
    const root = qs("eventTree");
    const segments = this.app.store.state.meta?.anomaly_segments || [];
    root.innerHTML = segments.length
      ? segments.map((item) => `<div class="card" style="--c:#f97316"><b>异常时间段 #${item.index}</b><small>${item.start_frame}-${item.end_frame} · ${item.length} 帧</small></div>`).join("")
      : '<div class="empty">无帧级异常片段</div>';
  }

  handleSlotKeyboard(event) {
    if (!["a", "d"].includes(event.key.toLowerCase())) return;
    event.preventDefault();
    const slots = this.app.store.state.objectSlots;
    if (!slots.length) return;
    const delta = event.key.toLowerCase() === "a" ? -1 : 1;
    this.app.store.patch({ activeSlot: (this.app.store.state.activeSlot + delta + slots.length) % slots.length });
    this.renderSlots();
  }
}

