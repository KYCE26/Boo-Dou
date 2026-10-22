// JS-driven animations (WAAPI, FLIP, view transitions) bypass the CSS
// reduced-motion rule in base.css, so each one checks this first.
export function prefersReducedMotion(): boolean {
  return window.matchMedia('(prefers-reduced-motion: reduce)').matches
}
