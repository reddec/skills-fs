import { NavLink, Navigate, Route, Routes } from "react-router-dom";
import { cn } from "./lib/utils";
import { SkillsPage } from "./pages/Skills";
import { SkillDetailPage } from "./pages/SkillDetail";
import { TokensPage } from "./pages/Tokens";

export default function App() {
  return (
    <div className="min-h-screen">
      <header className="border-b bg-background">
        <div className="mx-auto flex max-w-4xl items-center gap-6 px-4 py-3">
          <span className="font-semibold tracking-tight">Skills-FS</span>
          <nav className="flex gap-1 text-sm">
            <NavLink
              to="/skills"
              className={({ isActive }) =>
                cn(
                  "rounded-md px-3 py-1.5 transition-colors",
                  isActive ? "bg-accent font-medium text-accent-foreground" : "text-muted-foreground hover:text-foreground"
                )
              }
            >
              Skills
            </NavLink>
            <NavLink
              to="/tokens"
              className={({ isActive }) =>
                cn(
                  "rounded-md px-3 py-1.5 transition-colors",
                  isActive ? "bg-accent font-medium text-accent-foreground" : "text-muted-foreground hover:text-foreground"
                )
              }
            >
              Tokens
            </NavLink>
          </nav>
        </div>
      </header>
      <main className="mx-auto max-w-4xl px-4 py-6">
        <Routes>
          <Route path="/" element={<Navigate to="/skills" replace />} />
          <Route path="/skills" element={<SkillsPage />} />
          <Route path="/skills/:id" element={<SkillDetailPage />} />
          <Route path="/tokens" element={<TokensPage />} />
          <Route path="*" element={<Navigate to="/skills" replace />} />
        </Routes>
      </main>
    </div>
  );
}
