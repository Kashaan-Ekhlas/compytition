"use client";

import { useEffect, useState } from "react";

const STORAGE_KEY = "compytition-theme";

export function ThemeToggle() {
  const [theme, setTheme] = useState<"paper" | "arcade">("paper");

  useEffect(() => {
    const stored = window.localStorage.getItem(STORAGE_KEY);
    const initial = stored === "arcade" ? "arcade" : "paper";
    setTheme(initial);
    document.documentElement.dataset.theme = initial;
  }, []);

  function toggle() {
    const next = theme === "paper" ? "arcade" : "paper";
    setTheme(next);
    document.documentElement.dataset.theme = next;
    window.localStorage.setItem(STORAGE_KEY, next);
  }

  return (
    <button
      type="button"
      onClick={toggle}
      className="flex h-12 w-12 items-center justify-center border-2 border-ink bg-surface font-pixel text-sm shadow-hard-sm transition-transform active:translate-x-[2px] active:translate-y-[2px] active:shadow-none"
      aria-label={theme === "paper" ? "Switch to arcade theme" : "Switch to paper theme"}
      title={theme === "paper" ? "Arcade theme" : "Paper theme"}
    >
      {theme === "paper" ? "◐" : "☀"}
    </button>
  );
}
