const raw = String(import.meta.env.VITE_GIT_SHA ?? '').trim()

/** Full commit SHA when built from git; `dev` for local Vite. */
export const GIT_SHA = raw || 'dev'

/** Short SHA for UI (7 chars) or `dev`. */
export const GIT_SHA_SHORT = GIT_SHA === 'dev' ? 'dev' : GIT_SHA.slice(0, 7)
