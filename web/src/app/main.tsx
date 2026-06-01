import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { AppProviders } from "@app/providers/AppProviders";
import { AgentOverviewPage } from "@pages/agent-overview/AgentOverviewPage";
import "@app/styles/app.css";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <AppProviders>
      <AgentOverviewPage />
    </AppProviders>
  </StrictMode>
);
