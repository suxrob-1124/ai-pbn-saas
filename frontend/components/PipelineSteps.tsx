'use client';

import { CheckCircle2, CircleDashed, Loader2, Play, RefreshCw, RotateCcw } from 'lucide-react';
import { PIPELINE_STEPS, computeDisplayProgress, isStepComplete } from '../lib/pipelineProgress';

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

const ACTIVE_STATUSES = new Set([
  'pending',
  'processing',
  'pause_requested',
  'paused',
  'cancelling',
]);

const STATUS_TEXT: Record<string, string> = {
  done: 'Завершено',
  running: 'Выполняется',
  pending: 'Ожидание',
};

export default function PipelineSteps({
  domainId,
  generation,
  disabled,
  activeStep,
  onForceStep,
}: PipelineStepsProps) {
  const generationStatus = generation?.status || '';

  // Бизнес-логика: определяем, какой шаг сейчас в работе
  const firstIncompleteIndex = PIPELINE_STEPS.findIndex(
    (step) => !isStepComplete(generation?.artifacts, step.artifactKeys),
  );
  const isActiveRun = ACTIVE_STATUSES.has(generationStatus);
  const runningIndex = isActiveRun && firstIncompleteIndex >= 0 ? firstIncompleteIndex : -1;
  const displayProgress = computeDisplayProgress(
    generation?.artifacts,
    generation?.progress,
    generationStatus,
  );

  const handleForceClick = async (stepId: string) => {
    if (!onForceStep) return;
    await onForceStep(stepId);
  };

  return (
    <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3" data-domain-id={domainId}>
      {PIPELINE_STEPS.map((step, index) => {
        const isDone = isStepComplete(generation?.artifacts, step.artifactKeys);
        const statusKey = isDone ? 'done' : runningIndex === index ? 'running' : 'pending';
        const statusLabel = STATUS_TEXT[statusKey];

        const showForce =
          step.forceable &&
          (statusKey === 'done' || statusKey === 'pending' || statusKey === 'running');
        const buttonDisabled = disabled || Boolean(activeStep);
        const isButtonLoading = activeStep === step.id;

        // Динамические стили для карточки в зависимости от статуса
        let cardStyle =
          'bg-slate-50 dark:bg-[#0a1020] border-slate-200 dark:border-slate-800/60 opacity-60 grayscale-[50%]'; // pending
        let iconStyle = 'text-slate-400 dark:text-slate-600';
        let textStyle = 'text-slate-500 dark:text-slate-400';
        let Icon = CircleDashed;

        if (statusKey === 'done') {
          cardStyle =
            'bg-emerald-50 dark:bg-emerald-900/10 border-emerald-200 dark:border-emerald-800/50 shadow-sm';
          iconStyle = 'text-emerald-500 dark:text-emerald-400';
          textStyle = 'text-emerald-700 dark:text-emerald-300';
          Icon = CheckCircle2;
        } else if (statusKey === 'running') {
          cardStyle =
            'bg-indigo-50 dark:bg-indigo-900/20 border-indigo-300 dark:border-indigo-700 shadow-md ring-1 ring-indigo-500/20 dark:ring-indigo-400/20';
          iconStyle = 'text-indigo-600 dark:text-indigo-400 animate-pulse';
          textStyle = 'text-indigo-800 dark:text-indigo-300';
          Icon = Loader2;
        }

        return (
          <div
            key={step.id}
            className={`relative flex flex-col justify-between p-4 rounded-2xl border transition-all duration-300 overflow-hidden group ${cardStyle}`}>
            {/* Полоска прогресса для активного шага (внизу карточки) */}
            {statusKey === 'running' && (
              <div className="absolute bottom-0 left-0 h-1 bg-indigo-500/20 dark:bg-indigo-400/20 w-full">
                <div
                  className="h-full bg-indigo-600 dark:bg-indigo-400 transition-all duration-500 ease-out"
                  style={{ width: `${displayProgress}%` }}
                />
              </div>
            )}

            <div>
              {/* Шапка карточки: Номер шага и Иконка */}
              <div className="flex items-center justify-between mb-3">
                <span className="text-[10px] font-mono font-bold uppercase tracking-widest text-slate-400 dark:text-slate-500">
                  Шаг 0{index + 1}
                </span>
                <Icon
                  className={`w-5 h-5 ${statusKey === 'running' ? 'animate-spin' : ''} ${iconStyle}`}
                />
              </div>

              {/* Название шага */}
              <h4
                className={`text-sm font-bold leading-tight ${statusKey === 'pending' ? 'text-slate-600 dark:text-slate-400' : 'text-slate-900 dark:text-white'}`}>
                {step.label}
              </h4>
            </div>

            {/* Подвал: Статус и Кнопка */}
            <div
              className={`mt-4 pt-3 flex items-center justify-between border-t ${statusKey === 'done' ? 'border-emerald-200/60 dark:border-emerald-800/40' : statusKey === 'running' ? 'border-indigo-200/60 dark:border-indigo-800/40' : 'border-slate-200 dark:border-slate-800'}`}>
              <div className={`text-[11px] font-semibold uppercase tracking-wider ${textStyle}`}>
                {statusLabel}{' '}
                {statusKey === 'running' && (
                  <span className="ml-1 opacity-70 normal-case font-mono">
                    ({displayProgress}%)
                  </span>
                )}
              </div>

              {showForce && (
                <button
                  type="button"
                  onClick={() => handleForceClick(step.id)}
                  disabled={buttonDisabled}
                  className={`inline-flex items-center justify-center p-1.5 rounded-lg transition-colors ${
                    statusKey === 'done'
                      ? 'bg-emerald-100 text-emerald-700 hover:bg-emerald-200 dark:bg-emerald-800/50 dark:text-emerald-300 dark:hover:bg-emerald-700'
                      : statusKey === 'running'
                        ? 'bg-indigo-100 text-indigo-700 hover:bg-indigo-200 dark:bg-indigo-800/50 dark:text-indigo-300 dark:hover:bg-indigo-700'
                        : 'bg-slate-200 text-slate-600 hover:bg-slate-300 dark:bg-slate-800 dark:text-slate-400 dark:hover:bg-slate-700'
                  } disabled:opacity-50 disabled:cursor-not-allowed`}
                  title={`Перезапустить шаг: ${step.label}`}>
                  {isButtonLoading ? (
                    <Loader2 className="w-3.5 h-3.5 animate-spin" />
                  ) : (
                    <RotateCcw className="w-3.5 h-3.5" />
                  )}
                </button>
              )}
            </div>
          </div>
        );
      })}
    </div>
  );
}
