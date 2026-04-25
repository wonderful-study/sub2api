import { apiClient } from '@/api/client'
import { getLocale } from '@/i18n'
import type {
  OnlineExperienceGroup,
  OnlineExperienceImageResponse,
  OnlineExperienceModel
} from './types'

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || '/api/v1'

const buildAuthHeaders = (headers?: Record<string, string>): HeadersInit => {
  const token = localStorage.getItem('auth_token')
  return {
    ...(token ? { Authorization: `Bearer ${token}` } : {}),
    'Accept-Language': getLocale(),
    ...headers
  }
}

const extractErrorMessage = async (response: Response): Promise<string> => {
  try {
    const data = await response.json() as {
      message?: string
      error?: { message?: string }
    }
    return data.error?.message || data.message || `${response.status} ${response.statusText}`
  } catch {
    return `${response.status} ${response.statusText}`
  }
}

const readJson = async <T>(response: Response): Promise<T> => {
  if (!response.ok) {
    throw new Error(await extractErrorMessage(response))
  }
  return await response.json() as T
}

export async function listGroups(): Promise<OnlineExperienceGroup[]> {
  const { data } = await apiClient.get<OnlineExperienceGroup[]>('/online-experience/groups')
  return data
}

export async function listModels(groupId: number): Promise<OnlineExperienceModel[]> {
  const { data } = await apiClient.get<OnlineExperienceModel[]>('/online-experience/models', {
    params: { group_id: groupId }
  })
  return data
}

export async function streamChat(
  groupId: number,
  payload: Record<string, unknown>,
  onDelta: (delta: string) => void,
  signal?: AbortSignal
): Promise<unknown> {
  const response = await fetch(`${API_BASE_URL}/online-experience/chat`, {
    method: 'POST',
    headers: buildAuthHeaders({
      'Content-Type': 'application/json'
    }),
    body: JSON.stringify({
      ...payload,
      group_id: groupId,
      stream: true
    }),
    signal
  })

  if (!response.ok) {
    throw new Error(await extractErrorMessage(response))
  }

  if (!response.body) {
    return null
  }

  const reader = response.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''
  let finalPayload: unknown = null

  while (true) {
    const { done, value } = await reader.read()
    if (done) {
      break
    }

    buffer += decoder.decode(value, { stream: true })
    const lines = buffer.split('\n')
    buffer = lines.pop() || ''

    for (const line of lines) {
      if (!line.startsWith('data:')) {
        continue
      }

      const chunk = line.slice(5).trim()
      if (!chunk || chunk === '[DONE]') {
        continue
      }

      try {
        const event = JSON.parse(chunk) as {
          choices?: Array<{
            delta?: { content?: string }
            message?: { content?: string }
          }>
        }
        finalPayload = event

        const delta = event.choices?.[0]?.delta?.content ?? event.choices?.[0]?.message?.content ?? ''
        if (delta) {
          onDelta(delta)
        }
      } catch {
        // Ignore non-JSON SSE comments or incomplete chunks.
      }
    }
  }

  return finalPayload
}

export async function generateImage(
  groupId: number,
  payload: Record<string, unknown>,
  signal?: AbortSignal
): Promise<OnlineExperienceImageResponse> {
  const response = await fetch(`${API_BASE_URL}/online-experience/images/generations`, {
    method: 'POST',
    headers: buildAuthHeaders({
      'Content-Type': 'application/json'
    }),
    body: JSON.stringify({
      ...payload,
      group_id: groupId
    }),
    signal
  })

  return readJson<OnlineExperienceImageResponse>(response)
}

export async function editImage(
  groupId: number,
  payload: FormData,
  signal?: AbortSignal
): Promise<OnlineExperienceImageResponse> {
  const formData = new FormData()
  formData.append('group_id', String(groupId))
  for (const [key, value] of payload.entries()) {
    formData.append(key, value)
  }

  const response = await fetch(`${API_BASE_URL}/online-experience/images/edits`, {
    method: 'POST',
    headers: buildAuthHeaders(),
    body: formData,
    signal
  })

  return readJson<OnlineExperienceImageResponse>(response)
}

export const onlineExperienceAPI = {
  listGroups,
  listModels,
  streamChat,
  generateImage,
  editImage
}
