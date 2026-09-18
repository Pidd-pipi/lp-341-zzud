import { useCallback, useEffect, useState } from 'react';
import { Card, Input, Modal, Select, Space, Table, Tag, message } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { listAbnormalMetrics, updateFollowUp } from '../api/abnormalMetric';
import type { AbnormalMetric } from '../types';
import AbnormalTag from '../components/common/AbnormalTag';
import { FollowUpStatus, FollowUpStatusLabels, FOLLOW_UP_HIGH_PRIORITY_DAYS } from '../constants/report';
import { formatReferenceRange } from '../utils/formatReferenceRange';
import { formatDateTime } from '../utils/dateFormat';
import { usePagination } from '../hooks/usePagination';

export default function AbnormalMetricTrack() {
  const [items, setItems] = useState<AbnormalMetric[]>([]);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [examineeId, setExamineeId] = useState('');
  const { pagination, setTotal, onPageChange } = usePagination(1, 10);
  const total = pagination.total;
  const [current, setCurrent] = useState<AbnormalMetric | null>(null);
  const [status, setStatus] = useState<string>(FollowUpStatus.PENDING);
  const [advice, setAdvice] = useState('');

  // 标记已复查必须填写专科建议：界面拒绝（禁用确定 + 红框提示）
  const adviceMissing = status === FollowUpStatus.DONE && !advice.trim();

  const load = useCallback(async (page = pagination.page, size = pagination.pageSize) => {
    setLoading(true);
    try {
      const data = await listAbnormalMetrics({ examinee_id: Number(examineeId) || undefined, page, page_size: size });
      setItems(data.list);
      setTotal(data.total);
    } finally {
      setLoading(false);
    }
  }, [pagination.page, pagination.pageSize, setTotal, examineeId]);

  useEffect(() => { load(); }, [load]);

  async function onSave() {
    if (!current) return;
    if (adviceMissing) {
      message.error('标记已复查必须填写专科建议');
      return;
    }
    setSaving(true);
    try {
      await updateFollowUp(current.id, { status, specialist_advice: advice.trim() });
      message.success('复查跟踪已更新');
      setCurrent(null);
      load();
    } catch {
      // 接口拒绝或保存失败：拦截器已提示，状态与建议保持原样，弹窗不关闭
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
        <Space size={4}>
          <span>{FollowUpStatusLabels[v] ?? v}</span>
          {r.high_priority && <Tag color="red" title={`待复查自记录时间起已满 ${FOLLOW_UP_HIGH_PRIORITY_DAYS} 天`}>高优先级</Tag>}
        </Space>
      ),
    },
    { title: '专科建议', dataIndex: 'specialist_advice', render: (v) => v || '-' },
    { title: '记录时间', dataIndex: 'created_at', render: (v) => formatDateTime(v) },
    { title: '操作', render: (_, r) => <a onClick={() => { setCurrent(r); setStatus(r.follow_up_status); setAdvice(r.specialist_advice); }}>跟踪</a> },
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
        okButtonProps={{ disabled: adviceMissing, loading: saving }}
        onCancel={() => setCurrent(null)}
        destroyOnClose
      >
        <Space direction="vertical" style={{ width: '100%' }}>
          <div>结果值：{current?.value}（参考 {formatReferenceRange(current?.ref_value_range)}）</div>
          <div>状态：
            <Select style={{ width: 160 }} value={status} onChange={setStatus} options={Object.entries(FollowUpStatusLabels).map(([value, label]) => ({ value, label }))} />
          </div>
          <div>
            专科建议{status === FollowUpStatus.DONE && <span style={{ color: '#ff4d4f' }}>（标记已复查必填）</span>}：
            <Input.TextArea
              rows={3}
              value={advice}
              onChange={(e) => setAdvice(e.target.value)}
              status={adviceMissing ? 'error' : ''}
              placeholder={status === FollowUpStatus.DONE ? '必填：请填写专科就诊建议' : ''}
            />
          </div>
        </Space>
      </Modal>
    </Card>
  );
}
