const stepLabels: Record<string, string> = {
  ssh_probe: "Проверка сервера",
  file_sync: "Синхронизация файлов",
  link_decode: "Извлечение ссылки",
  link_baseline: "Создание baseline",
  artifacts: "Сборка артефактов",
  inventory_update: "Обновление инвентаря",
};

export function getStepLabel(step: string): string {
  return stepLabels[step] || step || "Ожидание";
}

const statusLabels: Record<string, string> = {
  pending: "Ожидание",
  running: "Выполняется",
  success: "Готово",
  failed: "Ошибка",
  skipped: "Пропущено",
  completed: "Завершено",
};

export function getStatusLabel(status: string): string {
  return statusLabels[status] || status;
}

const statusColors: Record<string, string> = {
  pending: "text-slate-400",
  running: "text-indigo-500",
  success: "text-emerald-500",
  failed: "text-red-500",
  skipped: "text-amber-500",
  completed: "text-emerald-500",
};

export function getStatusColor(status: string): string {
  return statusColors[status] || "text-slate-400";
}
