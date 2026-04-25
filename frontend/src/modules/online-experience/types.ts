import type { Group } from '@/types'

export type OnlineExperienceTab = 'chat' | 'text2img' | 'img2img'

export interface OnlineExperienceModel {
  id: string
  display_name: string
  type: 'chat' | 'image'
}

export interface OnlineExperienceChatMessage {
  role: 'user' | 'assistant'
  content: string
}

export interface OnlineExperienceImageDataItem {
  url?: string
  b64_json?: string
  revised_prompt?: string
}

export interface OnlineExperienceImageResponse {
  created?: number
  data?: OnlineExperienceImageDataItem[]
}

export interface OnlineExperiencePersistedState {
  activeTab?: OnlineExperienceTab
  groupId?: number | null
}

export type OnlineExperienceGroup = Group
