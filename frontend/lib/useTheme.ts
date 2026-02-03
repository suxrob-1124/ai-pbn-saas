import { useEffect, useState } from "react";

const THEME_KEY = "ui-theme";

export function useTheme() {
  const [theme, setThemeState] = useState<"light" | "dark">("dark");

  useEffect(() => {
    const saved = (typeof window !== "undefined" && localStorage.getItem(THEME_KEY)) as
      | "light"
      | "dark"
      | null;
    const initial = saved || "dark";
    setTheme(initial);
  }, []);

  const setTheme = (t: "light" | "dark") => {
    setThemeState(t);
    if (typeof window !== "undefined") {
      document.documentElement.classList.toggle("dark", t === "dark");
      document.documentElement.classList.toggle("light", t === "light");
      localStorage.setItem(THEME_KEY, t);
    }
  };

  const toggle = () => setTheme(theme === "dark" ? "light" : "dark");

  return { theme, setTheme, toggle };
}
