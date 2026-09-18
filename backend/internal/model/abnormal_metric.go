package model

import "time"

// AbnormalMetric 异常指标。
type AbnormalMetric struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	ExamineeID      uint      `gorm:"index;not null" json:"examinee_id"`
	PackageItemID   uint      `gorm:"index;not null" json:"package_item_id"`
	AbnormalLevel   string    `gorm:"size:20;not null" json:"abnormal_level"`
	Value           string    `gorm:"size:100" json:"value"`
	RefValueRange   string    `gorm:"size:100" json:"ref_value_range"`
	TrendJSON       string    `gorm:"type:text" json:"trend_json"`
	FollowUpStatus  string    `gorm:"size:20;default:pending" json:"follow_up_status"`
	SpecialistAdvice string   `gorm:"size:500" json:"specialist_advice"`
	CreatedAt       time.Time `json:"created_at"`
	// FollowUpPriority 复查到期分级，按记录时间动态计算，不落库。
	FollowUpPriority string     `gorm:"-" json:"follow_up_priority"`
	PackageItem     PackageItem `gorm:"foreignKey:PackageItemID" json:"package_item,omitempty"`
}
