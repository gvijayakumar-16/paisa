/** @type {import('tailwindcss').Config} */
export default {
  corePlugins: {
    preflight: false
  },
  content: ["./src/**/*.{html,js,svelte,ts}"],
  theme: {
    extend: {}
  },
  plugins: [require("daisyui")],
  daisyui: {
    // Custom themes so DaisyUI components (dropdown, menu, tabs, ...) match
    // the violet accent + rounded corners set in src/app.scss for Bulma.
    themes: [
      {
        light: {
          primary: "#7c3aed",
          "primary-content": "#ffffff",
          secondary: "#0ea5e9",
          "secondary-content": "#ffffff",
          accent: "#10b981",
          "accent-content": "#ffffff",
          neutral: "#1e293b",
          "neutral-content": "#f1f5f9",
          "base-100": "#ffffff",
          "base-200": "#f8fafc",
          "base-300": "#e2e8f0",
          "base-content": "#1f2937",
          info: "#0ea5e9",
          success: "#22c55e",
          warning: "#f59e0b",
          error: "#ef4444",
          "--rounded-box": "0.875rem",
          "--rounded-btn": "0.625rem",
          "--rounded-badge": "9999px"
        },
        dark: {
          primary: "#a78bfa",
          "primary-content": "#1e1b4b",
          secondary: "#38bdf8",
          "secondary-content": "#0c2534",
          accent: "#34d399",
          "accent-content": "#022c22",
          neutral: "#1e293b",
          "neutral-content": "#e2e8f0",
          "base-100": "#17191d",
          "base-200": "#1d2025",
          "base-300": "#262a30",
          "base-content": "#e5e7eb",
          info: "#38bdf8",
          success: "#4ade80",
          warning: "#fbbf24",
          error: "#f87171",
          "--rounded-box": "0.875rem",
          "--rounded-btn": "0.625rem",
          "--rounded-badge": "9999px"
        }
      }
    ],
    darkTheme: "dark", // name of one of the included themes for dark mode
    base: false, // applies background color and foreground color for root element by default
    styled: true, // include daisyUI colors and design decisions for all components
    utils: true, // adds responsive and modifier utility classes
    rtl: false, // rotate style direction from left-to-right to right-to-left. You also need to add dir="rtl" to your html tag and install `tailwindcss-flip` plugin for Tailwind CSS.
    prefix: "du-", // prefix for daisyUI classnames (components, modifiers and responsive class names. Not colors)
    logs: false // Shows info about daisyUI version and used config in the console when building your CSS
  }
};
