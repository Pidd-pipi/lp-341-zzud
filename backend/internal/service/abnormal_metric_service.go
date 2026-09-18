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

// FollowUpHighPriorityBefore 复查到期分级阈值：记录时间不晚于该时间的待复查即为高优先级。
func FollowUpHighPriorityBefore(now time.Time) time.Time {
	return now.AddDate(0, 0, -constants.FollowUpHighPriorityDays)
}

// List 分页查询异常指标，并按记录时间标记复查到期的高优先级（历史记录同样参与计算）。
func (s *AbnormalMetricService) List(ctx context.Context, examineeID uint, page, pageSize int) ([]model.AbnormalMetric, int64, error) {
	highPriorityBefore := FollowUpHighPriorityBefore(time.Now())
	items, total, err := s.repo.List(examineeID, page, pageSize, highPriorityBefore)
	if err != nil {
		return nil, 0, err
	}
	for i := range items {
		items[i].HighPriority = items[i].FollowUpStatus == constants.FollowUpPending && !items[i].CreatedAt.After(highPriorityBefore)
	}
	return items, total, nil
}

// UpdateFollowUp 更新复查跟踪与专科建议。标记已复查必须填写专科建议；
// 状态与建议在同一事务内一次保存，任一步失败都保持原样。
func (s *AbnormalMetricService) UpdateFollowUp(ctx context.Context, id uint, status, advice string) (*model.AbnormalMetric, error) {
	if status != constants.FollowUpPending && status != constants.FollowUpDone {
		return nil, util.BadRequest("复查状态（AbnormalMetric.follow_up_status）不合法", errors.New("invalid status"))
	}
	advice = strings.TrimSpace(advice)
	if status == constants.FollowUpDone && advice == "" {
		return nil, util.BadRequest(constants.MsgFollowUpAdviceRequired, errors.New("specialist advice required when follow-up done"))
	}
	err := s.repo.Transaction(func(tx *gorm.DB) error {
		return s.repo.WithTx(tx).UpdateFollowUp(id, status, advice)
	})
	if err != nil {
		return nil, util.LogError(s.log, constants.LOG_ABNORMAL_METRIC_FOLLOWUP, fmt.Errorf("update follow-up: %w", err))
	}
	m, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	s.log.InfoContext(ctx, constants.LOG_ABNORMAL_METRIC_FOLLOWUP, "metric_id", id, "status", status)
	return m, nil
}
