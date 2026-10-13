import assert from 'node:assert/strict'
import { validateToolInput } from '../src/views/tooldeck/components/inputValidation'
import { presentRunOutput, presentRunError } from '../src/views/tooldeck/components/runPresentation'
import type { Run, Field } from '../src/api/tooldeck'

const schema: Field = {
  type: 'object',
  required: ['text', 'enabled', 'count'],
  properties: {
    text: { type: 'string', title: '文本' },
    enabled: { type: 'boolean' },
    count: { type: 'number' }
  }
}
assert.equal(validateToolInput(schema, { text: '', enabled: false, count: 0 })?.path, 'text')
assert.equal(validateToolInput(schema, { text: '   ', enabled: false, count: 0 })?.path, 'text')
assert.equal(validateToolInput(schema, { text: 'hello', enabled: false, count: 0 }), null)
const nested: Field = {
  type: 'object',
  properties: {
    settings: { type: 'object', required: ['name'], properties: { name: { type: 'string' } } }
  }
}
assert.equal(validateToolInput(nested, { settings: {} })?.path, 'settings.name')
assert.match(
  validateToolInput({ type: 'string', format: 'json' }, '{"a":}', 'text')!.message,
  /JSON 格式错误/
)
assert.equal(validateToolInput({ type: 'string', format: 'json' }, '{"a":1}', 'text'), null)
const run = (result: any, output_type = 'stream') => ({ result, output_type }) as Run
assert.equal(presentRunOutput(run({ text: '你好' })).value, '你好')
assert.equal(presentRunOutput(run({ result: '你好' })).value, '你好')
assert.equal(presentRunOutput(run('你好')).value, '你好')
assert.equal(presentRunOutput(run({ text: '你好' }, 'text/event-stream')).value, '你好')
assert.deepEqual(presentRunOutput(run({ text: '你好', count: 2 })).extra, { count: 2 })
assert.deepEqual(presentRunOutput(run({ text: '你好' }, 'json')).value, { text: '你好' })
assert.match(
  presentRunError({
    error: 'exit status 1',
    logs: 'Syntax error',
    input: { text: '{"a":}' }
  } as unknown as Run),
  /Syntax error/
)
assert.match(
  presentRunError({
    error: 'exit status 1',
    logs: 'Dependency unavailable',
    input: {}
  } as unknown as Run),
  /Dependency unavailable/
)
assert.equal(presentRunError({ error: '运行超时', logs: 'secret detail' } as Run), '运行超时')
console.log('TD02/TD03/TD04 presentation and input regressions passed')
