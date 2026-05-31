export interface FolderDatasetDraft {
  name: string;
  merge_root: string;
  frame_root: string;
  mask_root?: string;
}

export interface ManifestDatasetDraft {
  name: string;
  manifest_path: string;
}

export function canRegisterFolderDataset(draft: FolderDatasetDraft): boolean {
  return Boolean(draft.name.trim() && draft.merge_root.trim() && draft.frame_root.trim());
}

export function canRegisterManifestDataset(draft: ManifestDatasetDraft): boolean {
  return Boolean(draft.name.trim() && draft.manifest_path.trim());
}

