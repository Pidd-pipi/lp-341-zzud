package service

import (
	"context"
	"testing"
	"time"

	"github.com/blueship581/gbcheckup/internal/constants"
	"github.com/blueship581/gbcheckup/internal/model"
	"github.com/blueship581/gbcheckup/internal/repository"
	"gorm.io/gorm"
)

// seedMetric 创建异常指标并将记录时间回拨到 createdAt（模拟历史记录）。
func seedMetric(t *testing.T, db *gorm.DB, examineeID, itemID uint, status string, createdAt time.Time) model.AbnormalMetric {
	t.Helper()
	m := model.AbnormalMetric{
		ExamineeID: examineeID, PackageItemID: itemID,
		AbnormalLevel: constants.AbnormalMild, Value: "9.9", RefValueRange: "3.9-6.1",
		FollowUpStatus: status,
	}
	if err := db.Create(&m).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE abnormal_metrics SET created_at = ? WHERE id = ?", createdAt, m.ID).Error; err != nil {
		t.Fatal(err)
	}
	return m
}

func newMetricSvc(t *testing.T) (*AbnormalMetricService, *repository.AbnormalMetricRepository, *gorm.DB, uint, uint) {
	t.Helper()
	db := newTestDB(t)
	item := model.PackageItem{ItemName: "血糖", RefValueRange: "3.9-6.1", Department: "检验科"}
	if err := db.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	examinee := model.Examinee{Name: "张三", IDCardNo: "110101199001011234"}
	if err := db.Create(&examinee).Error; err != nil {
		t.Fatal(err)
	}
	repo := repository.NewAbnormalMetricRepository(db)
	return NewAbnormalMetricService(repo, testLogger()), repo, db, examinee.ID, item.ID
}

// 待复查满七天（按记录时间，含历史记录）标记高优先级；列表高优先级置顶、已复查沉底。
func TestAbnormalMetricService_ListPriorityOrder(t *testing.T) {
	svc, _, db, examineeID, itemID := newMetricSvc(t)
	now := time.Now()
	overdue := seedMetric(t, db, examineeID, itemID, constants.FollowUpPending, now.AddDate(0, 0, -8))
	recent := seedMetric(t, db, examineeID, itemID, constants.FollowUpPending, now.AddDate(0, 0, -1))
	done := seedMetric(t, db, examineeID, itemID, constants.FollowUpDone, now.AddDate(0, 0, -30))

	items, total, err := svc.List(context.Background(), examineeID, 1, 10)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if total != 3 || len(items) != 3 {
		t.Fatalf("List() total = %d, len = %d, want 3", total, len(items))
	}
	wantOrder := []uint{overdue.ID, recent.ID, done.ID}
	wantPriority := []bool{true, false, false}
	for i, m := range items {
		if m.ID != wantOrder[i] {
			t.Fatalf("items[%d].ID = %d, want %d（高优先级置顶、已复查沉底）", i, m.ID, wantOrder[i])
		}
		if m.HighPriority != wantPriority[i] {
			t.Fatalf("items[%d].HighPriority = %v, want %v", i, m.HighPriority, wantPriority[i])
		}
	}
}

// 标记已复查缺少专科建议时接口拒绝，且状态与建议都保持原样。
func TestAbnormalMetricService_UpdateFollowUpDoneRequiresAdvice(t *testing.T) {
	svc, repo, db, examineeID, itemID := newMetricSvc(t)
	m := seedMetric(t, db, examineeID, itemID, constants.FollowUpPending, time.Now())
	if err := db.Model(&model.AbnormalMetric{}).Where("id = ?", m.ID).Update("specialist_advice", "旧建议").Error; err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	for _, advice := range []string{"", "   "} {
		if _, err := svc.UpdateFollowUp(ctx, m.ID, constants.FollowUpDone, advice); err == nil {
			t.Fatalf("UpdateFollowUp(done, %q) 应被拒绝", advice)
		}
		got, err := repo.FindByID(m.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.FollowUpStatus != constants.FollowUpPending || got.SpecialistAdvice != "旧建议" {
			t.Fatalf("拒绝后记录应保持原样，got status=%s advice=%s", got.FollowUpStatus, got.SpecialistAdvice)
		}
	}

	updated, err := svc.UpdateFollowUp(ctx, m.ID, constants.FollowUpDone, "建议心内科复查")
	if err != nil {
		t.Fatalf("UpdateFollowUp() error = %v", err)
	}
	if updated.FollowUpStatus != constants.FollowUpDone || updated.SpecialistAdvice != "建议心内科复查" {
		t.Fatalf("状态与建议应一次保存成功，got status=%s advice=%s", updated.FollowUpStatus, updated.SpecialistAdvice)
	}
	got, err := repo.FindByID(m.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.FollowUpStatus != constants.FollowUpDone || got.SpecialistAdvice != "建议心内科复查" {
		t.Fatalf("持久化结果不符，got status=%s advice=%s", got.FollowUpStatus, got.SpecialistAdvice)
	}
}
