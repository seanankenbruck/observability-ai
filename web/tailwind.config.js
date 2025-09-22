export default {
  darkMode: 'class',
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}"
  ],
  theme: {
    extend: {
      colors: {
        glass: 'rgba(255,255,255,0.1)',
        glassDark: 'rgba(30,41,59,0.4)'
      },
      backdropBlur: {
        xs: '2px',
      },
      boxShadow: {
        glass: '0 4px 32px 0 rgba(0,0,0,0.12)',
      }
    },
  },
  plugins: [],
};