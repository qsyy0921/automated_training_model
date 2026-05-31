import type { PropsWithChildren, ReactNode } from "react";

interface AppShellProps {
  sidebar: ReactNode;
  inspector: ReactNode;
  status: string;
}

export function AppShell({ sidebar, inspector, status, children }: PropsWithChildren<AppShellProps>) {
  return (
    <div className="appShell">
      <aside className="sidebar">{sidebar}</aside>
      <main className="workspace">{children}</main>
      <aside className="inspector">
        <div className="statusPill">{status}</div>
        {inspector}
      </aside>
    </div>
  );
}

