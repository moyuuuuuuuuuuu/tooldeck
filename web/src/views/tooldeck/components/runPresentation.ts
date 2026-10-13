import type { Run } from '@/api/tooldeck'
import { normalizeOutputType } from './outputPresentation'

export function presentRunOutput(run: Run | null): {
  value: any
  extra: Record<string, any> | null
} {
  const raw = run?.result
  const wrapped =
    raw !== null &&
    typeof raw === 'object' &&
    !Array.isArray(raw) &&
    Object.prototype.hasOwnProperty.call(raw, 'result')
  let value = wrapped ? raw.result : raw
  const extras = wrapped
    ? Object.fromEntries(Object.entries(raw).filter(([key]) => key !== 'result'))
    : {}
  if (
    normalizeOutputType(run?.output_type) === 'stream' &&
    value &&
    typeof value === 'object' &&
    typeof value.text === 'string'
  ) {
    const { text, ...metadata } = value
    value = text
    Object.assign(extras, metadata)
  }
  return { value, extra: Object.keys(extras).length ? extras : null }
}

export function presentRunError(run: Run | null): string {
  if (!run) return ''
  const diagnostic = run.logs?.trim()
  if (diagnostic && /^(exit status \d+|程序退出码：\d+)$/.test(run.error || ''))
    return `执行失败：${diagnostic.slice(0, 600)}`
  return run.error || ''
}
