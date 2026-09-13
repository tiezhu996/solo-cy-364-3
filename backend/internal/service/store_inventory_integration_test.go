//go:build integration

package service

import (
	"log/slog"
	"os"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/ld/storeinventory/internal/constants"
	"github.com/ld/storeinventory/internal/model"
	"github.com/ld/storeinventory/internal/repository"
	"github.com/ld/storeinventory/internal/util"
)

// failingAlertService 让事务内的预警写入失败：在同一 tx 上执行非法 SQL，
// 强制事务进入 aborted 状态，从而验证外层业务必须整体回滚。
type failingAlertService struct{}

func (failingAlertService) TriggerAfterChangeTx(tx *gorm.DB, storeID, skuID uint) error {
	if err := tx.Exec("SELECT * FROM table_that_does_not_exist").Error; err != nil {
		return err
	}
	return nil
}
func (failingAlertService) TriggerAfterChange(storeID, skuID uint) error { return nil }
func (failingAlertService) List(viewer *util.JWTClaims, page, pageSize int, view constants.AlertView) ([]model.StockAlert, int64, error) {
	return nil, 0, nil
}
func (failingAlertService) UnreadCount(viewer *util.JWTClaims) (int64, error) { return 0, nil }
func (failingAlertService) MarkRead(viewer *util.JWTClaims, id uint) error    { return nil }
func (failingAlertService) MarkAllRead(viewer *util.JWTClaims) (int64, error) {
	return 0, nil
}

// TestSetSafetyStockAtomicRollback 验证：写预警失败时，安全库存不被部分保存。
func TestSetSafetyStockAtomicRollback(t *testing.T) {
	dsn := os.Getenv("TEST_DSN")
	if dsn == "" {
		t.Skip("TEST_DSN not set, skip integration test")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.Store{}, &model.SKU{}, &model.StoreInventory{}, &model.StockAlert{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	db.Exec("TRUNCATE stock_alerts, store_inventories, skus, stores RESTART IDENTITY CASCADE")

	store := model.Store{Code: "IT_ST001", Name: "集成测试门店"}
	sku := model.SKU{Code: "IT_SKU1", Name: "集成测试商品"}
	if err := db.Create(&store).Error; err != nil {
		t.Fatalf("create store: %v", err)
	}
	if err := db.Create(&sku).Error; err != nil {
		t.Fatalf("create sku: %v", err)
	}
	inv := model.StoreInventory{StoreID: store.ID, SKUID: sku.ID, Quantity: 10, SafetyStock: 5}
	if err := db.Create(&inv).Error; err != nil {
		t.Fatalf("create inventory: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	invRepo := repository.NewStoreInventoryRepository(db)
	skuRepo := repository.NewSKURepository(db)
	alertRepo := repository.NewStockAlertRepository(db)

	// 1) 预警写入失败：必须整体回滚。
	faultySvc := NewStoreInventoryService(invRepo, skuRepo, failingAlertService{}, db, logger)
	if _, err := faultySvc.SetSafetyStock(inv.ID, 20); err == nil {
		t.Fatal("expected error when alert write fails, got nil")
	}
	var after model.StoreInventory
	if err := db.First(&after, inv.ID).Error; err != nil {
		t.Fatalf("reload inventory: %v", err)
	}
	if after.SafetyStock != 5 {
		t.Fatalf("half-save detected: safety_stock changed to %d, want 5 (rollback)", after.SafetyStock)
	}
	var alertCount int64
	db.Model(&model.StockAlert{}).Where("store_id = ? AND sku_id = ?", store.ID, sku.ID).Count(&alertCount)
	if alertCount != 0 {
		t.Fatalf("half-save detected: %d alert rows written, want 0 (rollback)", alertCount)
	}

	// 2) 正常路径：保存与预警同事务成功。
	goodSvc := NewStoreInventoryService(
		invRepo, skuRepo, NewStockAlertService(alertRepo, invRepo, logger), db, logger)
	updated, err := goodSvc.SetSafetyStock(inv.ID, 20)
	if err != nil {
		t.Fatalf("successful set safety stock: %v", err)
	}
	if updated.SafetyStock != 20 {
		t.Fatalf("safety_stock=%d, want 20", updated.SafetyStock)
	}
	db.Model(&model.StockAlert{}).Where("store_id = ? AND sku_id = ? AND is_read = ?", store.ID, sku.ID, false).Count(&alertCount)
	if alertCount != 1 {
		t.Fatalf("expected exactly 1 unread alert, got %d", alertCount)
	}
}
