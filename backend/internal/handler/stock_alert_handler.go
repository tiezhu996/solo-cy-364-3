package handler

import (
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/ld/storeinventory/internal/constants"
	"github.com/ld/storeinventory/internal/dto"
	"github.com/ld/storeinventory/internal/middleware"
	"github.com/ld/storeinventory/internal/service"
	"github.com/ld/storeinventory/internal/util"
)

// StockAlertHandler 库存预警通知接口处理器。
type StockAlertHandler struct {
	alertSvc service.StockAlertService
}

// NewStockAlertHandler 构造库存预警通知处理器。
func NewStockAlertHandler(alertSvc service.StockAlertService) *StockAlertHandler {
	return &StockAlertHandler{alertSvc: alertSvc}
}

// List 预警通知列表，支持未读 / 全部两个视图。
func (h *StockAlertHandler) List(c *gin.Context) {
	viewer, err := middleware.CurrentUser(c)
	if err != nil {
		c.Error(util.Unauthorized(constants.MsgUnauthorized, err))
		return
	}
	var q dto.ListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		c.Error(util.Validation(constants.MsgInvalidRequest, err))
		return
	}
	q.Normalize()
	view := constants.NormalizeAlertView(constants.AlertView(c.Query("view")))
	alerts, total, err := h.alertSvc.List(viewer, q.Page, q.PageSize, view)
	if err != nil {
		c.Error(fmt.Errorf("handler list stock alerts: %w", err))
		return
	}
	util.OK(c, gin.H{"list": alerts, "total": total, "page": q.Page, "page_size": q.PageSize, "view": view})
}

// UnreadCount 当前用户可见的未读通知数量（供角标展示）。
func (h *StockAlertHandler) UnreadCount(c *gin.Context) {
	viewer, err := middleware.CurrentUser(c)
	if err != nil {
		c.Error(util.Unauthorized(constants.MsgUnauthorized, err))
		return
	}
	count, err := h.alertSvc.UnreadCount(viewer)
	if err != nil {
		c.Error(fmt.Errorf("handler count unread stock alerts: %w", err))
		return
	}
	util.OK(c, gin.H{"unread_count": count})
}

// MarkRead 标记单条通知为已读。
func (h *StockAlertHandler) MarkRead(c *gin.Context) {
	viewer, err := middleware.CurrentUser(c)
	if err != nil {
		c.Error(util.Unauthorized(constants.MsgUnauthorized, err))
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.Error(util.Validation("无效的通知ID", err))
		return
	}
	if err := h.alertSvc.MarkRead(viewer, uint(id)); err != nil {
		c.Error(fmt.Errorf("handler mark stock alert[id=%d] read: %w", id, err))
		return
	}
	util.OKMessage(c, constants.MsgStockAlertRead, gin.H{"id": id})
}

// MarkAllRead 标记当前用户可见范围的全部通知为已读（总部=全部门店，店长=本店）。
func (h *StockAlertHandler) MarkAllRead(c *gin.Context) {
	viewer, err := middleware.CurrentUser(c)
	if err != nil {
		c.Error(util.Unauthorized(constants.MsgUnauthorized, err))
		return
	}
	affected, err := h.alertSvc.MarkAllRead(viewer)
	if err != nil {
		c.Error(fmt.Errorf("handler mark all stock alerts read: %w", err))
		return
	}
	util.OKMessage(c, constants.MsgStockAlertAllRead, gin.H{"affected": affected})
}
