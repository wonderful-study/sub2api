<template>
  <AppLayout>
    <div class="space-y-6">
      <section class="overflow-hidden rounded-[28px] border border-gray-200/70 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-950">
        <div class="relative overflow-hidden px-6 py-6 sm:px-8">
          <div class="pointer-events-none absolute inset-0 bg-[radial-gradient(circle_at_top_left,_rgba(59,130,246,0.14),_transparent_42%),radial-gradient(circle_at_bottom_right,_rgba(16,185,129,0.14),_transparent_38%)]"></div>
          <div class="pointer-events-none absolute -left-16 top-0 h-40 w-40 rounded-full bg-sky-500/10 blur-3xl"></div>
          <div class="pointer-events-none absolute right-0 top-10 h-32 w-32 rounded-full bg-emerald-500/10 blur-3xl"></div>

          <div class="relative flex flex-col gap-6 xl:flex-row xl:items-end xl:justify-between">
            <div class="max-w-3xl space-y-4">
              <span class="inline-flex items-center gap-2 rounded-full border border-primary-200/70 bg-primary-50 px-3 py-1 text-xs font-semibold uppercase tracking-[0.18em] text-primary-700 dark:border-primary-500/20 dark:bg-primary-500/10 dark:text-primary-300">
                <Icon name="sparkles" size="sm" />
                {{ t('onlineExperience.badge') }}
              </span>
              <div class="space-y-2">
                <h1 class="text-3xl font-black tracking-tight text-gray-900 dark:text-white sm:text-[2.35rem]">
                  {{ t('onlineExperience.title') }}
                </h1>
                <p class="max-w-2xl text-sm leading-7 text-gray-600 dark:text-dark-300 sm:text-base">
                  {{ t('onlineExperience.description') }}
                </p>
              </div>

              <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
                <div class="rounded-2xl border border-white/70 bg-white/80 px-4 py-3 backdrop-blur dark:border-dark-700 dark:bg-dark-900/80">
                  <p class="text-[11px] font-semibold uppercase tracking-[0.14em] text-gray-500 dark:text-dark-400">{{ t('onlineExperience.balanceLabel') }}</p>
                  <p class="mt-1 text-lg font-bold text-gray-900 dark:text-white">{{ formattedBalance }}</p>
                </div>
                <div class="rounded-2xl border border-white/70 bg-white/80 px-4 py-3 backdrop-blur dark:border-dark-700 dark:bg-dark-900/80">
                  <p class="text-[11px] font-semibold uppercase tracking-[0.14em] text-gray-500 dark:text-dark-400">{{ t('onlineExperience.currentGroup') }}</p>
                  <p class="mt-1 truncate text-sm font-semibold text-gray-900 dark:text-white">{{ selectedGroup?.name || t('onlineExperience.noGroup') }}</p>
                </div>
                <div class="rounded-2xl border border-white/70 bg-white/80 px-4 py-3 backdrop-blur dark:border-dark-700 dark:bg-dark-900/80">
                  <p class="text-[11px] font-semibold uppercase tracking-[0.14em] text-gray-500 dark:text-dark-400">{{ t('onlineExperience.chatModels') }}</p>
                  <p class="mt-1 text-lg font-bold text-gray-900 dark:text-white">{{ chatModels.length }}</p>
                </div>
                <div class="rounded-2xl border border-white/70 bg-white/80 px-4 py-3 backdrop-blur dark:border-dark-700 dark:bg-dark-900/80">
                  <p class="text-[11px] font-semibold uppercase tracking-[0.14em] text-gray-500 dark:text-dark-400">{{ t('onlineExperience.imageModels') }}</p>
                  <p class="mt-1 text-lg font-bold text-gray-900 dark:text-white">{{ imageModels.length }}</p>
                </div>
              </div>
            </div>

            <div class="w-full max-w-xl rounded-3xl border border-gray-200/80 bg-white/90 p-5 shadow-sm backdrop-blur dark:border-dark-700 dark:bg-dark-900/90">
              <div class="flex items-center justify-between gap-3">
                <div>
                  <p class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('onlineExperience.groupLabel') }}</p>
                  <p class="mt-1 text-xs leading-6 text-gray-500 dark:text-dark-400">{{ currentGroupHint }}</p>
                </div>
                <span v-if="loadingModels" class="inline-flex items-center gap-2 rounded-full bg-gray-100 px-3 py-1 text-xs font-medium text-gray-500 dark:bg-dark-800 dark:text-dark-300">
                  <LoadingSpinner size="sm" color="secondary" />
                  {{ t('onlineExperience.loadingModels') }}
                </span>
              </div>

              <div class="mt-4">
                <Select
                  :model-value="selectedGroupId"
                  :options="groupOptions"
                  searchable
                  :placeholder="t('onlineExperience.selectGroup')"
                  @update:model-value="handleGroupChange"
                />
              </div>
            </div>
          </div>
        </div>
      </section>

      <div v-if="loading" class="flex items-center justify-center py-16">
        <LoadingSpinner size="xl" />
      </div>

      <EmptyState
        v-else-if="!groups.length"
        :title="t('onlineExperience.emptyTitle')"
        :description="t('onlineExperience.emptyDescription')"
        action-to="/subscriptions"
        :action-text="t('onlineExperience.emptyAction')"
      >
        <template #icon>
          <div class="flex h-20 w-20 items-center justify-center rounded-2xl bg-primary-50 text-primary-600 dark:bg-primary-500/10 dark:text-primary-300">
            <Icon name="sparkles" size="xl" />
          </div>
        </template>
      </EmptyState>

      <div v-else class="grid gap-6 xl:grid-cols-[minmax(0,380px)_minmax(0,1fr)]">
        <section class="card overflow-hidden">
          <div class="border-b border-gray-100 px-6 py-5 dark:border-dark-700">
            <div class="flex flex-wrap items-center gap-2">
              <button
                v-for="tab in tabs"
                :key="tab.value"
                type="button"
                class="inline-flex items-center gap-2 rounded-full px-4 py-2 text-sm font-semibold transition-all"
                :class="activeTab === tab.value ? 'bg-primary-600 text-white shadow-glow' : 'bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-dark-800 dark:text-dark-300 dark:hover:bg-dark-700'"
                @click="activeTab = tab.value"
              >
                <Icon :name="tab.icon" size="sm" />
                {{ tab.label }}
              </button>
            </div>
          </div>

          <div class="space-y-5 p-6">
            <template v-if="activeTab === 'chat'">
              <div class="space-y-2">
                <label class="input-label">{{ t('onlineExperience.modelLabel') }}</label>
                <Select
                  :model-value="selectedChatModel"
                  :options="chatModelOptions"
                  :placeholder="t('onlineExperience.selectModel')"
                  @update:model-value="selectedChatModel = String($event || '')"
                />
              </div>

              <TextArea
                v-model="systemPrompt"
                :label="t('onlineExperience.systemPrompt')"
                :rows="3"
                :placeholder="t('onlineExperience.systemPromptPlaceholder')"
              />

              <TextArea
                v-model="chatInput"
                :label="t('onlineExperience.chatPrompt')"
                :rows="8"
                :placeholder="t('onlineExperience.chatPlaceholder')"
              />

              <div class="flex flex-wrap gap-3">
                <button class="btn btn-primary" :disabled="!canSendChat" @click="sendChat">
                  <Icon name="play" size="sm" class="mr-2" />
                  {{ t('onlineExperience.send') }}
                </button>
                <button class="btn btn-secondary" :disabled="!chatLoading" @click="stopChat">
                  <Icon name="x" size="sm" class="mr-2" />
                  {{ t('onlineExperience.stop') }}
                </button>
                <button class="btn btn-secondary" :disabled="!canRetryChat" @click="retryLastReply">
                  <Icon name="refresh" size="sm" class="mr-2" />
                  {{ t('onlineExperience.retry') }}
                </button>
                <button class="btn btn-secondary" :disabled="!chatMessages.length" @click="clearChat">
                  <Icon name="trash" size="sm" class="mr-2" />
                  {{ t('onlineExperience.clearConversation') }}
                </button>
              </div>
            </template>

            <template v-else-if="activeTab === 'text2img'">
              <div class="space-y-2">
                <label class="input-label">{{ t('onlineExperience.imageModelLabel') }}</label>
                <Select
                  :model-value="selectedImageModel"
                  :options="imageModelOptions"
                  :placeholder="t('onlineExperience.selectModel')"
                  @update:model-value="selectedImageModel = String($event || '')"
                />
              </div>

              <div class="space-y-3">
                <label class="input-label">{{ t('onlineExperience.aspectRatio') }}</label>
                <div class="grid grid-cols-3 gap-3">
                  <button
                    v-for="option in aspectOptions"
                    :key="option.value"
                    type="button"
                    class="rounded-2xl border px-4 py-4 text-center transition-all"
                    :class="text2imgSize === option.value ? 'border-primary-500 bg-primary-50 text-primary-700 dark:bg-primary-500/10 dark:text-primary-300' : 'border-gray-200 bg-white text-gray-600 hover:border-primary-300 hover:text-primary-600 dark:border-dark-700 dark:bg-dark-900 dark:text-dark-300'"
                    @click="text2imgSize = option.value"
                  >
                    <div class="mx-auto mb-2 rounded-md border border-current/20 bg-current/10" :class="option.previewClass"></div>
                    <div class="text-sm font-semibold">{{ option.label }}</div>
                  </button>
                </div>
              </div>

              <div class="grid gap-4 sm:grid-cols-2">
                <div class="space-y-2">
                  <label class="input-label">{{ t('onlineExperience.imageCount') }}</label>
                  <Select
                    :model-value="text2imgCount"
                    :options="imageCountOptions"
                    @update:model-value="text2imgCount = Number($event || 1)"
                  />
                </div>
                <div class="space-y-2">
                  <label class="input-label">{{ t('onlineExperience.qualityLabel') }}</label>
                  <Select
                    :model-value="text2imgQuality"
                    :options="qualityOptions"
                    @update:model-value="text2imgQuality = String($event || 'high')"
                  />
                </div>
              </div>

              <TextArea
                v-model="text2imgPrompt"
                :label="t('onlineExperience.imagePrompt')"
                :rows="9"
                :placeholder="t('onlineExperience.imagePromptPlaceholder')"
              />

              <button class="btn btn-primary w-full" :disabled="!canGenerateText2Img" @click="submitTextToImage">
                <Icon name="sparkles" size="sm" class="mr-2" />
                {{ text2imgLoading ? t('onlineExperience.generatingAction') : t('onlineExperience.generateImage') }}
              </button>
            </template>

            <template v-else>
              <div class="space-y-2">
                <label class="input-label">{{ t('onlineExperience.imageModelLabel') }}</label>
                <Select
                  :model-value="selectedImageModel"
                  :options="imageModelOptions"
                  :placeholder="t('onlineExperience.selectModel')"
                  @update:model-value="selectedImageModel = String($event || '')"
                />
              </div>

              <div class="rounded-3xl border border-dashed border-gray-300 bg-gray-50/80 p-5 dark:border-dark-600 dark:bg-dark-900/60">
                <div class="flex flex-wrap items-center justify-between gap-3">
                  <div>
                    <p class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('onlineExperience.uploadTitle') }}</p>
                    <p class="mt-1 text-xs leading-6 text-gray-500 dark:text-dark-400">{{ t('onlineExperience.uploadHint') }}</p>
                  </div>
                  <button type="button" class="btn btn-secondary" @click="openUploadDialog">
                    <Icon name="upload" size="sm" class="mr-2" />
                    {{ t('onlineExperience.uploadImage') }}
                  </button>
                </div>
                <input ref="uploadInputRef" type="file" accept="image/*" multiple class="hidden" @change="handleUploadChange" />

                <div v-if="uploadItems.length" class="mt-4 grid gap-3 sm:grid-cols-2">
                  <div
                    v-for="item in uploadItems"
                    :key="item.id"
                    class="overflow-hidden rounded-2xl border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900"
                  >
                    <img :src="item.previewUrl" alt="" class="h-40 w-full object-cover" />
                    <div class="flex items-center justify-between gap-3 px-4 py-3">
                      <div class="min-w-0">
                        <p class="truncate text-sm font-medium text-gray-900 dark:text-white">{{ item.file.name }}</p>
                        <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ formatFileSize(item.file.size) }}</p>
                      </div>
                      <button type="button" class="rounded-full p-2 text-gray-400 transition hover:bg-gray-100 hover:text-red-500 dark:hover:bg-dark-800" @click="removeUpload(item.id)">
                        <Icon name="trash" size="sm" />
                      </button>
                    </div>
                  </div>
                </div>
              </div>

              <div class="grid gap-4 sm:grid-cols-2">
                <div class="space-y-2">
                  <label class="input-label">{{ t('onlineExperience.aspectRatio') }}</label>
                  <Select
                    :model-value="img2imgSize"
                    :options="aspectSelectOptions"
                    @update:model-value="img2imgSize = String($event || '1024x1024')"
                  />
                </div>
                <div class="space-y-2">
                  <label class="input-label">{{ t('onlineExperience.qualityLabel') }}</label>
                  <Select
                    :model-value="img2imgQuality"
                    :options="qualityOptions"
                    @update:model-value="img2imgQuality = String($event || 'high')"
                  />
                </div>
              </div>

              <TextArea
                v-model="img2imgPrompt"
                :label="t('onlineExperience.editPrompt')"
                :rows="7"
                :placeholder="t('onlineExperience.editPromptPlaceholder')"
              />

              <button class="btn btn-primary w-full" :disabled="!canGenerateImg2Img" @click="submitImageEdit">
                <Icon name="sparkles" size="sm" class="mr-2" />
                {{ img2imgLoading ? t('onlineExperience.generatingAction') : t('onlineExperience.generateEditedImage') }}
              </button>
            </template>
          </div>
        </section>

        <section class="card overflow-hidden">
          <template v-if="activeTab === 'chat'">
            <div class="border-b border-gray-100 px-6 py-5 dark:border-dark-700">
              <div class="flex flex-wrap items-center justify-between gap-3">
                <div>
                  <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('onlineExperience.conversationPanel') }}</h2>
                  <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ selectedChatModelLabel }}</p>
                </div>
                <span v-if="chatLoading" class="inline-flex items-center gap-2 rounded-full bg-primary-50 px-3 py-1 text-xs font-medium text-primary-700 dark:bg-primary-500/10 dark:text-primary-300">
                  <LoadingSpinner size="sm" />
                  {{ t('onlineExperience.generating') }}
                </span>
              </div>
            </div>

            <div ref="chatScrollRef" class="flex min-h-[640px] flex-1 flex-col gap-4 overflow-y-auto p-6">
              <template v-if="chatMessages.length">
                <article
                  v-for="message in chatMessages"
                  :key="message.id"
                  class="max-w-[88%] rounded-3xl px-5 py-4"
                  :class="message.role === 'user' ? 'ml-auto bg-primary-600 text-white' : 'bg-gray-50 text-gray-900 dark:bg-dark-900 dark:text-white'"
                >
                  <div class="mb-2 flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.14em]" :class="message.role === 'user' ? 'text-primary-100' : 'text-gray-400 dark:text-dark-400'">
                    <Icon :name="message.role === 'user' ? 'user' : 'sparkles'" size="xs" />
                    {{ message.role === 'user' ? t('onlineExperience.userRole') : t('onlineExperience.assistantRole') }}
                  </div>

                  <div v-if="message.role === 'assistant'" class="online-experience-markdown text-sm leading-7" v-html="renderMarkdown(message.content || (message.pending ? t('onlineExperience.generating') : ''))"></div>
                  <p v-else class="whitespace-pre-wrap text-sm leading-7">{{ message.content }}</p>
                </article>
              </template>

              <div v-else class="grid gap-4 md:grid-cols-2">
                <button
                  v-for="suggestion in chatSuggestions"
                  :key="suggestion.title"
                  type="button"
                  class="rounded-3xl border border-gray-200 bg-white p-5 text-left transition-all hover:-translate-y-0.5 hover:border-primary-300 hover:shadow-sm dark:border-dark-700 dark:bg-dark-900 dark:hover:border-primary-500/50"
                  @click="chatInput = suggestion.prompt"
                >
                  <div class="flex items-center gap-3">
                    <span class="inline-flex h-11 w-11 items-center justify-center rounded-2xl bg-primary-50 text-primary-600 dark:bg-primary-500/10 dark:text-primary-300">
                      <Icon :name="suggestion.icon" size="sm" />
                    </span>
                    <div>
                      <p class="text-sm font-semibold text-gray-900 dark:text-white">{{ suggestion.title }}</p>
                      <p class="mt-1 text-xs leading-6 text-gray-500 dark:text-dark-400">{{ suggestion.prompt }}</p>
                    </div>
                  </div>
                </button>
              </div>
            </div>
          </template>

          <template v-else>
            <div class="border-b border-gray-100 px-6 py-5 dark:border-dark-700">
              <div class="flex flex-wrap items-center justify-between gap-3">
                <div>
                  <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('onlineExperience.imagePanel') }}</h2>
                  <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ selectedImageModelLabel }}</p>
                </div>
                <span v-if="currentImageLoading" class="inline-flex items-center gap-2 rounded-full bg-primary-50 px-3 py-1 text-xs font-medium text-primary-700 dark:bg-primary-500/10 dark:text-primary-300">
                  <LoadingSpinner size="sm" />
                  {{ t('onlineExperience.generating') }}
                </span>
              </div>
            </div>

            <div class="min-h-[640px] p-6">
              <div v-if="currentImageResults.length" class="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
                <article
                  v-for="(result, index) in currentImageResults"
                  :key="`${activeTab}-${index}`"
                  class="overflow-hidden rounded-3xl border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900"
                >
                  <img :src="result.url" alt="" class="aspect-[4/4.8] w-full object-cover" />
                  <div class="space-y-3 px-5 py-4">
                    <div class="flex items-center justify-between gap-3">
                      <p class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('onlineExperience.imageResult', { index: index + 1 }) }}</p>
                      <button type="button" class="rounded-full p-2 text-gray-400 transition hover:bg-gray-100 hover:text-primary-600 dark:hover:bg-dark-800 dark:hover:text-primary-300" @click="downloadImage(result.url, index)">
                        <Icon name="download" size="sm" />
                      </button>
                    </div>
                    <p v-if="result.revisedPrompt" class="line-clamp-3 text-xs leading-6 text-gray-500 dark:text-dark-400">{{ result.revisedPrompt }}</p>
                  </div>
                </article>
              </div>

              <EmptyState
                v-else
                :title="t('onlineExperience.imageEmptyTitle')"
                :description="activeTab === 'text2img' ? t('onlineExperience.imageEmptyDescription') : t('onlineExperience.editEmptyDescription')"
              >
                <template #icon>
                  <div class="flex h-20 w-20 items-center justify-center rounded-2xl bg-emerald-50 text-emerald-600 dark:bg-emerald-500/10 dark:text-emerald-300">
                    <Icon name="sparkles" size="xl" />
                  </div>
                </template>
              </EmptyState>
            </div>
          </template>
        </section>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import DOMPurify from 'dompurify'
