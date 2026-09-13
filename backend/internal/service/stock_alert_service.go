package service

import (
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"github.com/ld/storeinventory/internal/constants"
	"github.com/ld/storeinventory/internal/model"
	"github.com/ld/storeinventory/internal/repository"
	"github.com/ld/storeinventory/internal/util"
)

// StockAlertService 库存预警通知业务逻辑：低库存生成未读通知、未读去重、已读管理。
type StockAlertService interface {
	// TriggerAfterChange 在库存发生变动后按当前水位刷新预警通知（事务内）。
	TriggerAfterChangeTx(tx *gorm.DB, storeID, skuID uint) error
	TriggerAfterChange(storeID, skuID uint) error
	List(viewer *util.JWTClaims, page, pageSize int, view constants.AlertView) ([]model.StockAlert, int64, error)
	UnreadCount(viewer *util.JWTClaims) (int64, error)
	MarkRead(viewer *util.JWTClaims, id uint) error
	MarkAllRead(viewer *util.JWTClaims) (int64, error)
}

type stockAlertService struct {
	alertRepo repository.StockAlertRepository
	invRepo   repository.StoreInventoryRepository
	logger    *slog.Logger
}

// NewStockAlertService 构造库存预警通知服务。
func NewStockAlertService(alertRepo repository.StockAlertRepository, invRepo repository.StoreInventoryRepository, logger *slog.Logger) StockAlertService {
	return &stockAlertService{alertRepo: alertRepo, invRepo: invRepo, logger: logger}
}

// scopeFor 解析可见范围：总部/管理员看全部门店，店长仅看本店。
func (s *stockAlertService) scopeFor(viewer *util.JWTClaims) (repository.AlertScope, error) {
	scope := repository.AlertScope{}
	if viewer == nil {
		return scope, fmt.Errorf("resolve alert scope: %w", util.ErrUnauthorized)
	}
	if viewer.Role == constants.RoleStoreManager {
		if viewer.StoreID == nil {
			return scope, fmt.Errorf("resolve alert scope manager[%s] without store: %w", viewer.Username, util.ErrForbidden)
		}
		scope.StoreID = *viewer.StoreID
	}
	return scope, nil
}

func (s *stockAlertService) TriggerAfterChange(storeID, skuID uint) error {
	return s.TriggerAfterChangeTx(nil, storeID, skuID)
}

func (s *stockAlertService) TriggerAfterChangeTx(tx *gorm.DB, storeID, skuID uint) error {
	inv, err := s.invRepo.FindByStoreAndSKUTx(tx, storeID, skuID)
	if err != nil {
		// 库存尚未建档时无预警可言，安静跳过而非中断业务事务。
		s.logger.Warn(constants.LogStockAlertSkipped, "store_id", storeID, "sku_id", skuID, "reason", "inventory_not_found")
		return nil
	}
	// 回到或高于安全线：不生成新通知（历史未读通知保留，由用户手动已读）。
	if inv.Quantity >= inv.SafetyStock {
		return nil
	}
	alert := &model.StockAlert{
		StoreID:     storeID,
		SKUID:       skuID,
		Quantity:    inv.Quantity,
		SafetyStock: inv.SafetyStock,
		ShortageQty: inv.SafetyStock - inv.Quantity,
		IsRead:      false,
		TriggeredAt: time.Now(),
	}
	saved, created, err := s.alertRepo.UpsertUnreadTx(tx, alert)
	if err != nil {
		s.logger.Error(constants.LogStockWarnAlertFailed, "store_id", storeID, "sku_id", skuID, "error", err)
		return fmt.Errorf("trigger stock alert store[%d] sku[%d]: %w", storeID, skuID, err)
	}
	if created {
		s.logger.Info(constants.LogStockAlertCreated, "alert_id", saved.ID, "store_id", storeID, "sku_id", skuID,
			"quantity", saved.Quantity, "safety_stock", saved.SafetyStock, "shortage_qty", saved.ShortageQty)
	} else {
		s.logger.Info(constants.LogStockAlertRefreshed, "alert_id", saved.ID, "store_id", storeID, "sku_id", skuID,
			"trigger_count", saved.TriggerCount, "shortage_qty", saved.ShortageQty)
	}
	return nil
}

func (s *stockAlertService) List(viewer *util.JWTClaims, page, pageSize int, view constants.AlertView) ([]model.StockAlert, int64, error) {
	scope, err := s.scopeFor(viewer)
	if err != nil {
		return nil, 0, err
	}
	scope.Unread = view == constants.AlertViewUnread
	alerts, total, err := s.alertRepo.List(page, pageSize, scope)
	if err != nil {
		return nil, 0, fmt.Errorf("list stock alerts view[%s]: %w", view, err)
	}
	s.logger.Info(constants.LogStockAlertListQueried, "view", view, "store_id", scope.StoreID, "total", total)
	return alerts, total, nil
}

func (s *stockAlertService) UnreadCount(viewer *util.JWTClaims) (int64, error) {
	scope, err := s.scopeFor(viewer)
	if err != nil {
		return 0, err
	}
	total, err := s.alertRepo.CountUnread(scope)
	if err != nil {
		return 0, fmt.Errorf("count unread stock alerts: %w", err)
	}
	return total, nil
}

func (s *stockAlertService) MarkRead(viewer *util.JWTClaims, id uint) error {
	scope, err := s.scopeFor(viewer)
	if err != nil {
		return err
	}
	// 先用可见范围校验归属，店长不能标记其他门店通知。
	if _, err := s.alertRepo.FindByIDForScope(id, scope); err != nil {
		return fmt.Errorf("mark stock alert read id[%d] role[%s]: %w", id, viewer.Role, err)
	}
	if err := s.alertRepo.MarkReadByID(id); err != nil {
		return fmt.Errorf("mark stock alert read id[%d]: %w", id, err)
	}
	s.logger.Info(constants.LogStockAlertMarkRead, "alert_id", id, "username", viewer.Username, "role", viewer.Role)
	return nil
}

func (s *stockAlertService) MarkAllRead(viewer *util.JWTClaims) (int64, error) {
	scope, err := s.scopeFor(viewer)
	if err != nil {
		return 0, err
	}
	affected, err := s.alertRepo.MarkAllRead(scope)
	if err != nil {
		return 0, fmt.Errorf("mark all stock alerts read role[%s]: %w", viewer.Role, err)
	}
	s.logger.Info(constants.LogStockAlertMarkAllRead, "affected", affected, "username", viewer.Username, "role", viewer.Role, "store_id", scope.StoreID)
	return affected, nil
}
