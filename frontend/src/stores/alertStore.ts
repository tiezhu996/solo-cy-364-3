import { defineStore } from 'pinia'
import { ref } from 'vue'
import {
  listStockAlerts,
  getUnreadAlertCount,
  markAlertRead,
  markAllAlertsRead
} from '@/api/stockAlert'
import { AlertView, type AlertViewValue } from '@/constants/stockAlert'
import type { StockAlert } from '@/types'

export const useAlertStore = defineStore('stockAlert', () => {
  const list = ref<StockAlert[]>([])
  const total = ref(0)
  const unreadCount = ref(0)
  const view = ref<AlertViewValue>(AlertView.UNREAD)
  const page = ref(1)
  const pageSize = ref(20)
  const loading = ref(false)

  async function fetchList(next?: { view?: AlertViewValue; page?: number; page_size?: number }) {
    if (next?.view) view.value = next.view
    if (next?.page) page.value = next.page
    if (next?.page_size) pageSize.value = next.page_size
    loading.value = true
    try {
      const res = await listStockAlerts({
        view: view.value,
        page: page.value,
        page_size: pageSize.value
      })
      list.value = res.list
      total.value = res.total
    } finally {
      loading.value = false
    }
  }

  async function fetchUnreadCount() {
    const res = await getUnreadAlertCount()
    unreadCount.value = res.unread_count
  }

  async function markRead(id: number) {
    await markAlertRead(id)
  }

  async function markAllRead() {
    const res = await markAllAlertsRead()
    return res.affected
  }

  function reset() {
    list.value = []
    total.value = 0
    unreadCount.value = 0
    page.value = 1
  }

  return {
    list,
    total,
    unreadCount,
    view,
    page,
    pageSize,
    loading,
    fetchList,
    fetchUnreadCount,
    markRead,
    markAllRead,
    reset
  }
})
