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
        bg: "var(--bg)",
        surface: "var(--surface)",
        ink: "var(--ink)",
        muted: "var(--muted)",
        accent: "var(--accent)",
        "accent-2": "var(--accent-2)",
        "accent-3": "var(--accent-3)",
        success: "var(--success)",
        focus: "var(--focus)",
      },
      fontFamily: {
        sans: ["Space Grotesk", "ui-sans-serif", "system-ui", "sans-serif"],
        pixel: ["Silkscreen", "Space Mono", "monospace"],
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
