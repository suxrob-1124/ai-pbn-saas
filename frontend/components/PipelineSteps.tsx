"use client";

import { FiCheck, FiClock, FiLoader, FiRefreshCw } from "react-icons/fi";
import { PIPELINE_STEPS, computeDisplayProgress, isStepComplete } from "../lib/pipelineProgress";

type PipelineGeneration = {
  artifacts?: Record<string, any>;
  status?: string;
  progress?: number;
};

type PipelineStepsProps = {
  domainId: string;
  generation?: PipelineGeneration;
  disabled?: boolean;
  activeStep?: string | null;
  onForceStep?: (stepId: string) => Promise<void>;
};

const ACTIVE_STATUSES = new Set(["pending", "processing", "pause_requested", "paused", "cancelling"]);

const STATUS_TEXT: Record<string, string> = {
  done: "Готово",
  running: "В процессе",
  pending: "Ожидание",
};

export default function PipelineSteps({ domainId, generation, disabled, activeStep, onForceStep }: PipelineStepsProps) {
  const generationStatus = generation?.status || "";
  const firstIncompleteIndex = PIPELINE_STEPS.findIndex((step) => !isStepComplete(generation?.artifacts, step.artifactKeys));
  const isActiveRun = ACTIVE_STATUSES.has(generationStatus);
  const runningIndex = isActiveRun && firstIncompleteIndex >= 0 ? firstIncompleteIndex : -1;
  const displayProgress = computeDisplayProgress(generation?.artifacts, generation?.progress, generationStatus);

  const handleForceClick = async (stepId: string) => {
    if (!onForceStep) {
      return;
    }
    await onForceStep(stepId);
  };

  return (
    <div className="space-y-3" data-domain-id={domainId}>
      <div className="grid gap-3 md:grid-cols-2">
        {PIPELINE_STEPS.map((step, index) => {
          const isDone = isStepComplete(generation?.artifacts, step.artifactKeys);
          const statusKey = isDone ? "done" : runningIndex === index ? "running" : "pending";
          const statusLabel = statusKey === "running" ? `${STATUS_TEXT[statusKey]} · ${displayProgress}%` : STATUS_TEXT[statusKey];
          const Icon = statusKey === "done" ? FiCheck : statusKey === "running" ? FiLoader : FiClock;
          const showForce = step.forceable && (statusKey === "done" || statusKey === "pending" || statusKey === "running");
          const buttonDisabled = disabled || Boolean(activeStep);
          const isButtonLoading = activeStep === step.id;

          return (
            <div
              key={step.id}
              className="flex items-center justify-between rounded-xl border border-slate-200 bg-slate-50 px-4 py-3 text-sm dark:border-slate-700 dark:bg-slate-900/40"
            >
              <div className="flex items-center gap-3">
                <span className={`text-lg ${statusKey === "done" ? "text-emerald-500" : statusKey === "running" ? "text-amber-500" : "text-slate-500"}`}>
                  <Icon className={statusKey === "running" ? "animate-spin" : ""} />
                </span>
                <div>
                  <div className="font-semibold text-slate-900 dark:text-slate-100">{step.label}</div>
                  <div className="text-xs text-slate-500 dark:text-slate-400">{statusLabel}</div>
                </div>
              </div>
              {showForce && (
                <button
                  type="button"
                  onClick={() => handleForceClick(step.id)}
                  disabled={buttonDisabled}
                  className="inline-flex h-8 w-8 items-center justify-center rounded-full border border-slate-200 bg-white text-slate-600 shadow-sm hover:border-slate-400 disabled:cursor-not-allowed disabled:opacity-60 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-300"
                  title={`Перегенерировать ${step.label}`}
                >
                  {isButtonLoading ? <FiLoader className="animate-spin" /> : <FiRefreshCw />}
                </button>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}
