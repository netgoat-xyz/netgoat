'use client'

import { ReactNode } from 'react'
import clsx from 'clsx'

export function TableGroup({
  title,
  children,
}: {
  title?: string
  children: ReactNode
}) {
  return (
    <div className="overflow-hidden rounded-xl border border-zinc-200 bg-zinc-50 shadow-sm dark:border-zinc-800 dark:bg-zinc-900/80 dark:shadow-none">
      {title && (
        <div className="border-b border-zinc-200 bg-white px-6 py-2.5 text-sm font-semibold text-zinc-700 dark:border-zinc-800 dark:bg-zinc-900 dark:text-zinc-100">
          {title}
        </div>
      )}
      <div className="overflow-x-auto">
        <table className="min-w-full text-sm">
          {children}
        </table>
      </div>
    </div>
  )
}

export function TableHead({ children }: { children: ReactNode }) {
  return (
    <thead className="bg-white dark:bg-zinc-900 border-b border-zinc-200 dark:border-zinc-800">
      {children}
    </thead>
  )
}

export function TableBody({ children }: { children: ReactNode }) {
  return <tbody className="divide-y divide-zinc-100 dark:divide-zinc-800 dark:bg-zinc-900">{children}</tbody>
}

export function TableRow({ children }: { children: ReactNode }) {
  return <tr className="hover:bg-zinc-100 dark:hover:bg-zinc-800 transition">{children}</tr>
}

export function TableHeaderCell({
  children,
  className,
}: {
  children: ReactNode
  className?: string
}) {
  return (
    <th
      scope="col"
      className={clsx(
        'px-6 py-2.5 text-left font-semibold text-zinc-700 dark:text-zinc-200',
        className,
      )}
    >
      {children}
    </th>
  )
}
export function TableCell({
  children,
  className,
}: {
  children: ReactNode
  className?: string
}) {
  return (
    <td className={clsx('px-6 py-4 text-zinc-700 dark:text-zinc-100', className)}>
      {children}
    </td>
  )
}

