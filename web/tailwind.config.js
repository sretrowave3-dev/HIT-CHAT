/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        cyber: {
          900: '#0a0d14',
          800: '#111827',
          700: '#1f2937',
          600: '#374151',
          accent: '#10b981',
          danger: '#ef4444',
          warning: '#f59e0b',
        }
      },
      animation: {
        'burn-fade': 'burnFade 1s ease-out forwards',
        'pulse-glow': 'pulseGlow 2s infinite',
      },
      keyframes: {
        burnFade: {
          '0%': { opacity: '1', transform: 'scale(1)', filter: 'blur(0)' },
          '50%': { opacity: '0.8', transform: 'scale(0.98)', filter: 'blur(2px) contrast(150%)' },
          '100%': { opacity: '0', transform: 'scale(0.9)', filter: 'blur(8px) contrast(200%)' }
        },
        pulseGlow: {
          '0%, 100%': { opacity: '1' },
          '50%': { opacity: '0.6' }
        }
      }
    },
  },
  plugins: [],
}
