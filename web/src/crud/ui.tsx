import { Link } from 'react-router-dom'
import type { ReactNode } from 'react'

export function PageHeader({
  title,
  createTo,
  createLabel,
}: {
  title: string
  createTo?: string
  createLabel?: string
}) {
  return (
    <div className="page-header">
      <h2 className="section-title">{title}</h2>
      {createTo && createLabel && (
        <Link className="button" to={createTo}>
          {createLabel}
        </Link>
      )}
    </div>
  )
}

export function ResourceTable({
  columns,
  rows,
  empty,
}: {
  columns: string[]
  rows: { key: string; cells: ReactNode[]; actions?: ReactNode }[]
  empty: string
}) {
  return (
    <div className="table-wrap">
      <table className="data-table">
        <thead>
          <tr>
            {columns.map((c) => (
              <th key={c}>{c}</th>
            ))}
            <th className="actions-col">Actions</th>
          </tr>
        </thead>
        <tbody>
          {rows.length === 0 && (
            <tr>
              <td colSpan={columns.length + 1} className="empty">
                {empty}
              </td>
            </tr>
          )}
          {rows.map((row) => (
            <tr key={row.key}>
              {row.cells.map((cell, i) => (
                <td key={i}>{cell}</td>
              ))}
              <td className="actions-col">
                <div className="row-actions">{row.actions}</div>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

export function RowActions({
  viewTo,
  editTo,
  onDelete,
  deleteDisabled,
  deleteTitle,
}: {
  viewTo: string
  editTo: string
  onDelete: () => void
  deleteDisabled?: boolean
  deleteTitle?: string
}) {
  return (
    <>
      <Link className="link-btn" to={viewTo}>
        View
      </Link>
      <Link className="link-btn" to={editTo}>
        Edit
      </Link>
      <button
        type="button"
        className="link-btn danger"
        disabled={deleteDisabled}
        title={deleteTitle}
        onClick={onDelete}
      >
        Delete
      </button>
    </>
  )
}

export function DetailList({
  items,
}: {
  items: { label: string; value: ReactNode }[]
}) {
  return (
    <dl className="detail-list">
      {items.map((item) => (
        <div key={item.label} className="detail-row">
          <dt>{item.label}</dt>
          <dd>{item.value}</dd>
        </div>
      ))}
    </dl>
  )
}

export function FormActions({
  cancelTo,
  submitLabel,
}: {
  cancelTo: string
  submitLabel: string
}) {
  return (
    <div className="form-actions">
      <Link className="ghost" to={cancelTo}>
        Cancel
      </Link>
      <button type="submit">{submitLabel}</button>
    </div>
  )
}
