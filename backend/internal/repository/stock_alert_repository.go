package repository

import (
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/ld/storeinventory/internal/model"
	"github.com/ld/storeinventory/internal/util"
)

// AlertScope 通知查询的可见范围：StoreID=0 表示总部/管理员可看全部门店。
type AlertScope struct {
	StoreID uint
	Unread  bool
}

// StockAlertRepository 库存预警通知仓储。
type StockAlertRepository interface {
	// UpsertUnread 同一门店 + SKU 只保留一条未读：存在则原地刷新，否则新增。
	UpsertUnread(alert *model.StockAlert) (*model.StockAlert, bool, error)
	UpsertUnreadTx(tx *gorm.DB, alert *model.StockAlert) (*model.StockAlert, bool, error)
	FindByIDForScope(id uint, scope AlertScope) (*model.StockAlert, error)
	List(page, pageSize int, scope AlertScope) ([]model.StockAlert, int64, error)
	CountUnread(scope AlertScope) (int64, error)
	MarkReadByID(id uint) error
	MarkAllRead(scope AlertScope) (int64, error)
}

type stockAlertRepository struct {
	db *gorm.DB
}

// NewStockAlertRepository 构造库存预警通知仓储。
func NewStockAlertRepository(db *gorm.DB) StockAlertRepository {
	return &stockAlertRepository{db: db}
}

func (r *stockAlertRepository) UpsertUnread(alert *model.StockAlert) (*model.StockAlert, bool, error) {
	return r.UpsertUnreadTx(nil, alert)
}

// UpsertUnreadTx 返回最新通知与是否为新建（false 表示刷新已有未读）。
func (r *stockAlertRepository) UpsertUnreadTx(tx *gorm.DB, alert *model.StockAlert) (*model.StockAlert, bool, error) {
	conn := dbOrTx(r.db, tx)
	var existing model.StockAlert
	err := conn.Where("store_id = ? AND sku_id = ? AND is_read = ?", alert.StoreID, alert.SKUID, false).
		Order("triggered_at DESC").First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		alert.IsRead = false
		alert.TriggerCount = 1
		if err := conn.Create(alert).Error; err != nil {
			if isDuplicate(err) {
				// 并发下部分唯一索引兜底：重读后刷新。
				if got, e := r.reloadAndRefreshTx(conn, alert); e == nil {
					return got, false, nil
				}
			}
			return nil, false, fmt.Errorf("create stock alert: %w", err)
		}
		return alert, true, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("find unread stock alert: %w", err)
	}
	existing.Quantity = alert.Quantity
	existing.SafetyStock = alert.SafetyStock
	existing.ShortageQty = alert.ShortageQty
	existing.TriggerCount++
	existing.TriggeredAt = alert.TriggeredAt
	if err := conn.Save(&existing).Error; err != nil {
		return nil, false, fmt.Errorf("refresh stock alert: %w", err)
	}
	return &existing, false, nil
}

func (r *stockAlertRepository) reloadAndRefreshTx(conn *gorm.DB, alert *model.StockAlert) (*model.StockAlert, error) {
	var existing model.StockAlert
	if err := conn.Where("store_id = ? AND sku_id = ? AND is_read = ?", alert.StoreID, alert.SKUID, false).
		Order("triggered_at DESC").First(&existing).Error; err != nil {
		return nil, fmt.Errorf("reload stock alert: %w", err)
	}
	existing.Quantity = alert.Quantity
	existing.SafetyStock = alert.SafetyStock
	existing.ShortageQty = alert.ShortageQty
	existing.TriggerCount++
	existing.TriggeredAt = alert.TriggeredAt
	if err := conn.Save(&existing).Error; err != nil {
		return nil, fmt.Errorf("refresh reloaded stock alert: %w", err)
	}
	return &existing, nil
}

func (r *stockAlertRepository) applyScope(q *gorm.DB, scope AlertScope) *gorm.DB {
	if scope.StoreID > 0 {
		q = q.Where("store_id = ?", scope.StoreID)
	}
	if scope.Unread {
		q = q.Where("is_read = ?", false)
	}
	return q
}

func (r *stockAlertRepository) FindByIDForScope(id uint, scope AlertScope) (*model.StockAlert, error) {
	var alert model.StockAlert
	q := r.db.Model(&model.StockAlert{}).Preload("Store").Preload("SKU")
	q = r.applyScope(q, scope)
	if err := q.First(&alert, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("find stock alert by id: %w", util.ErrNotFound)
		}
		return nil, fmt.Errorf("find stock alert by id: %w", err)
	}
	return &alert, nil
}

func (r *stockAlertRepository) List(page, pageSize int, scope AlertScope) ([]model.StockAlert, int64, error) {
	var alerts []model.StockAlert
	var total int64
	q := r.db.Model(&model.StockAlert{})
	q = r.applyScope(q, scope)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count stock alerts: %w", err)
	}
	if err := q.Preload("Store").Preload("SKU").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Order("is_read ASC, triggered_at DESC").
		Find(&alerts).Error; err != nil {
		return nil, 0, fmt.Errorf("list stock alerts: %w", err)
	}
	return alerts, total, nil
}

func (r *stockAlertRepository) CountUnread(scope AlertScope) (int64, error) {
	var total int64
	scope.Unread = true
	q := r.db.Model(&model.StockAlert{})
	q = r.applyScope(q, scope)
	if err := q.Count(&total).Error; err != nil {
		return 0, fmt.Errorf("count unread stock alerts: %w", err)
	}
	return total, nil
}

func (r *stockAlertRepository) MarkReadByID(id uint) error {
	res := r.db.Model(&model.StockAlert{}).
		Where("id = ? AND is_read = ?", id, false).
		Updates(map[string]any{"is_read": true, "read_at": gorm.Expr("NOW()")})
	if res.Error != nil {
		return fmt.Errorf("mark stock alert read: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("mark stock alert read: %w", util.ErrNotFound)
	}
	return nil
}

func (r *stockAlertRepository) MarkAllRead(scope AlertScope) (int64, error) {
	q := r.db.Model(&model.StockAlert{}).Where("is_read = ?", false)
	if scope.StoreID > 0 {
		q = q.Where("store_id = ?", scope.StoreID)
	}
	res := q.Updates(map[string]any{"is_read": true, "read_at": gorm.Expr("NOW()")})
	if res.Error != nil {
		return 0, fmt.Errorf("mark all stock alerts read: %w", res.Error)
	}
	return res.RowsAffected, nil
}