import { marked } from 'marked'
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Select from '@/components/common/Select.vue'
import TextArea from '@/components/common/TextArea.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore, useAuthStore } from '@/stores'
import { onlineExperienceAPI } from './api'
import type {
  OnlineExperienceGroup,
  OnlineExperienceImageResponse,
  OnlineExperienceModel,
  OnlineExperiencePersistedState,
  OnlineExperienceTab
} from './types'

marked.setOptions({ gfm: true, breaks: true })

interface UIMessage {
  id: string
  role: 'user' | 'assistant'
  content: string
  pending?: boolean
}

interface UploadItem {
  id: string
  file: File
  previewUrl: string
}

interface RenderedImageResult {
  url: string
  revisedPrompt: string
}

const STORAGE_KEY = 'sub2api.online-experience.state'
const MAX_UPLOAD_SIZE = 10 * 1024 * 1024
const MAX_UPLOAD_COUNT = 4

const { t } = useI18n()
const authStore = useAuthStore()
const appStore = useAppStore()

const loading = ref(true)
const loadingModels = ref(false)
const groups = ref<OnlineExperienceGroup[]>([])
const models = ref<OnlineExperienceModel[]>([])
const selectedGroupId = ref<number | null>(null)
const activeTab = ref<OnlineExperienceTab>('chat')

