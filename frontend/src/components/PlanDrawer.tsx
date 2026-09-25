import { Alert, Button, Checkbox, Descriptions, Drawer, Space, Tag } from 'antd';
import { SafetyCertificateOutlined, SendOutlined } from '@ant-design/icons';
import { useEffect, useState } from 'react';
import type { ControlCheckSnapshot, DomainRecord } from '../types/domain';
import { formatDate } from '../utils/format';
import { StatusBadge } from './common/StatusBadge';
import { ControlCheckPanel } from './ControlCheckPanel';

interface PlanDrawerProps {
  open: boolean;
  record: DomainRecord | null;
  kind: 'plan' | 'execution';
  username: string;
  role: string;
  busy?: boolean;
  error?: string;
  snapshot?: ControlCheckSnapshot | null;
  snapshotLive?: boolean;
  onClose: () => void;
  onRequest?: (record: DomainRecord) => Promise<void>;
  onConfirm?: (record: DomainRecord) => Promise<void>;
}

export function PlanDrawer({ open, record, kind, username, role, busy = false, error = '', snapshot = null, snapshotLive = false, onClose, onRequest, onConfirm }: PlanDrawerProps) {
  const [acknowledged, setAcknowledged] = useState(false);
  useEffect(() => setAcknowledged(false), [open, record?.id, record?.version, kind]);
  const blocked = Boolean(snapshot && snapshot.status === 'blocked');
  const canRequest = kind === 'execution' && record?.status === 'planned' && !record.controlRequestedBy && ['operator', 'admin'].includes(role);
  const canConfirm = kind === 'execution' && record?.status === 'planned' && Boolean(record.controlRequestedBy) && record?.controlRequestedBy !== username && ['reviewer', 'admin'].includes(role);

  return <Drawer title={kind === 'plan' ? '灌溉策略详情' : '计划与阀门控制'} placement="right" width="min(600px, 100vw)" open={open} onClose={onClose}>
    {record && <div className="plan-drawer">
      <div className="drawer-heading"><div><span>{record.code}</span><h2>{record.name}</h2></div><StatusBadge status={record.status} /></div>
      <Descriptions bordered size="small" column={1}>
        <Descriptions.Item label="策略版本">v{record.version}</Descriptions.Item>
        <Descriptions.Item label="目标分区">{record.zoneCode ? <Tag>{record.zoneCode}</Tag> : null}{record.facility}</Descriptions.Item>
        {kind === 'execution' && <Descriptions.Item label="灌溉计划">{record.planCode ? <Tag color="blue">{record.planCode}</Tag> : <Tag color="red">未绑定计划</Tag>}</Descriptions.Item>}
        {kind === 'plan' && <Descriptions.Item label="计划停灌线">{record.stopMoistureLine ? <Tag color="orange">{record.stopMoistureLine.toFixed(1)}%</Tag> : '-'}</Descriptions.Item>}
        <Descriptions.Item label="责任人">{record.owner}</Descriptions.Item>
        <Descriptions.Item label="灌溉指标">{record.metricValue} {record.metricUnit}</Descriptions.Item>
        <Descriptions.Item label="生效时间">{formatDate(record.effectiveAt)}</Descriptions.Item>
        <Descriptions.Item label="证据">{record.evidence || '未附加'}</Descriptions.Item>
      </Descriptions>

      {kind === 'execution' && <ControlCheckPanel snapshot={snapshot} live={snapshotLive} />}

      {kind === 'execution' && <section className="control-approval" aria-label="远程控制双确认">
        <h3>远程控制双确认</h3>
        <div className="approval-stage"><span>1</span><div><strong>操作员请求</strong><p>{record.controlRequestedBy ? `${record.controlRequestedBy} · ${formatDate(record.controlRequestedAt || '')}` : '等待操作员核对分区、计划版本与阀门连通性'}</p></div>{record.controlRequestedBy && <Tag color="gold">已请求</Tag>}</div>
        <div className="approval-stage"><span>2</span><div><strong>独立复核</strong><p>{record.controlConfirmedBy ? `${record.controlConfirmedBy} · ${formatDate(record.controlConfirmedAt || '')}` : '请求人之外的复核员，在启动前复核无冲突后确认启动'}</p></div>{record.controlConfirmedBy && <Tag color="green">已确认</Tag>}</div>
        {error && <Alert type="error" showIcon message={error} />}
        {canRequest && <div className="approval-action"><Checkbox checked={acknowledged} onChange={(event) => setAcknowledged(event.target.checked)}>我已核对分区、计划版本与阀门连通性</Checkbox><Button type="primary" icon={<SendOutlined />} disabled={!acknowledged} loading={busy} onClick={() => record && onRequest && void onRequest(record)}>提交远程启动请求</Button></div>}
        {canConfirm && blocked && <Alert type="error" showIcon message="启动前复核存在冲突，阀门不能启动；请先处理冲突（停止同分区任务、补测墒情或核对停灌线），状态保持待启动。" />}
        {canConfirm && <div className="approval-action">
          <Checkbox checked={acknowledged} onChange={(event) => setAcknowledged(event.target.checked)}>我已查看同分区运行中任务、最近一次已校验读数与计划停灌线，确认无冲突并批准远程启动</Checkbox>
          <Button type="primary" icon={<SafetyCertificateOutlined />} disabled={!acknowledged || blocked} loading={busy} onClick={() => record && onConfirm && void onConfirm(record)}>
            {blocked ? '复核未通过·保持待启动' : '复核并启动阀门'}
          </Button>
        </div>}
        {record.controlRequestedBy === username && !record.controlConfirmedBy && <Alert type="warning" showIcon message="请求人不能复核自己的远程启动，请切换独立复核账号。" />}
        {role === 'viewer' && !record.controlConfirmedBy && <Alert type="info" showIcon message="当前账号仅可查看远程控制确认链。" />}
        {record.status !== 'planned' && <Space><Tag color="green">控制链已闭环</Tag><span>当前执行状态为 {record.status}</span></Space>}
      </section>}
    </div>}
  </Drawer>;
}
