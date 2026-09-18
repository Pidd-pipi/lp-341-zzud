package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/blueship581/gbcheckup/internal/constants"
	"github.com/blueship581/gbcheckup/internal/model"
	"github.com/blueship581/gbcheckup/internal/repository"
	"github.com/blueship581/gbcheckup/internal/util"
	"gorm.io/gorm"
)

// AbnormalMetricService 异常指标服务：记录、趋势对比、复查跟踪。
type AbnormalMetricService struct {
	repo *repository.AbnormalMetricRepository
	log  *slog.Logger
}

// NewAbnormalMetricService 构造异常指标服务。
func NewAbnormalMetricService(repo *repository.AbnormalMetricRepository, log *slog.Logger) *AbnormalMetricService {
	return &AbnormalMetricService{repo: repo, log: log}
}

// followUpLocation 复查到期计算使用的业务时区，与数据库会话 TimeZone 一致。
var followUpLocation = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.Local
	}
	return loc
}()

// FollowUpPriority 计算复查到期分级：
// 待复查（pending）自记录日（created_at 在 Asia/Shanghai 的日期）起满 7 个自然日为高优先级，
// 其余为普通优先级；历史记录同样按记录时间实时计算，无需迁移落库。
func FollowUpPriority(status string, createdAt time.Time, now time.Time) string {
	if status == constants.FollowUpPending && !createdAt.IsZero() {
		now = now.In(followUpLocation)
		createdAt = createdAt.In(followUpLocation)
		y1, m1, d1 := now.AddDate(0, 0, -constants.FollowUpOverdueDays).Date()
		y2, m2, d2 := createdAt.Date()
		recordDate := time.Date(y2, m2, d2, 0, 0, 0, 0, followUpLocation)
		cutoffDate := time.Date(y1, m1, d1, 0, 0, 0, 0, followUpLocation)
		if !recordDate.After(cutoffDate) {
			return constants.FollowUpPriorityHigh
		}
	}
	return constants.FollowUpPriorityNormal
}

func (s *AbnormalMetricService) fillPriority(items []model.AbnormalMetric) {
	now := time.Now()
	for i := range items {
		items[i].FollowUpPriority = FollowUpPriority(items[i].FollowUpStatus, items[i].CreatedAt, now)
	}
}

// List 分页查询异常指标（高优先级在前、已复查沉底，见仓储排序）。
func (s *AbnormalMetricService) List(ctx context.Context, examineeID uint, page, pageSize int) ([]model.AbnormalMetric, int64, error) {
	items, total, err := s.repo.List(examineeID, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	s.fillPriority(items)
	return items, total, nil
}

// UpdateFollowUp 更新复查跟踪与专科建议。
// 标记已复查（done）必须填写专科建议，否则返回校验错误、数据保持原样；
// 状态与建议在同一事务内一次保存，任一步失败整体回滚。
func (s *AbnormalMetricService) UpdateFollowUp(ctx context.Context, id uint, status, advice string) (*model.AbnormalMetric, error) {
	if status != constants.FollowUpPending && status != constants.FollowUpDone {
		err := util.BadRequest(constants.MsgFollowUpStatusInvalid, errors.New("invalid follow_up_status"))
		return nil, util.LogError(s.log, constants.LOG_ABNORMAL_METRIC_FOLLOWUP_REJECTED,
			fmt.Errorf("AbnormalMetric[id=%d] follow-up rejected by role: %w", id, err))
	}
	advice = strings.TrimSpace(advice)
	if status == constants.FollowUpDone && advice == "" {
		err := util.NewAppError(constants.CodeAdviceRequired, 400, constants.MsgAdviceRequired, errors.New("specialist_advice required"))
		return nil, util.LogError(s.log, constants.LOG_ABNORMAL_METRIC_FOLLOWUP_REJECTED,
			fmt.Errorf("AbnormalMetric[id=%d] follow-up rejected: %w", id, err))
	}
	// 先确认记录存在，避免对不存在的记录“静默成功”。
	if _, err := s.repo.FindByID(id); err != nil {
		if errors.Is(err, util.ErrNotFound) {
			return nil, util.NotFoundError(fmt.Sprintf("异常指标（AbnormalMetric[id=%d]）不存在", id), err)
		}
		return nil, err
	}
	err := s.repo.Transaction(func(tx *gorm.DB) error {
		if err := s.repo.WithTx(tx).UpdateFollowUp(id, status, advice); err != nil {
			return fmt.Errorf("update follow-up status and specialist_advice: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, util.LogError(s.log, constants.LOG_ABNORMAL_METRIC_FOLLOWUP, err)
	}
	m, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	m.FollowUpPriority = FollowUpPriority(m.FollowUpStatus, m.CreatedAt, time.Now())
	s.log.InfoContext(ctx, constants.LOG_ABNORMAL_METRIC_FOLLOWUP, "metric_id", id, "status", status)
	return m, nil
}
