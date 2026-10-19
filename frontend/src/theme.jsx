import { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";
import { Icon } from "./components/common";

const STORAGE_KEY = "theme";
const ThemeContext = createContext(null);

function getPreferredTheme() {
  if (typeof window === "undefined") return "light";
  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (stored === "dark" || stored === "light") {
      return stored;
    }
  } catch (_) {}
  if (window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)").matches) {
    return "dark";
  }
  return "light";
}

function applyTheme(theme) {
  if (typeof document === "undefined") return;
  // Suppress transitions briefly so all colors flip instantly without contrast glitch
  let style;
  try {
    style = document.createElement("style");
    style.textContent = "*, *::before, *::after { transition: none !important; }";
    document.head.appendChild(style);
  } catch (_) {}

  document.documentElement.setAttribute("data-theme", theme);
  const metaThemeColor = document.querySelector('meta[name="theme-color"]');
  if (metaThemeColor) {
    metaThemeColor.setAttribute("content", theme === "dark" ? "#12151b" : "#f4f5f8");
  }

  if (style) {
    try {
      window.getComputedStyle(style).opacity;
      document.head.removeChild(style);
    } catch (_) {}
  }
}

export function ThemeProvider({ children }) {
  const [theme, setThemeState] = useState(getPreferredTheme);

  const setTheme = useCallback((newTheme) => {
    const next = newTheme === "dark" ? "dark" : "light";
    try {
      localStorage.setItem(STORAGE_KEY, next);
    } catch (_) {}
    setThemeState(next);
    applyTheme(next);
  }, []);

  const toggleTheme = useCallback(() => {
    setTheme(theme === "dark" ? "light" : "dark");
  }, [theme, setTheme]);

  useEffect(() => {
    applyTheme(theme);

    const mediaQuery = window.matchMedia ? window.matchMedia("(prefers-color-scheme: dark)") : null;
    if (!mediaQuery) return;

    const handleChange = (e) => {
      let stored = null;
      try {
        stored = localStorage.getItem(STORAGE_KEY);
      } catch (_) {}
      if (!stored) {
        const sysTheme = e.matches ? "dark" : "light";
        setThemeState(sysTheme);
        applyTheme(sysTheme);
      }
    };

    if (mediaQuery.addEventListener) {
      mediaQuery.addEventListener("change", handleChange);
      return () => mediaQuery.removeEventListener("change", handleChange);
    } else if (mediaQuery.addListener) {
      mediaQuery.addListener(handleChange);
      return () => mediaQuery.removeListener(handleChange);
    }
  }, [theme]);

  const value = useMemo(() => ({ theme, toggleTheme, setTheme }), [theme, toggleTheme, setTheme]);

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}

export function useTheme() {
  const context = useContext(ThemeContext);
  if (!context) {
    throw new Error("useTheme must be used within ThemeProvider");
  }
  return context;
}

export function ThemeToggle({ className = "" }) {
  const { theme, toggleTheme } = useTheme();
  const isDark = theme === "dark";
  const label = isDark ? "Ganti ke mode terang" : "Ganti ke mode gelap";
  const title = isDark ? "Mode terang" : "Mode gelap";

  return (
    <button
      type="button"
      className={`icon-button theme-toggle ${className}`.trim()}
      aria-label={label}
      title={title}
      onClick={toggleTheme}
    >
      <Icon name={isDark ? "sun" : "moon"} />
    </button>
  );
}
