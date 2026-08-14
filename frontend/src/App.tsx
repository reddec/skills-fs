import { lazy, Suspense } from "react";
import { NavLink, Navigate, Route, Routes, useLocation } from "react-router-dom";
import { cn } from "./lib/utils";
import { SkillsPage } from "./pages/Skills";
import { SkillDetailPage } from "./pages/SkillDetail";
import { TokensPage } from "./pages/Tokens";
import { SetupPage } from "./pages/Setup";

// The editor pulls in CodeMirror; load it only on the editor route to keep the app bundle lean.
const SkillEditorPage = lazy(() => import("./pages/SkillEditor").then((m) => ({ default: m.SkillEditorPage })));
const editorFallback = <div className="text-sm text-muted-foreground">Loading editor…</div>;

export default function App() {
  const { pathname } = useLocation();
  const fullWidth = pathname.endsWith("/edit") || pathname === "/skills/new";
  const mainClass = fullWidth ? "px-4 py-6" : "mx-auto max-w-6xl px-4 py-6";
  return (
    <div className="min-h-screen">
      <header className="border-b bg-background">
        <div className="mx-auto flex max-w-6xl items-center gap-6 px-4 py-3">
          <span className="font-semibold tracking-tight">Skills-FS</span>
          <nav className="flex flex-wrap gap-1 text-sm">
            <NavLink to="/skills" className={navClass}>
              Skills
            </NavLink>
            <NavLink to="/tokens" className={navClass}>
              Tokens
            </NavLink>
            <NavLink to="/setup" className={navClass}>
              Mount
            </NavLink>
          </nav>
        </div>
      </header>
      <main className={mainClass}>
        <Routes>
          <Route path="/" element={<Navigate to="/skills" replace />} />
          <Route path="/skills" element={<SkillsPage />} />
          <Route
            path="/skills/new"
            element={
              <Suspense fallback={editorFallback}>
                <SkillEditorPage mode="create" />
              </Suspense>
            }
          />
          <Route path="/skills/:id" element={<SkillDetailPage />} />
          <Route
            path="/skills/:id/edit"
            element={
              <Suspense fallback={editorFallback}>
                <SkillEditorPage mode="edit" />
              </Suspense>
            }
          />
          <Route path="/tokens" element={<TokensPage />} />
          <Route path="/setup" element={<SetupPage />} />
          <Route path="*" element={<Navigate to="/skills" replace />} />
        </Routes>
      </main>
    </div>
  );
}

function navClass({ isActive }: { isActive: boolean }) {
  return cn(
    "rounded-md px-3 py-1.5 transition-colors",
    isActive ? "bg-accent font-medium text-accent-foreground" : "text-muted-foreground hover:text-foreground"
  );
}