const selectedChatModel = ref('')
const selectedImageModel = ref('')
const systemPrompt = ref('你是一个准确、克制、能直接给出可执行结果的助手。')
const chatInput = ref('')
const chatMessages = ref<UIMessage[]>([])
const chatLoading = ref(false)
const chatAbortController = ref<AbortController | null>(null)
const chatScrollRef = ref<HTMLElement | null>(null)

const text2imgPrompt = ref('')
const text2imgSize = ref('1024x1024')
const text2imgCount = ref(1)
const text2imgQuality = ref('high')
const text2imgLoading = ref(false)
const text2imgResults = ref<RenderedImageResult[]>([])

const img2imgPrompt = ref('')
const img2imgSize = ref('1024x1024')
const img2imgQuality = ref('high')
const img2imgLoading = ref(false)
const img2imgResults = ref<RenderedImageResult[]>([])
const uploadItems = ref<UploadItem[]>([])
const uploadInputRef = ref<HTMLInputElement | null>(null)

const tabs = computed(() => ([
  { value: 'chat', label: t('onlineExperience.tabs.chat'), icon: 'chatBubble' },
  { value: 'text2img', label: t('onlineExperience.tabs.text2img'), icon: 'sparkles' },
  { value: 'img2img', label: t('onlineExperience.tabs.img2img'), icon: 'upload' }
]) as Array<{ value: OnlineExperienceTab, label: string, icon: 'chatBubble' | 'sparkles' | 'upload' }>)

