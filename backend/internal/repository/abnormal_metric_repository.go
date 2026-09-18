package repository

import (
	"errors"
	"time"

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

// Transaction 在事务内执行 fn，任一步返回 error 则状态与建议整体回滚。
func (r *AbnormalMetricRepository) Transaction(fn func(tx *gorm.DB) error) error {
	return r.db.Transaction(fn)
}

func (r *AbnormalMetricRepository) Create(m *model.AbnormalMetric) error { return r.db.Create(m).Error }

// List 分页查询异常指标。
// 排序规则：待复查且距记录时间满 7 天（高优先级）置顶，其余待复查次之，已复查始终沉底；
// 待复查按记录时间升序（越早到期越靠前），已复查按记录时间倒序，同时间按 id 兜底。
func (r *AbnormalMetricRepository) List(examineeID uint, page, pageSize int) ([]model.AbnormalMetric, int64, error) {
	q := r.db.Model(&model.AbnormalMetric{})
	if examineeID > 0 {
		q = q.Where("examinee_id = ?", examineeID)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	// 到期分界：按数据库会话时区的日历日比较，created_at 的日期 <= 今天-7 即满 7 天。
	// 排序必须放在同一条 Raw ORDER BY 中：GORM 1.25 的 clause.OrderBy.Expression
	// 在与字符串 Order 合并时会被覆盖，而 Raw 列不做占位符替换，
	// 因此这里用 Dialector.Explain 完成参数转义后拼入。
	deadline := time.Now().AddDate(0, 0, -7)
	// 分组号：高优先级（待复查且满 7 天）0 → 其余待复查 1 → 已复查 2（已复查始终沉底）；
	// 待复查组记录日升序（越早到期越靠前），已复查组记录日倒序，同日按 id 兜底。
	orderBy := r.db.Dialector.Explain(
		"CASE WHEN follow_up_status = 'done' THEN 2 "+
			"WHEN follow_up_status = 'pending' AND date(created_at) <= date(?) THEN 0 ELSE 1 END ASC, "+
			"CASE WHEN follow_up_status = 'done' THEN created_at END DESC, "+
			"CASE WHEN follow_up_status = 'pending' THEN created_at END ASC, id ASC",
		deadline)
	var items []model.AbnormalMetric
	err := r.db.Preload("PackageItem").
		Model(&model.AbnormalMetric{}).
		Scopes(func(db *gorm.DB) *gorm.DB {
			if examineeID > 0 {
				return db.Where("examinee_id = ?", examineeID)
			}
			return db
		}).
		Order(orderBy).
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&items).Error
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
