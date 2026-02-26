import Link from "next/link";
import { FiCheck, FiClock, FiPauseCircle, FiPlay, FiRefreshCw, FiX } from "react-icons/fi";
import { Badge } from "../../../components/Badge";
import { getDomainLinkStatusMeta } from "../../../lib/linkTaskStatus";
import { hasLinkSettings } from "../services/actionGuards";
import { getDomainLinkBadgeStatus } from "../services/statusMeta";
import { getGenerationStatusMeta } from "../services/statusCta";

type Generation = {
  id: string;
  status: string;
  progress: number;
  error?: string;
  created_at?: string;
  updated_at?: string;
  started_at?: string;
  finished_at?: string;
};

type LinkDomain = {
  link_anchor_text?: string;
  link_acceptor_url?: string;
  link_status?: string;
  link_status_effective?: string;
  link_last_task_id?: string;
};

export function ProjectStatusBadge({ status }: { status: string }) {
  const meta = getGenerationStatusMeta(status);
  const icon =
    meta.icon === "play"
      ? <FiPlay />
      : meta.icon === "pause"
        ? <FiPauseCircle />
        : meta.icon === "check"
          ? <FiCheck />
          : meta.icon === "alert"
            ? <FiPauseCircle />
            : meta.icon === "x"
              ? <FiX />
              : <FiClock />;
  return <Badge label={meta.text} tone={meta.tone} icon={icon} className="text-xs" />;
}

export function ProjectLinkStatusBadge({ domain }: { domain: LinkDomain }) {
  const hasSettings = hasLinkSettings(domain.link_anchor_text, domain.link_acceptor_url);
  const status = getDomainLinkBadgeStatus(domain);
  const meta = getDomainLinkStatusMeta(status, hasSettings);
  const icon =
    meta.icon === "refresh" ? <FiRefreshCw /> : meta.icon === "check" ? <FiCheck /> : meta.icon === "alert" ? <FiPauseCircle /> : <FiClock />;
  return <Badge label={meta.text} tone={meta.tone} icon={icon} className="text-xs" />;
}

export function ProjectRunsList({ runs }: { runs: Generation[] }) {
  if (!Array.isArray(runs) || runs.length === 0) return null;
  const displayRuns = runs.slice(0, 4);
  return (
    <div className="mt-2 text-left text-xs bg-slate-50 dark:bg-slate-800/60 border border-slate-200 dark:border-slate-700 rounded-lg p-2 space-y-1">
      {displayRuns.map((run) => {
        const when = run.updated_at || run.created_at || run.started_at || run.finished_at;
        const label = when ? new Date(when).toLocaleString() : "Запуск";
        return (
          <Link
            key={run.id}
            href={`/queue/${run.id}`}
            className="flex items-center justify-between rounded-lg px-2 py-1 hover:bg-slate-100 dark:hover:bg-slate-700/60"
          >
            <span className="font-semibold">{label}</span>
            <div className="flex items-center gap-2">
              <ProjectStatusBadge status={run.status} />
              <span className="text-slate-500 dark:text-slate-400">{run.progress}%</span>
              {run.error && <span className="text-red-500">ошибка</span>}
            </div>
          </Link>
        );
      })}
      {runs.length > 4 && (
        <div className="text-xs text-slate-500 dark:text-slate-400 px-2 py-1">
          ... и еще {runs.length - 4} запусков
        </div>
      )}
    </div>
  );
}
