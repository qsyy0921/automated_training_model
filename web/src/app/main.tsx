import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { AppProviders } from "@app/providers/AppProviders";
import { AnnotationWorkbenchPage } from "@pages/annotation-workbench/AnnotationWorkbenchPage";
import "@app/styles/app.css";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <AppProviders>
      <AnnotationWorkbenchPage />
    </AppProviders>
  </StrictMode>
);