const aspectOptions = [
  { value: '1024x1024', label: '1:1', previewClass: 'h-12 w-12' },
  { value: '1536x1024', label: '16:9', previewClass: 'h-8 w-14' },
  { value: '1024x1536', label: '9:16', previewClass: 'h-14 w-8' }
]

const aspectSelectOptions = aspectOptions.map(option => ({
  value: option.value,
  label: option.label
}))

const imageCountOptions = [1, 2, 3, 4].map(count => ({
  value: count,
  label: String(count)
}))

const qualityOptions = [
  { value: 'high', label: t('onlineExperience.quality.high') },
  { value: 'medium', label: t('onlineExperience.quality.medium') },
  { value: 'low', label: t('onlineExperience.quality.low') }
]

const chatSuggestions = computed(() => ([
  { title: t('onlineExperience.suggestions.debug'), prompt: t('onlineExperience.suggestionPrompts.debug'), icon: 'cpu' },
  { title: t('onlineExperience.suggestions.summary'), prompt: t('onlineExperience.suggestionPrompts.summary'), icon: 'document' },
  { title: t('onlineExperience.suggestions.write'), prompt: t('onlineExperience.suggestionPrompts.write'), icon: 'sparkles' },
  { title: t('onlineExperience.suggestions.translate'), prompt: t('onlineExperience.suggestionPrompts.translate'), icon: 'globe' }
]) as const)

