package service

import (
	"context"
	"testing"
	"time"

	"errors"
	"github.com/blueship581/gbcheckup/internal/constants"
	"github.com/blueship581/gbcheckup/internal/model"
	"github.com/blueship581/gbcheckup/internal/repository"
	"github.com/blueship581/gbcheckup/internal/util"
)

func TestFollowUpPriority(t *testing.T) {
	now := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name      string
		status    string
		createdAt time.Time
		want      string
	}{
		{"待复查满7天-高优先级", constants.FollowUpPending, now.AddDate(0, 0, -7), constants.FollowUpPriorityHigh},
		{"待复查超过7天-高优先级（历史记录）", constants.FollowUpPending, now.AddDate(0, 0, -30), constants.FollowUpPriorityHigh},
		{"待复查仅6天-普通", constants.FollowUpPending, now.AddDate(0, 0, -6), constants.FollowUpPriorityNormal},
		{"待复查1天-普通", constants.FollowUpPending, now.AddDate(0, 0, -1), constants.FollowUpPriorityNormal},
		{"已复查满7天-普通（沉底）", constants.FollowUpDone, now.AddDate(0, 0, -30), constants.FollowUpPriorityNormal},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := FollowUpPriority(tc.status, tc.createdAt, now); got != tc.want {
				t.Fatalf("FollowUpPriority() = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestAbnormalMetricService_ListPrioritySorting(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	item := model.PackageItem{ItemName: "血糖", RefValueRange: "3.9-6.1", Department: "检验科"}
	if err := db.Create(&item).Error; err != nil {
		t.Fatal(err)
	}

	// doneOld：已复查、记录于 30 天前（必须沉底）
	doneOld := &model.AbnormalMetric{PackageItemID: item.ID, AbnormalLevel: constants.AbnormalMild, Value: "1", FollowUpStatus: constants.FollowUpDone, SpecialistAdvice: "已建议复诊"}
	// pendingRecent：待复查、1 天前（普通优先级）
	pendingRecent := &model.AbnormalMetric{PackageItemID: item.ID, AbnormalLevel: constants.AbnormalMild, Value: "2", FollowUpStatus: constants.FollowUpPending}
	// pendingOldest：待复查、10 天前（高优先级，且比 pendingOld8 更早记录，应排最前）
	pendingOldest := &model.AbnormalMetric{PackageItemID: item.ID, AbnormalLevel: constants.AbnormalSevere, Value: "3", FollowUpStatus: constants.FollowUpPending}
	// pendingOld8：待复查、恰好 8 天前（高优先级）
	pendingOld8 := &model.AbnormalMetric{PackageItemID: item.ID, AbnormalLevel: constants.AbnormalModerate, Value: "4", FollowUpStatus: constants.FollowUpPending}
	for _, m := range []*model.AbnormalMetric{doneOld, pendingRecent, pendingOldest, pendingOld8} {
		if err := db.Create(m).Error; err != nil {
			t.Fatal(err)
		}
	}
	agedAt := func(days int) time.Time { return time.Now().AddDate(0, 0, -days) }
	db.Exec("UPDATE abnormal_metrics SET created_at = ? WHERE id = ?", agedAt(30), doneOld.ID)
	db.Exec("UPDATE abnormal_metrics SET created_at = ? WHERE id = ?", agedAt(1), pendingRecent.ID)
	db.Exec("UPDATE abnormal_metrics SET created_at = ? WHERE id = ?", agedAt(10), pendingOldest.ID)
	db.Exec("UPDATE abnormal_metrics SET created_at = ? WHERE id = ?", agedAt(8), pendingOld8.ID)

	repo := repository.NewAbnormalMetricRepository(db)
	svc := NewAbnormalMetricService(repo, testLogger())

	items, total, err := svc.List(ctx, 0, 1, 20)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if total != 4 {
		t.Fatalf("total = %d, want 4", total)
	}
	wantOrder := []uint{pendingOldest.ID, pendingOld8.ID, pendingRecent.ID, doneOld.ID}
	for i, wantID := range wantOrder {
		if items[i].ID != wantID {
			t.Fatalf("position %d got metric id %d, want %d", i, items[i].ID, wantID)
		}
	}
	// 高优先级标签（含历史记录按记录时间计算）。
	priorityByID := map[uint]string{}
	for _, m := range items {
		priorityByID[m.ID] = m.FollowUpPriority
	}
	if priorityByID[pendingOldest.ID] != constants.FollowUpPriorityHigh {
		t.Fatal("10 天前待复查应为高优先级")
	}
	if priorityByID[pendingOld8.ID] != constants.FollowUpPriorityHigh {
		t.Fatal("8 天前待复查应为高优先级")
	}
	if priorityByID[pendingRecent.ID] != constants.FollowUpPriorityNormal {
		t.Fatal("1 天前待复查应为普通优先级")
	}
	if priorityByID[doneOld.ID] != constants.FollowUpPriorityNormal {
		t.Fatal("已复查记录必须为普通优先级并沉底")
	}
}

func TestAbnormalMetricService_UpdateDoneRequiresAdvice(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	item := model.PackageItem{ItemName: "血糖", RefValueRange: "3.9-6.1", Department: "检验科"}
	if err := db.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	m := &model.AbnormalMetric{PackageItemID: item.ID, AbnormalLevel: constants.AbnormalModerate, Value: "72", FollowUpStatus: constants.FollowUpPending}
	if err := db.Create(m).Error; err != nil {
		t.Fatal(err)
	}

	repo := repository.NewAbnormalMetricRepository(db)
	svc := NewAbnormalMetricService(repo, testLogger())

	// 缺少专科建议：接口（service）拒绝。
	if _, err := svc.UpdateFollowUp(ctx, m.ID, constants.FollowUpDone, "   "); err == nil {
		t.Fatal("expected error when done without advice")
	} else {
		var appErr *util.AppError
		if !errors.As(err, &appErr) || appErr.Code != constants.CodeAdviceRequired {
			t.Fatalf("err = %v, want CodeAdviceRequired(%d)", err, constants.CodeAdviceRequired)
		}
	}
	// 状态与建议均保持原样。
	after, err := repo.FindByID(m.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.FollowUpStatus != constants.FollowUpPending || after.SpecialistAdvice != "" {
		t.Fatalf("record changed after rejected update: status=%s advice=%q", after.FollowUpStatus, after.SpecialistAdvice)
	}

	// 非法状态同样拒绝。
	if _, err := svc.UpdateFollowUp(ctx, m.ID, "weird", "建议"); err == nil {
		t.Fatal("expected error for invalid status")
	}

	// 状态与建议一次保存成功。
	updated, err := svc.UpdateFollowUp(ctx, m.ID, constants.FollowUpDone, "建议内分泌科就诊")
	if err != nil {
		t.Fatalf("UpdateFollowUp() error = %v", err)
	}
	if updated.FollowUpStatus != constants.FollowUpDone || updated.SpecialistAdvice != "建议内分泌科就诊" {
		t.Fatalf("updated record = %+v", updated)
	}
	if updated.FollowUpPriority != constants.FollowUpPriorityNormal {
		t.Fatal("已复查记录优先级应为普通")
	}
}
