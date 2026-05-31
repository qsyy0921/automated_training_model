export const classCatalog = {
  0: { name: "人", color: "#08bdd6" },
  1: { name: "自行车", color: "#f4c600" },
  2: { name: "汽车", color: "#ef4444" },
  3: { name: "摩托车", color: "#9b5cf6" },
  5: { name: "公交车", color: "#2f6fed" },
  7: { name: "卡车", color: "#1ca66a" },
  36: { name: "滑板", color: "#d946ef" },
  80: { name: "婴儿车", color: "#f97316" },
};

export function className(classID) {
  return classCatalog[classID]?.name ?? `类别 ${classID}`;
}

export function classColor(classID) {
  return classCatalog[classID]?.color ?? "#8aa0b8";
}

export function renderClassOptions(selected = "") {
  const options = ['<option value="">全部类别</option>'];
  for (const [id, item] of Object.entries(classCatalog)) {
    options.push(`<option value="${id}" ${String(selected) === id ? "selected" : ""}>${item.name}</option>`);
  }
  return options.join("");
}

