type EmptyStateProps = {
  title: string
  description?: string
  action?: { label: string; onClick: () => void }
  icon?: React.ReactNode
}

export function EmptyState({ title, description, action, icon }: EmptyStateProps) {
  return (
    <div className="empty-state">
      {icon ? <div className="empty-state__icon">{icon}</div> : null}
      <strong className="empty-state__title">{title}</strong>
      {description ? <p className="empty-state__desc">{description}</p> : null}
      {action ? (
        <button type="button" className="empty-state__action" onClick={action.onClick}>
          {action.label}
        </button>
      ) : null}
    </div>
  )
}
