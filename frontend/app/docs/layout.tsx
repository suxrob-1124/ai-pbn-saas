import { ReactNode } from "react";
import { DocsSidebar } from "../../components/DocsSidebar";

export default function DocsLayout({ children }: { children: ReactNode }) {
  return (
    <div className="grid gap-6 lg:grid-cols-[260px_1fr]">
      <DocsSidebar />
      <div className="rounded-2xl border border-slate-200 bg-white/80 p-8 shadow-sm dark:border-slate-800 dark:bg-slate-900/70">
        {children}
      </div>
    </div>
  );
}
