package service

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/ld/storeinventory/internal/constants"
	"github.com/ld/storeinventory/internal/model"
	"github.com/ld/storeinventory/internal/repository"
	"github.com/ld/storeinventory/internal/util"
)

// memoryAlertRepo 内存版预警仓储，用于验证未读去重与已读语义。
type memoryAlertRepo struct {
	rows []*model.StockAlert
	seq  uint
}

func (m *memoryAlertRepo) key(a *model.StockAlert) string {
	return string(rune(a.StoreID)) + ":" + string(rune(a.SKUID))
}

func (m *memoryAlertRepo) UpsertUnread(a *model.StockAlert) (*model.StockAlert, bool, error) {
	return m.UpsertUnreadTx(nil, a)
}
func (m *memoryAlertRepo) UpsertUnreadTx(tx *gorm.DB, a *model.StockAlert) (*model.StockAlert, bool, error) {
	for _, r := range m.rows {
		if !r.IsRead && r.StoreID == a.StoreID && r.SKUID == a.SKUID {
			r.Quantity = a.Quantity
			r.SafetyStock = a.SafetyStock
			r.ShortageQty = a.ShortageQty
			r.TriggerCount++
			r.TriggeredAt = a.TriggeredAt
			return r, false, nil
		}
	}
	m.seq++
	a.ID = m.seq
	a.IsRead = false
	a.TriggerCount = 1
	cp := *a
	m.rows = append(m.rows, &cp)
	return a, true, nil
}

func (m *memoryAlertRepo) FindByIDForScope(id uint, scope repository.AlertScope) (*model.StockAlert, error) {
	for _, r := range m.rows {
		if r.ID == id && (scope.StoreID == 0 || r.StoreID == scope.StoreID) {
			return r, nil
		}
	}
	return nil, util.ErrNotFound
}

func (m *memoryAlertRepo) List(page, pageSize int, scope repository.AlertScope) ([]model.StockAlert, int64, error) {
	var out []model.StockAlert
	for _, r := range m.rows {
		if scope.StoreID != 0 && r.StoreID != scope.StoreID {
			continue
		}
		if scope.Unread && r.IsRead {
			continue
		}
		cp := *r
		out = append(out, cp)
	}
	return out, int64(len(out)), nil
}

func (m *memoryAlertRepo) CountUnread(scope repository.AlertScope) (int64, error) {
	var n int64
	for _, r := range m.rows {
		if !r.IsRead && (scope.StoreID == 0 || r.StoreID == scope.StoreID) {
			n++
		}
	}
	return n, nil
}

func (m *memoryAlertRepo) MarkReadByID(id uint) error {
	for _, r := range m.rows {
		if r.ID == id && !r.IsRead {
			r.IsRead = true
			now := time.Now()
			r.ReadAt = &now
			return nil
		}
	}
	return util.ErrNotFound
}

func (m *memoryAlertRepo) MarkAllRead(scope repository.AlertScope) (int64, error) {
	var n int64
	now := time.Now()
	for _, r := range m.rows {
		if !r.IsRead && (scope.StoreID == 0 || r.StoreID == scope.StoreID) {
			r.IsRead = true
			r.ReadAt = &now
			n++
		}
	}
	return n, nil
}

func newTestAlertService() (StockAlertService, *memoryAlertRepo, *mockInventoryRepo) {
	invRepo := newMockInventoryRepo()
	alertRepo := &memoryAlertRepo{}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	return NewStockAlertService(alertRepo, invRepo, logger), alertRepo, invRepo
}

// setStock 直接设置测试库存水位。
func setStock(repo *mockInventoryRepo, storeID, skuID uint, qty, safety int) {
	k := keyOf(storeID, skuID)
	repo.items[k] = &model.StoreInventory{StoreID: storeID, SKUID: skuID, Quantity: qty, SafetyStock: safety}
}

func hqViewer() *util.JWTClaims {
	return &util.JWTClaims{UserID: 1, Username: "hq", Role: constants.RoleHQ}
}

func managerViewer(storeID uint) *util.JWTClaims {
	return &util.JWTClaims{UserID: 2, Username: "mgr", Role: constants.RoleStoreManager, StoreID: &storeID}
}

