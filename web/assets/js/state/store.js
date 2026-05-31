export function createStore(initial = {}) {
  const state = {
    videos: [],
    scene: "",
    meta: null,
    tracks: [],
    boxes: [],
    annotations: [],
    frame: 1,
    selectedTrackKey: "",
    pendingDeletes: {},
    playing: false,
    timer: null,
    lockedSegment: null,
    objectSlots: [],
    activeSlot: 0,
    tracksCollapsed: false,
    activeDataset: "",
    ...initial,
  };
  const listeners = new Set();
  return {
    state,
    patch(update) {
      Object.assign(state, update);
      listeners.forEach((listener) => listener(state));
    },
    subscribe(listener) {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
  };
}

