package constants

// AlertView 预警通知视图：未读 / 全部，前后端共享定义
// （frontend/src/constants/stockAlert.ts 对应实现）。
type AlertView string

const (
	AlertViewUnread AlertView = "unread"
	AlertViewAll    AlertView = "all"
)

// NormalizeAlertView 将入参归一为合法视图，默认未读。
func NormalizeAlertView(v AlertView) AlertView {
	switch v {
	case AlertViewUnread, AlertViewAll:
		return v
	}
	return AlertViewUnread
}
