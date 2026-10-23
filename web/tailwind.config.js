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
          red: '#D7191F',
          deepred: '#8E0E12',
          mustard: '#F2A33A',
          yellow: '#F7D63C',
          green: '#2E8B3D',
          cream: '#F6EACB',
          ink: '#2A0A08',
          cobalt: '#2457E0',
        }
      },
      boxShadow: {
        'hard': '3px 3px 0px #2A0A08',
        'hard-sm': '2px 2px 0px #2A0A08',
        'hard-lg': '5px 5px 0px #2A0A08',
      },
      borderWidth: {
        '2': '2px',
        '3': '3px',
      },
      borderRadius: {
        'asym-lg': '28px 6px 28px 6px',
        'asym-md': '20px 6px 20px 6px',
        'asym-sm': '12px 4px 12px 4px',
        'bubble-me': '24px 6px 24px 24px',
        'bubble-peer': '6px 24px 24px 24px',
      }
    },
  },
  plugins: [],
}
