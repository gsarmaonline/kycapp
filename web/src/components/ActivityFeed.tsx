import { useState } from 'react'
import type { ActivityEvent } from '../api'
import { actionLabel, formatRelativeTime } from '../observability'

export function ActivityFeed({
  items,
  emptyLabel = 'No activity yet',
}: {
  items: ActivityEvent[]
  emptyLabel?: string
}) {
  if (!items.length) {
    return <p className="empty-inline">{emptyLabel}</p>
  }

  return (
    <ul className="activity-feed">
      {items.map((item) => (
        <ActivityRow key={item.id} item={item} />
      ))}
    </ul>
  )
}

function ActivityRow({ item }: { item: ActivityEvent }) {
  const [open, setOpen] = useState(false)
  const hasPayload = item.payload && Object.keys(item.payload).length > 0

  return (
    <li className="activity-row">
      <div className="activity-row-main">
        <div className="activity-row-text">
          <strong>{item.summary || actionLabel(item.action)}</strong>
          <span className="activity-meta">
            <span className="activity-action">{item.action}</span>
            {item.actor_label ? <span>{item.actor_label}</span> : null}
            <time dateTime={item.created_at} title={item.created_at}>
              {formatRelativeTime(item.created_at)}
            </time>
          </span>
        </div>
        {hasPayload ? (
          <button
            type="button"
            className="button ghost activity-toggle"
            aria-expanded={open}
            onClick={() => setOpen((v) => !v)}
          >
            {open ? 'Hide' : 'Details'}
          </button>
        ) : null}
      </div>
      {open && hasPayload ? (
        <pre className="activity-payload">{JSON.stringify(item.payload, null, 2)}</pre>
      ) : null}
    </li>
  )
}
