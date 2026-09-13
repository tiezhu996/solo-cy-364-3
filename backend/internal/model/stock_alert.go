package model

import "time"

// StockAlert 库存预警通知。库存低于安全线时生成；同一门店 + 同一 SKU
// 只保留最新一条未读通知（再次触发时原地刷新缺口与触发时间）。
// 未读唯一性由部分唯一索引 uniq_alert_store_sku_unread 保证
// （WHERE is_read = false，见 database/init.sql 与 router.migrate）。
type StockAlert struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	StoreID      uint       `gorm:"index;not null" json:"store_id"`
	Store        *Store     `gorm:"foreignKey:StoreID" json:"store,omitempty"`
	SKUID        uint       `gorm:"column:sku_id;index;not null" json:"sku_id"`
	SKU          *SKU       `gorm:"foreignKey:SKUID" json:"sku,omitempty"`
	Quantity     int        `gorm:"not null;default:0" json:"quantity"`
	SafetyStock  int        `gorm:"not null;default:0" json:"safety_stock"`
	ShortageQty  int        `gorm:"not null;default:0" json:"shortage_qty"`
	TriggerCount int        `gorm:"not null;default:1" json:"trigger_count"`
	IsRead       bool       `gorm:"index;not null;default:false" json:"is_read"`
	TriggeredAt  time.Time  `gorm:"index;not null" json:"triggered_at"`
	ReadAt       *time.Time `json:"read_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}
