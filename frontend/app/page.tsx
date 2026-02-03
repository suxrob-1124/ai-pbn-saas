"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { getToken } from "../lib/token";

export default function Home() {
  const router = useRouter();

  useEffect(() => {
    router.replace("/login");
  }, [router]);

  return null;
}
