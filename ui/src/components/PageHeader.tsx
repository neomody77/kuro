type PageHeaderProps = {
  title: string
  description?: string
  actions?: React.ReactNode
}

function PageHeader({ title, description, actions }: PageHeaderProps) {
  return (
    <div
      className="flex items-center justify-between px-6 h-14 shrink-0"
      style={{ borderBottom: '1px solid var(--color-border-primary)' }}
    >
      <div>
        <h1 className="text-base font-semibold" style={{ color: 'var(--color-text-primary)' }}>{title}</h1>
        {description && (
          <p className="text-xs" style={{ color: 'var(--color-text-tertiary)' }}>{description}</p>
        )}
      </div>
      {actions && <div className="flex items-center gap-2">{actions}</div>}
    </div>
  )
}

export default PageHeader
