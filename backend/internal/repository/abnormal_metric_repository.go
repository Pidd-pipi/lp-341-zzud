package repository

import (
	"errors"
	"fmt"
	"time"

	"github.com/blueship581/gbcheckup/internal/constants"
	"github.com/blueship581/gbcheckup/internal/model"
	"github.com/blueship581/gbcheckup/internal/util"
	"gorm.io/gorm"
)

// AbnormalMetricRepository 异常指标仓储。
type AbnormalMetricRepository struct{ db *gorm.DB }

// NewAbnormalMetricRepository 构造异常指标仓储。
func NewAbnormalMetricRepository(db *gorm.DB) *AbnormalMetricRepository {
	return &AbnormalMetricRepository{db: db}
}

// WithTx 使用事务连接构造仓储。
func (r *AbnormalMetricRepository) WithTx(tx *gorm.DB) *AbnormalMetricRepository {
	return &AbnormalMetricRepository{db: tx}
}

// Transaction 在事务内执行 fn，任一步返回 error 则整体回滚。
func (r *AbnormalMetricRepository) Transaction(fn func(tx *gorm.DB) error) error {
	return r.db.Transaction(fn)
}

func (r *AbnormalMetricRepository) Create(m *model.AbnormalMetric) error { return r.db.Create(m).Error }

// List 分页查询异常指标。排序：高优先级待复查（记录时间早于 highPriorityBefore）置顶，
// 普通待复查居中，已复查始终沉底；同级按 id 倒序。
func (r *AbnormalMetricRepository) List(examineeID uint, page, pageSize int, highPriorityBefore time.Time) ([]model.AbnormalMetric, int64, error) {
	q := r.db.Model(&model.AbnormalMetric{})
	if examineeID > 0 {
		q = q.Where("examinee_id = ?", examineeID)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	// 分级排序 CASE：0=高优先级待复查，1=普通待复查，2=已复查（沉底）。
	// 阈值与状态值均为后端内部常量/时间，无注入风险。
	priorityOrder := fmt.Sprintf(
		"CASE WHEN follow_up_status = '%s' THEN 2 WHEN follow_up_status = '%s' AND created_at <= '%s' THEN 0 ELSE 1 END, id DESC",
		constants.FollowUpDone, constants.FollowUpPending, highPriorityBefore.Format("2006-01-02 15:04:05.999999999-07:00"),
	)
	var items []model.AbnormalMetric
	err := q.Preload("PackageItem").Order(priorityOrder).Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error
	return items, total, err
}

func (r *AbnormalMetricRepository) FindByID(id uint) (*model.AbnormalMetric, error) {
	var m model.AbnormalMetric
	if err := r.db.Preload("PackageItem").First(&m, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, util.ErrNotFound
		}
		return nil, err
	}
	return &m, nil
}

func (r *AbnormalMetricRepository) UpdateFollowUp(id uint, status, advice string) error {
	return r.db.Model(&model.AbnormalMetric{}).Where("id = ?", id).Updates(map[string]any{
		"follow_up_status": status, "specialist_advice": advice,
	}).Error
}

func (r *AbnormalMetricRepository) Count() (int64, error) {
	var count int64
	err := r.db.Model(&model.AbnormalMetric{}).Count(&count).Error
	return count, err
}
