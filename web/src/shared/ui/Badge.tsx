import type { CSSProperties, PropsWithChildren } from "react";

export function Badge({ color, children }: PropsWithChildren<{ color?: string }>) {
  return (
    <span className="badge" style={{ "--badge-color": color ?? "#7a8ca8" } as CSSProperties}>
      {children}
    </span>
  );
}
