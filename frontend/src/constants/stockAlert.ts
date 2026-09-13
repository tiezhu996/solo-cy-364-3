// 库存预警通知视图枚举（与 backend/internal/constants/stock_alert.go 对应）
export const AlertView = {
  UNREAD: 'unread',
  ALL: 'all'
} as const

export type AlertViewValue = (typeof AlertView)[keyof typeof AlertView]

export const ALERT_VIEW_OPTIONS: { label: string; value: AlertViewValue }[] = [
  { label: '未读', value: AlertView.UNREAD },
  { label: '全部', value: AlertView.ALL }
]
