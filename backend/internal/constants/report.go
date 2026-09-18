package constants

// AbnormalLevel 异常等级枚举（README 枚举出现位置清单必列）。
const (
	AbnormalMild     = "mild"     // 轻度异常
	AbnormalModerate = "moderate" // 中度异常
	AbnormalSevere   = "severe"   // 重度异常
)

// AbnormalLevels 全部异常等级。
var AbnormalLevels = []string{AbnormalMild, AbnormalModerate, AbnormalSevere}

// ReportStatus 报告状态枚举（README 枚举出现位置清单必列）。
const (
	ReportDraft     = "draft"     // 草稿
	ReportGenerated = "generated" // 已生成
	ReportReviewed  = "reviewed"  // 已审核
	ReportPublished = "published" // 已发布
)

// ReportStatuses 全部报告状态。
var ReportStatuses = []string{ReportDraft, ReportGenerated, ReportReviewed, ReportPublished}

// ResultStatus 检查结果状态。
const (
	ResultPending  = "pending"  // 待录入
	ResultEntered  = "entered"  // 已录入
	ResultReviewed = "reviewed" // 已审核
)

// RegistrationStatus 登记状态。
const (
	RegistrationRegistered = "registered" // 已登记
	RegistrationInProgress = "in_progress" // 进行中
	RegistrationCompleted  = "completed"  // 已完成
)

// PackageStatus 套餐状态。
const (
	PackageActive   = "active"
	PackageInactive = "inactive"
)

// PackageType 套餐类型。
const (
	PackageEntry   = "entry"   // 入职体检
	PackageAnnual  = "annual"  // 年度体检
	PackagePremium = "premium" // 高端体检
	PackageOther   = "other"   // 其他
)

// GroupOrderStatus 团检订单状态。
const (
	GroupOrderPending   = "pending"
	GroupOrderConfirmed = "confirmed"
	GroupOrderDone      = "done"
)

// FollowUpStatus 复查跟踪状态。
const (
	FollowUpPending = "pending"
	FollowUpDone    = "done"
)

// FollowUpPriority 复查到期分级（依据记录时间动态计算，不落库）。
const (
	FollowUpPriorityHigh   = "high"   // 高优先级：待复查且距记录时间已满复查期限
	FollowUpPriorityNormal = "normal" // 普通优先级：未到期或已复查
)

// FollowUpOverdueDays 复查到期天数：待复查从记录时间起满 7 天为高优先级。
const FollowUpOverdueDays = 7