const selectedGroup = computed(() => groups.value.find(group => group.id === selectedGroupId.value) ?? null)
const groupOptions = computed<Array<Record<string, unknown>>>(() => groups.value.map(group => ({
  value: group.id,
  label: group.name
})))
const chatModels = computed(() => models.value.filter(model => model.type === 'chat'))
const imageModels = computed(() => models.value.filter(model => model.type === 'image'))
const chatModelOptions = computed(() => chatModels.value.map(model => ({ value: model.id, label: model.display_name })))
const imageModelOptions = computed(() => imageModels.value.map(model => ({ value: model.id, label: model.display_name })))
const selectedChatModelLabel = computed(() => chatModels.value.find(model => model.id === selectedChatModel.value)?.display_name || t('onlineExperience.noModelSelected'))
const selectedImageModelLabel = computed(() => imageModels.value.find(model => model.id === selectedImageModel.value)?.display_name || t('onlineExperience.noModelSelected'))
const formattedBalance = computed(() => new Intl.NumberFormat(undefined, {
  style: 'currency',
  currency: 'USD',
  minimumFractionDigits: 2,
  maximumFractionDigits: 4
}).format(authStore.user?.balance ?? 0))
const currentGroupHint = computed(() => selectedGroup.value?.description || t('onlineExperience.groupHint'))
const canSendChat = computed(() => Boolean(selectedGroupId.value && selectedChatModel.value && chatInput.value.trim() && !chatLoading.value))
const canRetryChat = computed(() => !chatLoading.value && chatMessages.value.some(message => message.role === 'user'))
const canGenerateText2Img = computed(() => Boolean(selectedGroupId.value && selectedImageModel.value && text2imgPrompt.value.trim() && !text2imgLoading.value))
const canGenerateImg2Img = computed(() => Boolean(selectedGroupId.value && selectedImageModel.value && uploadItems.value.length && img2imgPrompt.value.trim() && !img2imgLoading.value))
const currentImageLoading = computed(() => activeTab.value === 'img2img' ? img2imgLoading.value : text2imgLoading.value)
const currentImageResults = computed(() => activeTab.value === 'img2img' ? img2imgResults.value : text2imgResults.value)

