import { Alert, Button, Checkbox, Descriptions, Drawer, List, Space, Tag, Typography } from 'antd';
import { SafetyCertificateOutlined, SendOutlined, ReloadOutlined } from '@ant-design/icons';
import { useEffect, useState } from 'react';
import type { DomainRecord, PreStartCheck } from '../types/domain';
import { formatDate } from '../utils/format';
import { StatusBadge } from './common/StatusBadge';

interface PlanDrawerProps {
  open: boolean;
  record: DomainRecord | null;
  kind: 'plan' | 'execution';
  username: string;
  role: string;
  busy?: boolean;
  error?: string;
  check?: PreStartCheck | null;
  checkLoading?: boolean;
  onRefreshCheck?: () => void;
  onClose: () => void;
  onRequest?: (record: DomainRecord) => Promise<void>;
  onConfirm?: (record: DomainRecord) => Promise<void>;
}

export function PlanDrawer({ open, record, kind, username, role, busy = false, error = '', check, checkLoading = false, onRefreshCheck, onClose, onRequest, onConfirm }: PlanDrawerProps) {
  const [acknowledged, setAcknowledged] = useState(false);
  useEffect(() => setAcknowledged(false), [open, record?.id, record?.version, kind]);
  const canRequest = kind === 'execution' && record?.status === 'planned' && !record.controlRequestedBy && ['operator', 'admin'].includes(role);
  const canConfirm = kind === 'execution' && record?.status === 'planned' && Boolean(record.controlRequestedBy) && record?.controlRequestedBy !== username && ['reviewer', 'admin'].includes(role);
  const hasConflicts = Boolean(check && check.conflicts.length > 0);

  return <Drawer title={kind === 'plan' ? '灌溉策略详情' : '计划与阀门控制'} placement="right" width="min(560px, 100vw)" open={open} onClose={onClose}>
    {record && <div className="plan-drawer">
      <div className="drawer-heading"><div><span>{record.code}</span><h2>{record.name}</h2></div><StatusBadge status={record.status} /></div>
      <Descriptions bordered size="small" column={1}>
        <Descriptions.Item label="策略版本">v{record.version}</Descriptions.Item>
        {kind === 'execution' && <Descriptions.Item label="关联分区">{record.zoneCode || '未关联'} <small>{record.facility}</small></Descriptions.Item>}
        {kind === 'execution' && <Descriptions.Item label="灌溉计划">{record.planCode || '未关联'}{check?.plan ? ` · ${check.plan.name}（${check.plan.status}）` : ''}</Descriptions.Item>}
        {kind === 'plan' && <Descriptions.Item label="适用分区">{record.zoneCode || '未设置'}</Descriptions.Item>}
        {kind === 'plan' && <Descriptions.Item label="计划停灌线">{typeof record.stopMoisture === 'number' && record.stopMoisture > 0 ? `${record.stopMoisture}%` : '未设置'}</Descriptions.Item>}
        <Descriptions.Item label="目标分区">{record.facility}</Descriptions.Item>
        <Descriptions.Item label="责任人">{record.owner}</Descriptions.Item>
        <Descriptions.Item label="灌溉指标">{record.metricValue} {record.metricUnit}</Descriptions.Item>
        <Descriptions.Item label="生效时间">{formatDate(record.effectiveAt)}</Descriptions.Item>
        <Descriptions.Item label="证据">{record.evidence || '未附加'}</Descriptions.Item>
      </Descriptions>

      {kind === 'execution' && <section className="control-approval" aria-label="远程控制双确认">
        <div className="approval-heading"><h3>启动前复核</h3>{onRefreshCheck && <Button size="small" icon={<ReloadOutlined />} loading={checkLoading} onClick={onRefreshCheck}>刷新复核视图</Button>}</div>
        {check && <div className="prestart-check">
          <Descriptions bordered size="small" column={1}>
            <Descriptions.Item label="同分区运行中任务">
              {check.runningTasks.length === 0 ? <Tag color="green">无运行中任务</Tag> :
                <List size="small" dataSource={check.runningTasks} renderItem={(task) => <List.Item><Tag color="red">运行中</Tag><span>{task.code} {task.name} · 请求人 {task.startedRequest || '未知'} · 更新 {formatDate(task.updatedAt)}</span></List.Item>} />}
            </Descriptions.Item>
            <Descriptions.Item label="最近已校验读数">
              {check.reading ? <Space direction="vertical" size={2}>
                <span><strong>{check.reading.code}</strong> · 含水率 <Typography.Text strong type={check.readingFresh ? 'success' : 'danger'}>{check.reading.moisture}%</Typography.Text></span>
                <span>读数时间：{formatDate(check.reading.measuredAt)}（{check.reading.ageMinutes} 分钟前）{check.readingFresh ? <Tag color="green">30 分钟内</Tag> : <Tag color="red">已超 30 分钟</Tag>}</span>
              </Space> : <Tag color="red">无已校验读数</Tag>}
            </Descriptions.Item>
            <Descriptions.Item label="计划停灌线">
              {check.plan ? <span>计划 {check.plan.code} 停灌线 <Typography.Text strong>{check.plan.stopMoisture}%</Typography.Text>（{check.plan.status}）</span> : <Tag color="red">计划缺失，无停灌线</Tag>}
            </Descriptions.Item>
          </Descriptions>

          {hasConflicts && <Alert type="error" showIcon message={`复核未通过（${check!.conflicts.length} 项冲突），状态保持待启动`}
            description={<List size="small" dataSource={check!.conflicts} renderItem={(conflict, index) => <List.Item>
              <Space direction="vertical" size={0}>
                <span><Tag color="red">{conflict.code}</Tag><Typography.Text code>{record.controlConflictNo}{String.fromCharCode(65 + index)}</Typography.Text></span>
                {conflict.readingTime && <small>读数时间：{formatDate(conflict.readingTime)}{typeof conflict.moisture === 'number' ? ` · 含水率 ${conflict.moisture}%` : ''}</small>}
                <span>{conflict.message}</span>
              </Space>
            </List.Item>} />} />}
          {check.conflicts.length === 0 && check.readingFresh && <Alert type="success" showIcon message="检查通过：无运行中任务、读数在有效期内且低于停灌线，可进入双人启动" />}
          {record.controlDetail && <div className="control-detail"><strong>控制详情：</strong><pre>{record.controlDetail}</pre></div>}
          {record.controlCheckSnapshot && <div className="control-snapshot"><strong>启动检查快照：</strong><pre>{record.controlCheckSnapshot}</pre></div>}
        </div>}
        {!check && !checkLoading && <Alert type="info" showIcon message="点击“刷新复核视图”查看同分区运行中任务、最近已校验读数和计划停灌线。" />}

        <h3>远程控制双确认</h3>
        <div className="approval-stage"><span>1</span><div><strong>操作员请求</strong><p>{record.controlRequestedBy ? `${record.controlRequestedBy} · ${formatDate(record.controlRequestedAt || '')}` : '等待操作员核对策略和阀门连通性'}</p></div>{record.controlRequestedBy && <Tag color="gold">已请求</Tag>}</div>
        <div className="approval-stage"><span>2</span><div><strong>独立复核</strong><p>{record.controlConfirmedBy ? `${record.controlConfirmedBy} · ${formatDate(record.controlConfirmedAt || '')}` : '请求人之外的复核员确认后才会启动'}</p></div>{record.controlConfirmedBy && <Tag color="green">已确认</Tag>}</div>
        {record.controlCheckResult === 'blocked' && <Alert type="warning" showIcon message={`上次复核被阻断：${record.controlConflictNo}`} description={record.controlDetail} />}
        {error && <Alert type="error" showIcon message={error} />}
        {canRequest && <div className="approval-action"><Checkbox checked={acknowledged} onChange={(event) => setAcknowledged(event.target.checked)}>我已核对分区、计划版本与阀门连通性</Checkbox><Button type="primary" icon={<SendOutlined />} disabled={!acknowledged} loading={busy} onClick={() => record && onRequest && void onRequest(record)}>提交远程启动请求</Button></div>}
        {canConfirm && <div className="approval-action"><Checkbox checked={acknowledged} onChange={(event) => setAcknowledged(event.target.checked)}>我已查看运行中任务、最近已校验读数与停灌线，确认无冲突并批准启动</Checkbox><Button type="primary" danger={hasConflicts} icon={<SafetyCertificateOutlined />} disabled={!acknowledged || hasConflicts} loading={busy} onClick={() => record && onConfirm && void onConfirm(record)}>{hasConflicts ? '存在冲突不可启动' : '复核并启动阀门'}</Button></div>}
        {record.controlRequestedBy === username && !record.controlConfirmedBy && <Alert type="warning" showIcon message="请求人不能复核自己的远程启动，请切换独立复核账号。" />}
        {role === 'viewer' && !record.controlConfirmedBy && <Alert type="info" showIcon message="当前账号仅可查看远程控制确认链。" />}
        {record.status !== 'planned' && <Space><Tag color="green">控制链已闭环</Tag><span>当前执行状态为 {record.status}</span></Space>}
      </section>}
    </div>}
  </Drawer>;
}
