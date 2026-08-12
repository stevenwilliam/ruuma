/** @type {import('tailwindcss').Config} */
// Colours come from CSS variables so light and dark are one source of truth
// (docs/10 §2). Every pair in that table is contrast-checked.
export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        bg: 'var(--bg)',
        surface: 'var(--surface)',
        border: 'var(--border)',
        body: 'var(--text)',
        muted: 'var(--text-muted)',
        primary: 'var(--primary)',
        'primary-hover': 'var(--primary-hover)',
        // Emerald as TEXT. --primary is the fill sampled from the logo (D20)
        // and stays untouched; on the ambient wash it only reaches 4.15:1, so
        // primary-coloured text uses this darker shade instead (docs/10 §2.1).
        'primary-ink': 'var(--primary-ink)',
        // Translucent, so the ambient wash reads through the cards rather
        // than only in the gutters (docs/10 §2.1). --surface stays opaque for
        // inputs and the sticky bar, which sit over scrolling content.
        card: 'var(--surface-card)',
        'primary-fg': 'var(--primary-fg)',
        'primary-subtle': 'var(--primary-subtle)',
        warning: 'var(--warning)',
        danger: 'var(--danger)',
      },
      fontFamily: {
        display: ['"Plus Jakarta Sans"', 'system-ui', 'sans-serif'],
        sans: ['Inter', 'system-ui', 'sans-serif'],
      },
      borderRadius: { xl: '0.875rem', '2xl': '1.25rem' },
    },
  },
  plugins: [],
}