const loadPersistedState = (): OnlineExperiencePersistedState => {
  try {
    return JSON.parse(localStorage.getItem(STORAGE_KEY) || '{}') as OnlineExperiencePersistedState
  } catch {
    return {}
  }
}

const persistState = (): void => {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify({
      activeTab: activeTab.value,
      groupId: selectedGroupId.value
    }))
  } catch {
    // Ignore storage failures
  }
}

watch([activeTab, selectedGroupId], persistState)

const renderMarkdown = (content: string): string => {
  if (!content.trim()) return ''
  return DOMPurify.sanitize(marked.parse(content) as string)
}

const resolveErrorMessage = (error: unknown): string => {
  if (error instanceof Error && error.message) {
    return error.message
  }
  return t('onlineExperience.genericError')
}

const scrollChatToBottom = async (force = false): Promise<void> => {
  await nextTick()
  const element = chatScrollRef.value
  if (!element) return

  if (force) {
    element.scrollTop = element.scrollHeight
    return
  }

  const distance = element.scrollHeight - element.scrollTop - element.clientHeight
  if (distance < 180) {
    element.scrollTop = element.scrollHeight
  }
}

const syncSelectedModels = (): void => {
  if (!chatModels.value.some(model => model.id === selectedChatModel.value)) {
    selectedChatModel.value = chatModels.value[0]?.id || ''
  }
  if (!imageModels.value.some(model => model.id === selectedImageModel.value)) {
    selectedImageModel.value = imageModels.value[0]?.id || ''
  }
}

const loadModels = async (groupId: number): Promise<void> => {
  loadingModels.value = true
  try {
    models.value = await onlineExperienceAPI.listModels(groupId)
    syncSelectedModels()
  } catch (error) {
    models.value = []
    selectedChatModel.value = ''
    selectedImageModel.value = ''
    appStore.showError(resolveErrorMessage(error))
  } finally {
    loadingModels.value = false
  }
}

