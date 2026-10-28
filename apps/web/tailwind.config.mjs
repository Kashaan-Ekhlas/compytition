/** @type {import('tailwindcss').Config} */
const config = {
  darkMode: ["class", '[data-theme="arcade"]'],
  content: [
    "./app/**/*.{ts,tsx}",
    "./components/**/*.{ts,tsx}",
    "./lib/**/*.{ts,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        bg: "rgb(var(--bg-rgb) / <alpha-value>)",
        surface: "rgb(var(--surface-rgb) / <alpha-value>)",
        ink: "rgb(var(--ink-rgb) / <alpha-value>)",
        muted: "rgb(var(--muted-rgb) / <alpha-value>)",
        accent: "rgb(var(--accent-rgb) / <alpha-value>)",
        "accent-2": "rgb(var(--accent-2-rgb) / <alpha-value>)",
        "accent-3": "rgb(var(--accent-3-rgb) / <alpha-value>)",
        success: "rgb(var(--success-rgb) / <alpha-value>)",
        focus: "rgb(var(--focus-rgb) / <alpha-value>)",
      },
      fontFamily: {
        sans: ["Space Grotesk", "ui-sans-serif", "system-ui", "sans-serif"],
        pixel: ["Space Grotesk", "ui-sans-serif", "system-ui", "sans-serif"],
        mono: ["IBM Plex Mono", "ui-monospace", "SFMono-Regular", "monospace"],
      },
      boxShadow: {
        hard: "4px 4px 0 var(--ink)",
        "hard-sm": "2px 2px 0 var(--ink)",
        "hard-accent": "4px 4px 0 var(--accent)",
      },
      borderRadius: {
        DEFAULT: "4px",
        sm: "2px",
      },
      maxWidth: {
        content: "1100px",
        reading: "720px",
      },
      keyframes: {
        blink: {
          "0%, 49%": { opacity: "1" },
          "50%, 100%": { opacity: "0" },
        },
        "step-in": {
          "0%": { opacity: "0", transform: "translateY(4px)" },
          "100%": { opacity: "1", transform: "translateY(0)" },
        },
      },
      animation: {
        blink: "blink 1s steps(1) infinite",
        "step-in": "step-in 140ms steps(3) both",
      },
    },
  },
  plugins: [],
};

export default config;
