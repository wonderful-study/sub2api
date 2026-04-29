import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import type { Account } from '@/types'
import AccountsView from '../AccountsView.vue'

const {
  listAccounts,
  getBatchTodayStats,
  batchTest,
  getAllProxies,
  getAllGroups,
  showError,
  showSuccess
} = vi.hoisted(() => ({
  listAccounts: vi.fn(),
  getBatchTodayStats: vi.fn(),
  batchTest: vi.fn(),
  getAllProxies: vi.fn(),
  getAllGroups: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      list: listAccounts,
      listWithEtag: vi.fn(),
      getBatchTodayStats,
      batchTest
    },
    proxies: {
      getAll: getAllProxies
    },
    groups: {
      getAll: getAllGroups
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess
  })
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    isSimpleMode: false
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

vi.mock('@/components/account', () => {
  const Stub = { template: '<div />' }
  return {
    CreateAccountModal: Stub,
    EditAccountModal: Stub,
    BulkEditAccountModal: Stub,
    SyncFromCrsModal: Stub,
    TempUnschedStatusModal: Stub
  }
})

const makeAccount = (id: number): Account => ({
  id,
  name: `account-${id}`,
  platform: 'openai',
  type: 'oauth',
  credentials: {},
  extra: {},
  proxy_id: null,
  concurrency: 1,
  current_concurrency: 0,
  priority: 1,
  rate_multiplier: 1,
  status: 'active',
  error_message: null,
  last_used_at: null,
  expires_at: null,
  auto_pause_on_expired: false,
  created_at: '2026-04-30T00:00:00Z',
  updated_at: '2026-04-30T00:00:00Z',
  group_ids: [],
  groups: [],
  schedulable: true,
  rate_limited_at: null,
  rate_limit_reset_at: null,
  overload_until: null,
  temp_unschedulable_until: null,
  temp_unschedulable_reason: null,
  session_window_start: null,
  session_window_end: null,
  session_window_status: null
})

const DataTableStub = {
  props: ['data'],
  template: `
    <table>
      <thead>
        <tr><th><slot name="header-select" /></th></tr>
      </thead>
      <tbody>
        <tr v-for="row in data" :key="row.id" :data-row-id="row.id">
          <td><slot name="cell-select" :row="row" /></td>
          <td><slot name="cell-expires_at" :row="row" /></td>
        </tr>
      </tbody>
    </table>
  `
}

const mountAccountsView = () => mount(AccountsView, {
  global: {
    stubs: {
      AppLayout: { template: '<div><slot /></div>' },
      TablePageLayout: {
        template: '<div class="table-page-layout"><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
      },
      DataTable: DataTableStub,
      Pagination: true,
      ConfirmDialog: true,
      EmptyState: true,
      AccountTableActions: { template: '<div><slot name="after" /><slot name="beforeCreate" /></div>' },
      AccountTableFilters: true,
      AccountActionMenu: true,
      ImportDataModal: true,
      ReAuthAccountModal: true,
      AccountTestModal: true,
      AccountStatsModal: true,
      ScheduledTestsPanel: true,
      ErrorPassthroughRulesModal: true,
      TLSFingerprintProfilesModal: true,
      AccountStatusIndicator: true,
      AccountUsageCell: true,
      AccountTodayStatsCell: true,
      AccountGroupsCell: true,
      AccountCapacityCell: true,
      PlatformTypeBadge: true,
      Icon: true,
      Teleport: true
    }
  }
})

const selectFirstAccount = async (wrapper: ReturnType<typeof mountAccountsView>) => {
  const checkboxes = wrapper.findAll('input[type="checkbox"]')
  expect(checkboxes.length).toBeGreaterThanOrEqual(2)
  await checkboxes[1].setValue(true)
  await flushPromises()
}

const clickBatchTest = async (wrapper: ReturnType<typeof mountAccountsView>) => {
  const button = wrapper.findAll('button').find(btn => btn.text() === 'admin.accounts.bulkActions.testConnection')
  expect(button).toBeTruthy()
  await button!.trigger('click')
  await flushPromises()
}

describe('admin AccountsView batch test', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.stubGlobal('confirm', vi.fn(() => true))

    listAccounts.mockReset()
    getBatchTodayStats.mockReset()
    batchTest.mockReset()
    getAllProxies.mockReset()
    getAllGroups.mockReset()
    showError.mockReset()
    showSuccess.mockReset()

    listAccounts.mockResolvedValue({
      items: [makeAccount(101)],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1
    })
    getBatchTodayStats.mockResolvedValue({ stats: {} })
    getAllProxies.mockResolvedValue([])
    getAllGroups.mockResolvedValue([])
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('calls batch test API, shows success, and reloads accounts', async () => {
    batchTest.mockResolvedValue({
      total: 1,
      success: 1,
      failed: 0,
      results: [{ account_id: 101, success: true, status: 'success', latency_ms: 12 }]
    })

    const wrapper = mountAccountsView()
    await flushPromises()
    expect(listAccounts).toHaveBeenCalledTimes(1)

    await selectFirstAccount(wrapper)
    await clickBatchTest(wrapper)

    expect(batchTest).toHaveBeenCalledWith([101])
    expect(showSuccess).toHaveBeenCalledWith('admin.accounts.bulkActions.testConnectionSuccess')
    expect(showError).not.toHaveBeenCalled()
    expect(listAccounts).toHaveBeenCalledTimes(2)
  })

  it('shows partial failure message and reloads accounts', async () => {
    batchTest.mockResolvedValue({
      total: 1,
      success: 0,
      failed: 1,
      results: [{ account_id: 101, success: false, status: 'failed', latency_ms: 0, error: 'failed' }]
    })

    const wrapper = mountAccountsView()
    await flushPromises()

    await selectFirstAccount(wrapper)
    await clickBatchTest(wrapper)

    expect(batchTest).toHaveBeenCalledWith([101])
    expect(showError).toHaveBeenCalledWith('admin.accounts.bulkActions.partialSuccess')
    expect(showSuccess).not.toHaveBeenCalled()
    expect(listAccounts).toHaveBeenCalledTimes(2)
  })
})

describe('admin AccountsView expiry display', () => {
  beforeEach(() => {
    localStorage.clear()

    listAccounts.mockReset()
    getBatchTodayStats.mockReset()
    batchTest.mockReset()
    getAllProxies.mockReset()
    getAllGroups.mockReset()
    showError.mockReset()
    showSuccess.mockReset()

    getBatchTodayStats.mockResolvedValue({ stats: {} })
    getAllProxies.mockResolvedValue([])
    getAllGroups.mockResolvedValue([])
  })

  it('shows subscription expiry fallback without auto-pause badge', async () => {
    listAccounts.mockResolvedValue({
      items: [{
        ...makeAccount(201),
        credentials: {
          subscription_expires_at: '2026-05-10T00:00:00Z'
        },
        expires_at: null,
        auto_pause_on_expired: true
      }],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1
    })

    const wrapper = mountAccountsView()
    await flushPromises()

    expect(wrapper.text()).toContain('2026-05-10')
    expect(wrapper.text()).not.toContain('admin.accounts.autoPauseOnExpired')
  })
})
