// Applies the operator's backdrop choice from sys_parameters (BR-1.4.6).
//
// The default image is wired statically in index.css and preloaded from
// index.html, which is what makes it paint before React has even rendered. So
// this deliberately does nothing at all in the common case: it only touches the
// DOM when the operator has actually chosen something other than the default,
// or has switched the photograph off.
//
// That asymmetry is the point. Reading the file name from an API and then
// setting it would move the image behind a network round trip and undo the
// load work — the fast path has to stay the path nobody configured.

const DEFAULT_FILE = 'backdrop.jpg'

// The server validates this too, and its check is the one that matters. This
// is here because the value ends up inside a CSS url(), and a value that
// reaches the DOM should be checked where it is used, not only where it came
// from — if a second caller ever builds this string, the guard travels with it.
const SAFE_FILE = /^[A-Za-z0-9._-]{1,80}\.(jpg|jpeg|png|webp)$/i

export type BackdropConfig = { enabled: boolean; file: string }

export function applyBackdrop(config: BackdropConfig | undefined) {
  if (!config) return
  const root = document.documentElement

  if (!config.enabled) {
    // Not `none`: the layer also carries the scrim, and removing the whole
    // stack would drop the tint that the contrast budget is measured against.
    // An empty image leaves the scrim over the flat canvas.
    root.style.setProperty('--backdrop-image', 'none')
    return
  }

  const file = config.file
  if (!file || file === DEFAULT_FILE) return
  if (!SAFE_FILE.test(file) || file.includes('..')) return

  root.style.setProperty('--backdrop-image', `url("/brand/${file}")`)
}
