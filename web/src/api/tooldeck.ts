import request from '@/utils/http'
import { useUserStore } from '@/store/modules/user'

export interface Field {
  type: string; title?: string; description?: string; format?: string; properties?: Record<string, Field>
  items?: Field; required?: string[]; enum?: any[]; default?: any; minimum?: number; maximum?: number
  minLength?: number; maxLength?: number; minItems?: number; maxItems?: number
}
export interface Tool {
 build_status?:string;build_log?:string;build_error?:string;
 withdrawn?:boolean; created_at?:string; public?:boolean; review_status?:string;review_note?:string;api_enabled?: boolean; notify_result?:boolean; owner?:string
  id: string
  manifest: { env?: {name:string;description?:string;required?:boolean;sensitive?:boolean}[]; runtime_version?:string;name: string; title: string; version: string; description: string; runtime: string
    execution: {stream?:boolean;mode: string; timeout_seconds: number; memory_mb: number}
    input_schema: Field; ui_schema: Record<string, any>; output_schema: {type: string}
    network: {enabled: boolean; allowed_hosts: string[]}; secrets: string[] }
}
export interface Artifact { file_id: string; name: string; mime: string; size: number }
export interface Run { run_id: string; tool_id: string; status: string; input: Record<string, any>; result: any; error?: string; logs: string; duration_ms: number; artifacts: Artifact[]; created_at: string }
export const toolApiPrefix = () => useUserStore().isLogin ? '/v1' : '/public'
export const td = {
  tools: () => request.get<Tool[]>({url: `${toolApiPrefix()}/tools`}),
  runs: () => request.get<Run[]>({url: '/v1/runs?mine=1'}),
  run: (id: string) => request.get<Run>({url: `${toolApiPrefix()}/runs/${id}`}),
  execute: (id: string, input: any) => request.post<Run>({url: `${toolApiPrefix()}/tools/${id}/runs`, data: {input}, timeout: 25000, headers: {'Idempotency-Key': Array.from(crypto.getRandomValues(new Uint8Array(16)),v=>v.toString(16).padStart(2,'0')).join('')}}),
  cancel: (id: string) => request.post<Run>({url: `${toolApiPrefix()}/runs/${id}/cancel`}),
  upload: (file: File, kind = 'files', mode = '', options: Record<string,string> = {}) => {const data = new FormData(); data.append('file', file); if (mode) data.append('mode',mode); for(const [key,value] of Object.entries(options))data.append(key,value); return request.post<any>({url: `${kind==='files'?toolApiPrefix():'/v1'}/${kind}`, data, timeout: 120000})},
  list: (kind: string) => request.get<any[]>({url: `/v1/${kind}`}),
  save: (kind: string, data: any) => request.post<any>({url: `/v1/${kind}`, data}),
  remove: (kind: string, id: string) => request.del({url: `/v1/${kind}/${encodeURIComponent(id)}`}),
  blob: async (id: string) => {
    const response = await fetch(`${import.meta.env.VITE_API_URL}${toolApiPrefix()}/files/${id}`, {headers: {Authorization: `Bearer ${useUserStore().accessToken}`}})
    if (!response.ok) throw new Error('文件读取失败或授权已过期')
    return response.blob()
  }
}
