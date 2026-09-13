import request from '@/utils/request'
import type { AlertViewValue } from '@/constants/stockAlert'
import type { AlertPageResult, StockAlert } from '@/types'

export function listStockAlerts(params: {
  view?: AlertViewValue
  page?: number
  page_size?: number
}): Promise<AlertPageResult> {
  return request.get('/notifications', { params })
}

export function getUnreadAlertCount(): Promise<{ unread_count: number }> {
  return request.get('/notifications/unread-count')
}

export function markAlertRead(id: number): Promise<{ id: number }> {
  return request.put(`/notifications/${id}/read`)
}

export function markAllAlertsRead(): Promise<{ affected: number }> {
  return request.put('/notifications/read-all')
}

export type { StockAlert }