func TestAlertTriggerCreatesThenRefreshesSingleUnread(t *testing.T) {
	svc, alertRepo, invRepo := newTestAlertService()
	setStock(invRepo, 1, 10, 5, 20) // 缺口 15

	if err := svc.TriggerAfterChange(1, 10); err != nil {
		t.Fatalf("first trigger: %v", err)
	}
	if len(alertRepo.rows) != 1 || alertRepo.rows[0].ShortageQty != 15 || alertRepo.rows[0].TriggerCount != 1 {
		t.Fatalf("unexpected first alert: %+v", alertRepo.rows)
	}
	firstID := alertRepo.rows[0].ID

	// 库存继续下降到 2，缺口扩大为 18：应原地刷新同一条未读，而非新增。
	setStock(invRepo, 1, 10, 2, 20)
	if err := svc.TriggerAfterChange(1, 10); err != nil {
		t.Fatalf("second trigger: %v", err)
	}
	unread, _, _ := alertRepo.List(1, 100, repository.AlertScope{Unread: true})
	if len(unread) != 1 {
		t.Fatalf("expected exactly 1 unread, got %d", len(unread))
	}
	if unread[0].ID != firstID || unread[0].ShortageQty != 18 || unread[0].TriggerCount != 2 {
		t.Fatalf("unread not refreshed in place: %+v", unread[0])
	}
}

func TestAlertMarkReadThenTriggerCreatesNew(t *testing.T) {
	svc, alertRepo, invRepo := newTestAlertService()
	setStock(invRepo, 1, 10, 5, 20)
	if err := svc.TriggerAfterChange(1, 10); err != nil {
		t.Fatalf("trigger: %v", err)
	}
	if err := svc.MarkRead(hqViewer(), alertRepo.rows[0].ID); err != nil {
		t.Fatalf("mark read: %v", err)
	}
	// 再次低于安全线：已读历史保留，同时生成一条新的未读。
	if err := svc.TriggerAfterChange(1, 10); err != nil {
		t.Fatalf("re-trigger: %v", err)
	}
	all, total, _ := alertRepo.List(1, 100, repository.AlertScope{})
	unread, unreadTotal, _ := alertRepo.List(1, 100, repository.AlertScope{Unread: true})
	if total != 2 || unreadTotal != 1 {
		t.Fatalf("expected 2 total / 1 unread, got total=%d unread=%d", total, unreadTotal)
	}
	if len(all) != 2 || len(unread) != 1 {
		t.Fatalf("expected read history + 1 fresh unread")
	}
}

func TestAlertNoTriggerWhenAtOrAboveSafety(t *testing.T) {
	svc, alertRepo, invRepo := newTestAlertService()
	setStock(invRepo, 1, 10, 20, 20)
	if err := svc.TriggerAfterChange(1, 10); err != nil {
		t.Fatalf("trigger: %v", err)
	}
	if len(alertRepo.rows) != 0 {
		t.Fatalf("expected no alert when quantity >= safety, got %d", len(alertRepo.rows))
	}
}

func TestAlertManagerScopeIsOwnStoreOnly(t *testing.T) {
	svc, alertRepo, invRepo := newTestAlertService()
	setStock(invRepo, 1, 10, 1, 10)
	setStock(invRepo, 2, 10, 1, 10)
	if err := svc.TriggerAfterChange(1, 10); err != nil {
		t.Fatalf("trigger store1: %v", err)
	}
	if err := svc.TriggerAfterChange(2, 10); err != nil {
		t.Fatalf("trigger store2: %v", err)
	}

	hqCount, _ := svc.UnreadCount(hqViewer())
	if hqCount != 2 {
		t.Fatalf("hq should see 2 stores' alerts, got %d", hqCount)
	}
	mgrCount, _ := svc.UnreadCount(managerViewer(1))
	if mgrCount != 1 {
		t.Fatalf("manager should see only own store (1), got %d", mgrCount)
	}

	// 店长只能标记本店通知，标记他店应失败。
	var otherID uint
	for _, r := range alertRepo.rows {
		if r.StoreID == 2 {
			otherID = r.ID
		}
	}
	if err := svc.MarkRead(managerViewer(1), otherID); err == nil {
		t.Fatalf("manager must not mark another store's alert")
	}

	// 全部已读只作用于本店：店 2 仍有 1 条未读。
	if _, err := svc.MarkAllRead(managerViewer(1)); err != nil {
		t.Fatalf("mark all read: %v", err)
	}
	if n, _ := svc.UnreadCount(managerViewer(1)); n != 0 {
		t.Fatalf("own store unread should be 0, got %d", n)
	}
	if n, _ := svc.UnreadCount(hqViewer()); n != 1 {
		t.Fatalf("other store unread should remain 1, hq sees %d", n)
	}
}
