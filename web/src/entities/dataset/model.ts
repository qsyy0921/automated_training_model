export interface DatasetRecord {
  id: string;
  name: string;
  source_type: string;
  merge_root?: string;
  frame_root?: string;
  mask_root?: string;
  upload_path?: string;
  manifest_path?: string;
  active?: boolean;
  created_at?: string;
}

