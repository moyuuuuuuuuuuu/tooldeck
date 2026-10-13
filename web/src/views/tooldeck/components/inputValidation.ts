import type { Field } from '@/api/tooldeck'

export interface InputIssue {
  path: string
  message: string
}

export function validateToolInput(schema: Field, value: any, path = ''): InputIssue | null {
  if (schema.type === 'object') {
    for (const [name, child] of Object.entries(schema.properties || {})) {
      const childPath = path ? `${path}.${name}` : name
      const input = value?.[name]
      if (
        schema.required?.includes(name) &&
        (input == null ||
          (typeof input === 'string' && !input.trim()) ||
          (Array.isArray(input) && !input.length))
      ) {
        return { path: childPath, message: `请填写${child.title || name}` }
      }
      if (input !== undefined) {
        const issue = validateToolInput(child, input, childPath)
        if (issue) return issue
      }
    }
  }
  if (typeof value === 'string') {
    if (schema.minLength !== undefined && Array.from(value).length < schema.minLength) {
      return { path, message: `${schema.title || path}至少需要 ${schema.minLength} 个字符` }
    }
    if (schema.format === 'json') {
      try {
        JSON.parse(value)
      } catch (error) {
        return {
          path,
          message: `JSON 格式错误：${error instanceof Error ? error.message : '无效内容'}。请检查引号、逗号、括号及冒号后的值。`
        }
      }
    }
  }
  if (schema.type === 'array' && schema.items && Array.isArray(value)) {
    for (let i = 0; i < value.length; i++) {
      const issue = validateToolInput(schema.items, value[i], `${path}.${i}`)
      if (issue) return issue
    }
  }
  return null
}
