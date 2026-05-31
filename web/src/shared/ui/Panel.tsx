import type { PropsWithChildren } from "react";

export function Panel({ title, action, children, className = "" }: PropsWithChildren<{ title?: string; action?: React.ReactNode; className?: string }>) {
  return (
    <section className={`panel ${className}`}>
      {(title || action) && (
        <div className="panelHeader">
          {title ? <h3>{title}</h3> : <span />}
          {action}
        </div>
      )}
      {children}
    </section>
  );
}

