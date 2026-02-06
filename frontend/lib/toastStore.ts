export type ToastType = "success" | "error" | "info" | "warning";

export type ToastItem = {
  id: string;
  type: ToastType;
  title: string;
  message?: string;
  createdAt: number;
};

export type ToastInput = {
  type: ToastType;
  title: string;
  message?: string;
  timeoutMs?: number;
};

type ToastListener = (items: ToastItem[]) => void;

let toasts: ToastItem[] = [];
const listeners = new Set<ToastListener>();

const emit = () => {
  const snapshot = [...toasts];
  listeners.forEach((listener) => listener(snapshot));
};

export const getToasts = () => [...toasts];

export const subscribeToasts = (listener: ToastListener) => {
  listeners.add(listener);
  listener([...toasts]);
  return () => listeners.delete(listener);
};

const normalizeTitle = (value: string) => {
  const trimmed = value.trim();
  if (!trimmed) {
    throw new Error("toast title is required");
  }
  return trimmed;
};

export const showToast = (input: ToastInput) => {
  const title = normalizeTitle(input.title);
  const toast: ToastItem = {
    id: crypto.randomUUID(),
    type: input.type,
    title,
    message: input.message?.trim() || undefined,
    createdAt: Date.now()
  };
  toasts = [toast, ...toasts].slice(0, 5);
  emit();
  const timeout = input.timeoutMs ?? 4000;
  if (timeout > 0) {
    setTimeout(() => dismissToast(toast.id), timeout);
  }
  return toast.id;
};

export const dismissToast = (id: string) => {
  const next = toasts.filter((item) => item.id !== id);
  if (next.length === toasts.length) {
    return false;
  }
  toasts = next;
  emit();
  return true;
};

export const clearToasts = () => {
  if (toasts.length === 0) {
    return;
  }
  toasts = [];
  emit();
};
