"use client";

import type { ReactNode } from "react";
import { useOptionalMe } from "../lib/useAuth";

type AdminOnlySectionProps = {
  children: ReactNode;
};

export function AdminOnlySection({ children }: AdminOnlySectionProps) {
  const { me, loading } = useOptionalMe();
  if (loading) {
    return null;
  }
  const isAdmin = (me?.role || "").toLowerCase() === "admin";
  if (!isAdmin) {
    return null;
  }
  return <>{children}</>;
}
