import { FollowUpPriority, FOLLOW_UP_OVERDUE_DAYS } from '../constants/report';

// 复查到期分级：待复查（pending）自记录日起满 7 个自然日为高优先级。
// 历史记录同样按记录时间计算；已复查始终为普通优先级（列表中沉底）。
export function isFollowUpOverdue(
  status: string,
  createdAt?: string | null,
  now: Date = new Date()
): boolean {
  if (status !== 'pending' || !createdAt) return false;
  const recordedAt = new Date(createdAt);
  if (Number.isNaN(recordedAt.getTime())) return false;
  const dayStart = (d: Date) => new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime();
  const overdueBoundary = new Date(now);
  overdueBoundary.setDate(overdueBoundary.getDate() - FOLLOW_UP_OVERDUE_DAYS);
  return dayStart(recordedAt) <= dayStart(overdueBoundary);
}

export function followUpPriority(
  status: string,
  createdAt?: string | null,
  now: Date = new Date()
): string {
  return isFollowUpOverdue(status, createdAt, now)
    ? FollowUpPriority.HIGH
    : FollowUpPriority.NORMAL;
}