const handleGroupChange = async (value: string | number | boolean | null): Promise<void> => {
  const nextGroupId = typeof value === 'number' ? value : Number(value)
  if (!Number.isFinite(nextGroupId) || nextGroupId <= 0 || nextGroupId === selectedGroupId.value) {
    return
  }
  selectedGroupId.value = nextGroupId
  await loadModels(nextGroupId)
}

const initializePage = async (): Promise<void> => {
  loading.value = true
  try {
    await authStore.refreshUser().catch(() => undefined)
    groups.value = await onlineExperienceAPI.listGroups()

    const persisted = loadPersistedState()
    const availableTabs = new Set(tabs.value.map(item => item.value))
    activeTab.value = availableTabs.has(persisted.activeTab || 'chat') ? (persisted.activeTab || 'chat') : 'chat'

    if (groups.value.length) {
      const preferredGroup = persisted.groupId && groups.value.some(group => group.id === persisted.groupId)
        ? persisted.groupId
        : groups.value[0].id
      selectedGroupId.value = preferredGroup
      await loadModels(preferredGroup)
    }
  } catch (error) {
    appStore.showError(resolveErrorMessage(error))
  } finally {
    loading.value = false
  }
}

const buildChatHistory = (): Array<{ role: 'system' | 'user' | 'assistant', content: string }> => {
  const history: Array<{ role: 'system' | 'user' | 'assistant', content: string }> = []
  if (systemPrompt.value.trim()) {
    history.push({ role: 'system', content: systemPrompt.value.trim() })
  }
  for (const message of chatMessages.value) {
    if (!message.content.trim()) continue
    history.push({
      role: message.role,
      content: message.content
    })
  }
  return history
}

const sendChat = async (): Promise<void> => {
  if (!selectedGroupId.value || !selectedChatModel.value || !chatInput.value.trim() || chatLoading.value) {
    return
  }

  const userContent = chatInput.value.trim()
  const userMessage: UIMessage = {
    id: `${Date.now()}-user`,
    role: 'user',
    content: userContent
  }
  const assistantMessage: UIMessage = {
    id: `${Date.now()}-assistant`,
    role: 'assistant',
    content: '',
    pending: true
  }

  chatMessages.value.push(userMessage, assistantMessage)
  chatInput.value = ''
  chatLoading.value = true
  chatAbortController.value = new AbortController()
  await scrollChatToBottom(true)

  try {
    await onlineExperienceAPI.streamChat(
      selectedGroupId.value,
      {
        model: selectedChatModel.value,
        messages: buildChatHistory()
      },
      (delta) => {
        assistantMessage.content += delta
        assistantMessage.pending = false
        scrollChatToBottom()
      },
      chatAbortController.value.signal
    )

    assistantMessage.pending = false
    if (!assistantMessage.content.trim()) {
      assistantMessage.content = t('onlineExperience.emptyAssistantReply')
    }
  } catch (error) {
    if (!(error instanceof DOMException && error.name === 'AbortError')) {
      assistantMessage.content = assistantMessage.content || resolveErrorMessage(error)
      appStore.showError(resolveErrorMessage(error))
    }
    assistantMessage.pending = false
  } finally {
    chatLoading.value = false
    chatAbortController.value = null
    await authStore.refreshUser().catch(() => undefined)
    await scrollChatToBottom(true)
  }
}

const stopChat = (): void => {
  chatAbortController.value?.abort()
}

const clearChat = (): void => {
  stopChat()
  chatInput.value = ''
  chatMessages.value = []
}

const retryLastReply = async (): Promise<void> => {
  if (chatLoading.value) return

  let lastUserIndex = -1
  for (let index = chatMessages.value.length - 1; index >= 0; index -= 1) {
    if (chatMessages.value[index].role === 'user') {
      lastUserIndex = index
      break
    }
  }
  if (lastUserIndex < 0) return

  const prompt = chatMessages.value[lastUserIndex].content
  chatMessages.value = chatMessages.value.slice(0, lastUserIndex)
  chatInput.value = prompt
  await sendChat()
}

const normalizeImageResults = (response: OnlineExperienceImageResponse): RenderedImageResult[] => {
  return (response.data || [])
    .map(item => ({
      url: item.url || (item.b64_json ? `data:image/png;base64,${item.b64_json}` : ''),
      revisedPrompt: item.revised_prompt || ''
    }))
    .filter(item => Boolean(item.url))
}

