import { useEffect, useRef, useState } from "react";
import { apiBase } from "./http";

type CaptchaState = {
  id: string | null;
  question: string | null;
  ttl: number; // seconds
  remaining?: number;
};

export function useCaptcha(required: boolean = true) {
  const [captcha, setCaptcha] = useState<CaptchaState>({ id: null, question: null, ttl: 0 });
  const [answer, setAnswer] = useState("");
  const timer = useRef<NodeJS.Timeout | null>(null);

  const refresh = async () => {
    try {
      const res = await fetch(`${apiBase()}/api/captcha`);
      if (!res.ok) return;
      const data = await res.json();
      setCaptcha({
        id: data.id || null,
        question: data.question || null,
        ttl: typeof data.ttl === "number" ? data.ttl : 120,
        remaining: typeof data.remainingAttempts === "number" ? data.remainingAttempts : undefined
      });
      setAnswer("");
    } catch {
      // ignore errors
    }
  };

  useEffect(() => {
    refresh();
    return () => {
      if (timer.current) clearInterval(timer.current);
    };
  }, []);

  useEffect(() => {
    if (timer.current) clearInterval(timer.current);
    if (!captcha.id) return;
    timer.current = setInterval(() => {
      setCaptcha((prev) => {
        if (!prev.id) return prev;
        const nextTTL = prev.ttl - 1;
        if (nextTTL <= 0) {
          refresh();
          return { id: null, question: null, ttl: 0 };
        }
        return { ...prev, ttl: nextTTL };
      });
    }, 1000);
  }, [captcha.id]);

  return {
    captchaId: captcha.id,
    question: captcha.question,
    ttl: captcha.ttl,
    remainingAttempts: captcha.remaining,
    answer,
    setAnswer,
    refresh,
    required
  };
}
