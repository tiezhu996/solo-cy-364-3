package router

import (
	"github.com/gin-gonic/gin"

	"github.com/ld/storeinventory/internal/handler"
)

// registerStockAlertRoutes 库存预警通知路由。均需登录，可见范围由 service 按角色收敛：
// 总部/管理员看全部门店，店长仅看本店。
func registerStockAlertRoutes(v1 *gin.RouterGroup, h *handler.StockAlertHandler, auth gin.HandlerFunc) {
	alerts := v1.Group("/notifications", auth)
	alerts.GET("", h.List)
	alerts.GET("/unread-count", h.UnreadCount)
	alerts.PUT("/read-all", h.MarkAllRead)
	alerts.PUT("/:id/read", h.MarkRead)
}
