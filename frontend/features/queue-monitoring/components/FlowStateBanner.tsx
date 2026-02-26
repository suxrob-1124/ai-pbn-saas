import type { QueueMonitoringFlowState } from "../hooks/useFlowState";
import {
  getQueueMonitoringFlowStatusLabel,
  queueMonitoringRu,
  type QueueMonitoringFlowStatus
} from "../services/i18n-ru";

type FlowStateBannerProps = {
  title: string;
  flow: QueueMonitoringFlowState;
};

const FLOW_TONE_CLASS: Record<Exclude<QueueMonitoringFlowStatus, "idle">, string> = {
  validating: "border-amber-300 bg-amber-50 text-amber-800 dark:border-amber-700 dark:bg-amber-900/30 dark:text-amber-200",
  sending: "border-blue-300 bg-blue-50 text-blue-800 dark:border-blue-700 dark:bg-blue-900/30 dark:text-blue-200",
  done: "border-emerald-300 bg-emerald-50 text-emerald-800 dark:border-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-200",
  error: "border-red-300 bg-red-50 text-red-800 dark:border-red-700 dark:bg-red-900/30 dark:text-red-200"
};

export function FlowStateBanner({ title, flow }: FlowStateBannerProps) {
  if (flow.status === "idle") {
    return null;
  }
  const statusLabel = getQueueMonitoringFlowStatusLabel(flow.status);
  const toneClass = FLOW_TONE_CLASS[flow.status];
  return (
    <div className={`rounded-lg border px-3 py-2 text-xs ${toneClass}`}>
      <div className="font-semibold">
        {title}: {statusLabel}
      </div>
      {flow.message && <div className="mt-0.5">{flow.message}</div>}
      {flow.status === "error" && (
        <details className="mt-1">
          <summary className="cursor-pointer select-none text-[11px] opacity-80">
            {queueMonitoringRu.diagnostics.title}
          </summary>
          <div className="mt-1 whitespace-pre-wrap break-words text-[11px] opacity-90">
            {flow.diagnostics || queueMonitoringRu.diagnostics.empty}
          </div>
        </details>
      )}
    </div>
  );
}
