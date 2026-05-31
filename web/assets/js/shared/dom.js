export function qs(id) {
  return document.getElementById(id);
}

export function qsa(selector, root = document) {
  return [...root.querySelectorAll(selector)];
}

export function escapeHTML(value) {
  return String(value ?? "").replace(/[&<>"']/g, (ch) => ({
    "&": "&amp;",
    "<": "&lt;",
    ">": "&gt;",
    "\"": "&quot;",
    "'": "&#39;",
  })[ch]);
}

export function show(el, visible) {
  el.classList.toggle("is-hidden", !visible);
}

