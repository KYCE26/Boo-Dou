import { requestJSON } from './http'

export interface Attachment {
  id: string
  mime: string
  width: number
  height: number
}

export type MessageStatus = 'complete' | 'stopped' | 'error'

export interface StoredMessage {
  id: string
  role: 'user' | 'assistant'
  content: string
  status: MessageStatus
  model: string
  created_at: string
  attachments: Attachment[]
}

export interface Conversation {
  id: string
  title: string
  created_at: string
  updated_at: string
}

export interface ConversationDetail extends Conversation {
  messages: StoredMessage[]
}

export const listConversations = () => requestJSON<Conversation[]>('GET', '/api/conversations')

export const createConversation = () => requestJSON<Conversation>('POST', '/api/conversations')

export const getConversation = (id: string) =>
  requestJSON<ConversationDetail>('GET', `/api/conversations/${encodeURIComponent(id)}`)

export const renameConversation = (id: string, title: string) =>
  requestJSON<Conversation>('PATCH', `/api/conversations/${encodeURIComponent(id)}`, { title })

export const deleteConversation = (id: string) =>
  requestJSON<void>('DELETE', `/api/conversations/${encodeURIComponent(id)}`)

export const attachmentUrl = (id: string) => `/api/attachments/${encodeURIComponent(id)}`
