// Shared primitives. Every interactive element keeps a visible focus ring and a
// 44px touch target (docs/10 §4).

import { type ReactNode, type InputHTMLAttributes, type ButtonHTMLAttributes, useId } from 'react'

export function Button({
  variant = 'primary',
  className = '',
  children,
  ...rest
}: ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: 'primary' | 'secondary' | 'ghost' | 'danger'
}) {
  const base =
    'inline-flex min-h-[44px] items-center justify-center gap-2 rounded-xl px-4 py-2 text-sm font-medium transition-colors disabled:cursor-not-allowed disabled:opacity-50'
  const styles = {
    primary: 'bg-primary text-primary-fg hover:bg-primary-hover',
    secondary: 'border border-border bg-surface text-body hover:bg-primary-subtle',
    ghost: 'text-primary hover:bg-primary-subtle',
    danger: 'bg-danger text-white hover:opacity-90',
  }[variant]
  return (
    <button className={`${base} ${styles} ${className}`} {...rest}>
      {children}
    </button>
  )
}

export function Card({ children, className = '' }: { children: ReactNode; className?: string }) {
  return (
    <div className={`rounded-2xl border border-border bg-surface p-4 ${className}`}>{children}</div>
  )
}

export function Field({
  label,
  hint,
  error,
  children,
}: {
  label: string
  hint?: string
  error?: string
  children: (id: string, describedBy?: string) => ReactNode
}) {
  const id = useId()
  const hintId = hint ? `${id}-hint` : undefined
  const errorId = error ? `${id}-error` : undefined
  const describedBy = [hintId, errorId].filter(Boolean).join(' ') || undefined

  return (
    <div className="flex flex-col gap-1.5">
      <label htmlFor={id} className="text-sm font-medium text-body">
        {label}
      </label>
      {children(id, describedBy)}
      {hint && (
        <p id={hintId} className="text-xs text-muted">
          {hint}
        </p>
      )}
      {/* Errors are announced, not just coloured (docs/10 §5). */}
      {error && (
        <p id={errorId} role="alert" className="text-xs font-medium text-danger">
          {error}
        </p>
      )}
    </div>
  )
}

export function TextInput(props: InputHTMLAttributes<HTMLInputElement>) {
  return (
    <input
      {...props}
      className={`min-h-[44px] rounded-xl border border-border bg-surface px-3 py-2 text-sm text-body placeholder:text-muted ${props.className ?? ''}`}
    />
  )
}

export function Badge({
  tone = 'neutral',
  children,
}: {
  tone?: 'neutral' | 'primary' | 'warning' | 'danger'
  children: ReactNode
}) {
  const styles = {
    neutral: 'border-border text-muted',
    primary: 'border-primary bg-primary-subtle text-primary',
    warning: 'border-warning text-warning',
    danger: 'border-danger text-danger',
  }[tone]
  return (
    <span className={`inline-flex items-center rounded-full border px-2 py-0.5 text-xs ${styles}`}>
      {children}
    </span>
  )
}

export function Spinner({ label }: { label: string }) {
  return (
    <p role="status" className="py-8 text-center text-sm text-muted">
      {label}
    </p>
  )
}

export function EmptyState({ children }: { children: ReactNode }) {
  return (
    <div className="rounded-2xl border border-dashed border-border p-8 text-center text-sm text-muted">
      {children}
    </div>
  )
}

export function ErrorNote({ children }: { children: ReactNode }) {
  return (
    <div role="alert" className="rounded-xl border border-danger/40 bg-danger/5 p-3 text-sm text-danger">
      {children}
    </div>
  )
}
