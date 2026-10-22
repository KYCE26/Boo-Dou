import { requestJSON } from './http'

export interface Health {
  status: string
  llm_configured: boolean
  llm_vision: boolean
}

export const getHealth = () => requestJSON<Health>('GET', '/api/health')