const submitTextToImage = async (): Promise<void> => {
  if (!selectedGroupId.value || !selectedImageModel.value || !text2imgPrompt.value.trim()) {
    return
  }

  text2imgLoading.value = true
  text2imgResults.value = []

  try {
    const response = await onlineExperienceAPI.generateImage(selectedGroupId.value, {
      model: selectedImageModel.value,
      prompt: text2imgPrompt.value.trim(),
      size: text2imgSize.value,
      n: text2imgCount.value,
      quality: text2imgQuality.value
    })

    text2imgResults.value = normalizeImageResults(response)
    if (!text2imgResults.value.length) {
      appStore.showWarning(t('onlineExperience.noImageResult'))
    }
    await authStore.refreshUser().catch(() => undefined)
  } catch (error) {
    appStore.showError(resolveErrorMessage(error))
  } finally {
    text2imgLoading.value = false
  }
}

const openUploadDialog = (): void => {
  uploadInputRef.value?.click()
}

const releaseUploadItem = (item: UploadItem): void => {
  URL.revokeObjectURL(item.previewUrl)
}

const handleUploadChange = (event: Event): void => {
  const input = event.target as HTMLInputElement
  const files = Array.from(input.files || [])
  if (!files.length) return

  const nextItems = [...uploadItems.value]
  for (const file of files) {
    if (!file.type.startsWith('image/')) {
      appStore.showWarning(t('onlineExperience.invalidImageFile'))
      continue
    }
    if (file.size > MAX_UPLOAD_SIZE) {
      appStore.showWarning(t('onlineExperience.imageTooLarge'))
      continue
    }
    if (nextItems.length >= MAX_UPLOAD_COUNT) {
      appStore.showWarning(t('onlineExperience.tooManyUploads'))
      break
    }
    nextItems.push({
      id: `${Date.now()}-${file.name}-${nextItems.length}`,
      file,
      previewUrl: URL.createObjectURL(file)
    })
  }

  uploadItems.value = nextItems
  input.value = ''
}

const removeUpload = (id: string): void => {
  const current = uploadItems.value.find(item => item.id === id)
  if (current) {
    releaseUploadItem(current)
  }
  uploadItems.value = uploadItems.value.filter(item => item.id !== id)
}

const submitImageEdit = async (): Promise<void> => {
  if (!selectedGroupId.value || !selectedImageModel.value || !uploadItems.value.length || !img2imgPrompt.value.trim()) {
    return
  }

  const formData = new FormData()
  formData.append('model', selectedImageModel.value)
  formData.append('prompt', img2imgPrompt.value.trim())
  formData.append('size', img2imgSize.value)
  formData.append('quality', img2imgQuality.value)
  for (const item of uploadItems.value) {
    formData.append('image', item.file)
  }

  img2imgLoading.value = true
  img2imgResults.value = []

  try {
    const response = await onlineExperienceAPI.editImage(selectedGroupId.value, formData)
    img2imgResults.value = normalizeImageResults(response)
    if (!img2imgResults.value.length) {
      appStore.showWarning(t('onlineExperience.noImageResult'))
    }
    await authStore.refreshUser().catch(() => undefined)
  } catch (error) {
    appStore.showError(resolveErrorMessage(error))
  } finally {
    img2imgLoading.value = false
  }
}

const downloadImage = (url: string, index: number): void => {
  const link = document.createElement('a')
  link.href = url
  link.download = `online-experience-${activeTab.value}-${index + 1}.png`
  link.click()
}

const formatFileSize = (size: number): string => {
  if (size < 1024 * 1024) {
    return `${(size / 1024).toFixed(1)} KB`
  }
  return `${(size / (1024 * 1024)).toFixed(2)} MB`
}

onMounted(() => {
  initializePage()
})

onBeforeUnmount(() => {
  stopChat()
  for (const item of uploadItems.value) {
    releaseUploadItem(item)
  }
})
</script>

<style scoped>
.online-experience-markdown :deep(p:first-child) {
  margin-top: 0;
}

.online-experience-markdown :deep(p:last-child) {
  margin-bottom: 0;
}

.online-experience-markdown :deep(p) {
  margin: 0.6rem 0;
}

.online-experience-markdown :deep(ul),
.online-experience-markdown :deep(ol) {
  margin: 0.75rem 0;
  padding-left: 1.15rem;
}

.online-experience-markdown :deep(li) {
  margin: 0.35rem 0;
}

.online-experience-markdown :deep(pre) {
  overflow-x: auto;
  border-radius: 1rem;
  background: rgba(15, 23, 42, 0.95);
  padding: 1rem;
  color: #e2e8f0;
}

.online-experience-markdown :deep(code) {
  border-radius: 0.45rem;
  background: rgba(148, 163, 184, 0.14);
  padding: 0.18rem 0.35rem;
  font-size: 0.92em;
}

.online-experience-markdown :deep(pre code) {
  background: transparent;
  padding: 0;
}

.online-experience-markdown :deep(a) {
  color: rgb(37 99 235);
  text-decoration: underline;
}
</style>
