import { useEffect, useState } from 'react'
import { listAppUserGroups, type AppUserGroup } from '../../api'

/**
 * Which groups this one extends.
 *
 * A member of this group counts as a member of every parent, so "enterprise
 * customers are also beta customers" is one checkbox rather than a duplicated
 * membership list kept in step by hand.
 *
 * The same relation roles have always had. Groups only lacked it because
 * app_role_extends got built and nothing equivalent did, which made grouping
 * mean two different things depending on which object you were looking at.
 */
export function GroupParentsField({
  orgId,
  selfId,
  value,
  onChange,
}: {
  orgId: string
  /** Omitted when creating, since a new group has no id to exclude yet. */
  selfId?: string
  value: string[]
  onChange: (next: string[]) => void
}) {
  const [groups, setGroups] = useState<AppUserGroup[]>([])

  useEffect(() => {
    let live = true
    void listAppUserGroups(orgId)
      .then((r) => live && setGroups(r.items))
      .catch(() => live && setGroups([]))
    return () => {
      live = false
    }
  }, [orgId])

  // A group cannot extend itself. The server refuses it too; leaving it out of
  // the list keeps the form from offering something it would reject.
  const options = groups.filter((g) => g.id !== selfId)
  if (options.length === 0) return null

  function toggle(id: string) {
    onChange(value.includes(id) ? value.filter((v) => v !== id) : [...value, id])
  }

  return (
    <fieldset className="group-parents">
      <legend>Extends</legend>
      <p className="group-parents-hint">
        Members of this group are treated as members of everything it extends. A
        grant written on a parent reaches them without adding anyone twice.
      </p>
      {options.map((g) => (
        <label key={g.id} className="group-parents-option">
          <input type="checkbox" checked={value.includes(g.id)} onChange={() => toggle(g.id)} />
          <span>
            <strong>{g.key}</strong>
            {g.name && g.name !== g.key && <span className="group-parents-name"> {g.name}</span>}
          </span>
        </label>
      ))}
    </fieldset>
  )
}
