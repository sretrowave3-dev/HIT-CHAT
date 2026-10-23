/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      fontFamily: {
        bungee: ['"Bungee"', 'sans-serif'],
        yatra: ['"Yatra One"', 'cursive', 'sans-serif'],
        mono: ['"Space Mono"', 'monospace'],
        body: ['"Mukta"', 'sans-serif'],
      },
      colors: {
        poster: {
          red: '#C8141A',
          deepred: '#8E0E12',
          mustard: '#F2A33A',
          yellow: '#F7D63C',
          green: '#2E8B3D',
          cream: '#F4E7C8',
          ink: '#2A0A08',
          cobalt: '#1F4FD8',
        }
      },
      boxShadow: {
        'hard': '4px 4px 0px #2A0A08',
        'hard-sm': '2px 2px 0px #2A0A08',
        'hard-lg': '6px 6px 0px #2A0A08',
      },
      borderWidth: {
        '2': '2px',
        '3': '3px',
      },
      borderRadius: {
        none: '0px',
        sm: '2px',
        DEFAULT: '4px',
        md: '4px',
        lg: '4px',
        xl: '4px',
        '2xl': '4px',
        '3xl': '4px',
        full: '9999px',
      }
    },
  },
  plugins: [],
}
