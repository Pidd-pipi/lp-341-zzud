import { useCallback, useEffect, useState } from 'react';
import { Card, Form, Input, Modal, Select, Space, Table, Tag, message } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { listAbnormalMetrics, updateFollowUp } from '../api/abnormalMetric';
import type { AbnormalMetric } from '../types';
import AbnormalTag from '../components/common/AbnormalTag';
import {
  FollowUpPriority,
  FollowUpPriorityLabels,
  FollowUpStatusLabels,
} from '../constants/report';
import { isFollowUpOverdue } from '../utils/followUpPriority';
import { formatReferenceRange } from '../utils/formatReferenceRange';
import { formatDateTime } from '../utils/dateFormat';
import { usePagination } from '../hooks/usePagination';

// 高优先级标签：待复查且距记录时间满 7 天（历史记录同样按记录时间计算）。
function PriorityTag({ record }: { record: AbnormalMetric }) {
  if (!isFollowUpOverdue(record.follow_up_status, record.created_at)) return null;
  return <Tag color="red" style={{ marginLeft: 8 }}>{FollowUpPriorityLabels[FollowUpPriority.HIGH]}</Tag>;
}

export default function AbnormalMetricTrack() {
  const [items, setItems] = useState<AbnormalMetric[]>([]);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [examineeId, setExamineeId] = useState('');
  const { pagination, setTotal, onPageChange } = usePagination(1, 10);
  const total = pagination.total;
  const [current, setCurrent] = useState<AbnormalMetric | null>(null);
  const [status, setStatus] = useState('pending');
  const [advice, setAdvice] = useState('');

  const load = useCallback(async (page = pagination.page, size = pagination.pageSize) => {
    setLoading(true);
    try {
      const data = await listAbnormalMetrics({ examinee_id: Number(examineeId) || undefined, page, page_size: size });
      // 后端已按“高优先级在前、已复查沉底”排序，这里再做一次同规则兜底排序，
      // 保证历史记录/旧缓存数据展示顺序一致。
      const sorted = [...data.list].sort((a, b) => {
        const rank = (m: AbnormalMetric) =>
          isFollowUpOverdue(m.follow_up_status, m.created_at)
            ? 0
            : m.follow_up_status === 'pending'
              ? 1
              : 2;
        const ra = rank(a);
        const rb = rank(b);
        if (ra !== rb) return ra - rb;
        const ta = new Date(a.created_at).getTime() || 0;
        const tb = new Date(b.created_at).getTime() || 0;
        // 待复查：越早记录越靠前；已复查：越近复查越靠后。
        return a.follow_up_status === 'pending' ? ta - tb : tb - ta;
      });
      setItems(sorted);
      setTotal(data.total);
    } finally {
      setLoading(false);
    }
  }, [pagination.page, pagination.pageSize, setTotal, examineeId]);

  useEffect(() => { load(); }, [load]);

  async function onSave() {
    if (!current) return;
    const trimmedAdvice = advice.trim();
    // 标记已复查必须填写专科建议：界面侧先拦截，接口也会拒绝。
    if (status === 'done' && !trimmedAdvice) {
      message.error('标记已复查必须填写专科建议');
      return;
    }
    setSaving(true);
    try {
      // 状态与建议一次提交，任一保存失败则列表重新拉取、数据保持原样。
      await updateFollowUp(current.id, { status, specialist_advice: trimmedAdvice });
      message.success('复查跟踪已更新');
      setCurrent(null);
      await load();
    } finally {
      setSaving(false);
    }
  }

  const columns: ColumnsType<AbnormalMetric> = [
    { title: '指标', render: (_, r) => r.package_item?.item_name ?? '-' },
    { title: '异常等级', dataIndex: 'abnormal_level', render: (v) => <AbnormalTag level={v} /> },
    { title: '结果值', dataIndex: 'value' },
    { title: '参考值', dataIndex: 'ref_value_range', render: (v) => formatReferenceRange(v) },
    {
      title: '复查状态',
      dataIndex: 'follow_up_status',
      render: (v, r) => (
        <Space size={0}>
          <Tag color={v === 'done' ? 'green' : 'orange'}>{FollowUpStatusLabels[v] ?? v}</Tag>
          <PriorityTag record={r} />
        </Space>
      ),
    },
    { title: '专科建议', dataIndex: 'specialist_advice', render: (v) => v || '-' },
    { title: '记录时间', dataIndex: 'created_at', render: (v) => formatDateTime(v) },
    {
      title: '操作',
      render: (_, r) => (
        <a onClick={() => { setCurrent(r); setStatus(r.follow_up_status); setAdvice(r.specialist_advice); }}>跟踪</a>
      ),
    },
  ];

  return (
    <Card size="small" title="异常指标追踪">
      <Space direction="vertical" size={12} style={{ width: '100%' }}>
        <Input.Search placeholder="按体检人 ID 筛选" value={examineeId} onChange={(e) => setExamineeId(e.target.value)} style={{ width: 220 }} />
        <Table rowKey="id" columns={columns} dataSource={items} loading={loading} pagination={{ current: pagination.page, pageSize: pagination.pageSize, total, onChange: onPageChange }} />
      </Space>
      <Modal
        open={!!current}
        title={`复查跟踪：${current?.package_item?.item_name ?? ''}`}
        onOk={onSave}
        onCancel={() => setCurrent(null)}
        confirmLoading={saving}
        okButtonProps={{ danger: status === 'done' }}
        okText={status === 'done' ? '标记已复查' : '保存'}
        destroyOnClose
      >
        <Form layout="vertical" style={{ marginTop: 8 }}>
          <Form.Item label="结果值">
            <span>{current?.value}（参考 {formatReferenceRange(current?.ref_value_range)}）</span>
          </Form.Item>
          <Form.Item label="状态">
            <Select
              style={{ width: '100%' }}
              value={status}
              onChange={setStatus}
              options={Object.entries(FollowUpStatusLabels).map(([value, label]) => ({ value, label }))}
            />
          </Form.Item>
          <Form.Item
            label="专科建议"
            required={status === 'done'}
            validateStatus={status === 'done' && !advice.trim() ? 'error' : ''}
            help={status === 'done' && !advice.trim() ? '标记已复查必须填写专科建议' : undefined}
          >
            <Input.TextArea
              rows={3}
              value={advice}
              onChange={(e) => setAdvice(e.target.value)}
              placeholder={status === 'done' ? '请填写专科建议后再标记已复查' : '可填写复查、就诊建议'}
            />
          </Form.Item>
        </Form>
      </Modal>
    </Card>
  );
}
