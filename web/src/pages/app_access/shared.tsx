import { useCallback, useEffect, useState } from 'react'

/**
 * Shared plumbing for the customer access pages.
 *
 * Each page owns one object type and loads only what it needs, so a page is not
 * waiting on data it never shows.
 */
export function useResource<T>(load: () => Promise<T>, deps: unknown[]) {
  const [data, setData] = useState<T | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  const refresh = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      setData(await load())
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load')
    } finally {
      setLoading(false)
    }
  }, deps)

  useEffect(() => {
    void refresh()
  }, [refresh])

  // run wraps a mutation so every page reports failures the same way and
  // reloads on success.
  const run = useCallback(
    async (fn: () => Promise<unknown>) => {
      setError(null)
      try {
        await fn()
        await refresh()
      } catch (e) {
        setError(e instanceof Error ? e.message : 'Request failed')
      }
    },
    [refresh],
  )

  return { data, error, loading, refresh, run, setError }
}

export type Runner = (fn: () => Promise<unknown>) => Promise<void>

/**
 * A page header that says what the object is for.
 *
 * These five objects only make sense in relation to each other, and splitting
 * them across pages loses the adjacency that explained them. One line of
 * context per page carries that instead.
 */
export function AccessHeader({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <header className="page-header">
      <div>
        <h1>{title}</h1>
        <p className="muted">{children}</p>
      </div>
    </header>
  )
}
