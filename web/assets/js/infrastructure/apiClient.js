async function request(path, options = {}) {
  const headers = options.body instanceof FormData ? {} : { "Content-Type": "application/json" };
  const res = await fetch(path, { ...options, headers: { ...headers, ...(options.headers || {}) } });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `${res.status} ${res.statusText}`);
  }
  const contentType = res.headers.get("content-type") || "";
  return contentType.includes("application/json") ? res.json() : res.text();
}

export const api = {
  listVideos: () => request("/api/videos"),
  videoMeta: (scene) => request(`/api/video/${scene}/meta`),
  frameBoxes: (scene, frame) => request(`/api/video/${scene}/boxes?frame=${frame}`),
  saveAnnotation: (scene, payload) => request(`/api/video/${scene}/annotations`, { method: "POST", body: JSON.stringify(payload) }),
  deleteAnnotation: (scene, id) => request(`/api/video/${scene}/annotation/${id}`, { method: "DELETE" }),
  purgeTracks: (scene, trackKeys) => request(`/api/video/${scene}/purge-tracks`, { method: "POST", body: JSON.stringify({ track_keys: trackKeys }) }),
  listDatasets: () => request("/api/datasets"),
  registerFolderDataset: (payload) => request("/api/datasets/register-folder", { method: "POST", body: JSON.stringify(payload) }),
  registerManifestDataset: (payload) => request("/api/datasets/register-manifest", { method: "POST", body: JSON.stringify(payload) }),
  uploadArchiveDataset: (formData) => request("/api/datasets/upload-archive", { method: "POST", body: formData }),
  activateDataset: (id) => request(`/api/datasets/${id}/activate`, { method: "POST", body: "{}" }),
  submitTask: (path, payload) => request(path, { method: "POST", body: JSON.stringify(payload) }),
  taskStatus: (id) => request(`/api/tasks/${id}`),
};

