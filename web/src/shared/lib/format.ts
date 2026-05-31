export function pct(value?: number): string {
  if (value === undefined || Number.isNaN(value)) return "-";
  return `${Math.round(value * 100)}%`;
}

export function fixed(value?: number, digits = 2): string {
  if (value === undefined || Number.isNaN(value)) return "-";
  return value.toFixed(digits);
}

export function clamp(value: number, min: number, max: number): number {
  return Math.max(min, Math.min(max, value));
}

