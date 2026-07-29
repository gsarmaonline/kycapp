import type { DailyUsagePoint } from '../observability'

type SeriesKey = 'allowed' | 'denied'

const SERIES: { key: SeriesKey; label: string; colorVar: string }[] = [
  { key: 'allowed', label: 'Allowed', colorVar: 'var(--app-accent)' },
  { key: 'denied', label: 'Denied', colorVar: 'var(--app-danger)' },
]

export function StackedBarChart({
  points,
  height = 140,
  emptyLabel = 'No checks in this period',
}: {
  points: DailyUsagePoint[]
  height?: number
  emptyLabel?: string
}) {
  const max = Math.max(1, ...points.map((p) => p.total))
  const hasData = points.some((p) => p.total > 0)

  if (!hasData) {
    return <p className="chart-empty">{emptyLabel}</p>
  }

  const padX = 8
  const padTop = 8
  const padBottom = 22
  const chartH = height - padTop - padBottom
  const n = Math.max(points.length, 1)
  const gap = 2
  const barW = Math.max(4, (100 - padX * 2) / n - gap)

  return (
    <div className="chart-wrap">
      <svg
        className="stacked-bar-chart"
        viewBox={`0 0 100 ${height}`}
        role="img"
        aria-label="Entitlement checks by day"
        preserveAspectRatio="none"
      >
        {points.map((p, i) => {
          const x = padX + i * (barW + gap)
          let y = padTop + chartH
          const slices: { key: SeriesKey; h: number; y: number }[] = []
          for (const s of SERIES) {
            const value = p[s.key]
            if (value <= 0) continue
            const h = (value / max) * chartH
            y -= h
            slices.push({ key: s.key, h, y })
          }
          return (
            <g key={p.day}>
              {slices.map((sl) => {
                const color = SERIES.find((s) => s.key === sl.key)?.colorVar ?? 'currentColor'
                return (
                  <rect
                    key={sl.key}
                    x={x}
                    y={sl.y}
                    width={barW}
                    height={Math.max(sl.h, 0.4)}
                    fill={color}
                    rx={0.6}
                  >
                    <title>
                      {p.day}: {sl.key} {p[sl.key]}
                    </title>
                  </rect>
                )
              })}
            </g>
          )
        })}
      </svg>
      <div className="chart-x-labels" aria-hidden>
        <span>{points[0]?.day.slice(5)}</span>
        <span>{points[points.length - 1]?.day.slice(5)}</span>
      </div>
      <ul className="chart-legend">
        {SERIES.map((s) => (
          <li key={s.key}>
            <span className="chart-swatch" style={{ background: s.colorVar }} />
            {s.label}
          </li>
        ))}
      </ul>
    </div>
  )
}
