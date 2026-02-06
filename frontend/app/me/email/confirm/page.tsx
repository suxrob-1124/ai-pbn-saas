"use client";

import { Suspense, useEffect, useState } from "react";
import { useSearchParams, useRouter } from "next/navigation";
import { post } from "../../../../lib/http";

export default function EmailConfirmPage() {
  return (
    <Suspense fallback={<div className="text-slate-500">Загрузка...</div>}>
      <Content />
    </Suspense>
  );
}

function Content() {
  const search = useSearchParams();
  const router = useRouter();
  const [status, setStatus] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    const token = search.get("token");
    if (!token) {
      setError("Токен не найден");
      return;
    }
    confirm(token);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [search]);

  const confirm = async (token: string) => {
    setLoading(true);
    setStatus(null);
    setError(null);
    try {
      await post("/api/email/change/confirm", { token });
      setStatus("Почта обновлена, перенаправляем...");
      setTimeout(() => router.replace("/me"), 800);
    } catch (err: any) {
      setError(err?.message || "Не удалось подтвердить");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="space-y-3">
      <h2 className="text-xl font-semibold">Подтверждение почты</h2>
      {status && <div className="text-emerald-500">{status}</div>}
      {error && <div className="text-red-500">{error}</div>}
      {loading && <div className="text-slate-500">Подтверждаем...</div>}
    </div>
  );
}
