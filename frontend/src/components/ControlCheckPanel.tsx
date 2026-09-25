import { Alert, Descriptions, Empty, Space, Table, Tag, Typography } from 'antd';
import { ClockCircleOutlined, StopOutlined, ThunderboltOutlined } from '@ant-design/icons';
import type { ControlCheckSnapshot } from '../types/domain';
import { formatDate } from '../utils/format';

const { Text } = Typography;

interface ControlCheckPanelProps {
  snapshot: ControlCheckSnapshot | null;
  live: boolean;
}

const conflictTone: Record<string, string> = {
  running_task_same_zone: 'volcano',
  validated_reading_missing: 'volcano',
  validated_reading_stale: 'volcano',
  above_stop_moisture_line: 'volcano',
  zone_missing: 'volcano',
  plan_missing: 'volcano',
  plan_zone_mismatch: 'volcano',
};

export function ControlCheckPanel({ snapshot, live }: ControlCheckPanelProps) {
  if (!snapshot) {
    return <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无启动前复核数据" />;
  }
  const blocked = snapshot.status === 'blocked';
  return <section className="control-check" aria-label="启动前复核">
    <div className="control-check__head">
      <h3>启动前复核</h3>
      <Space size={6}>
        <Tag color={blocked ? 'error' : snapshot.status === 'passed' ? 'success' : 'default'}>
          {blocked ? '复核未通过·待启动' : snapshot.status === 'passed' ? '复核通过' : '待复核'}
        </Tag>
        {live && <Tag color="processing">实时预览</Tag>}
      </Space>
    </div>
    <Descriptions bordered size="small" column={1}>
      <Descriptions.Item label="复核时间">{formatDate(snapshot.checkedAt)}{snapshot.checkedBy ? ` · ${snapshot.checkedBy}` : ''}</Descriptions.Item>
      <Descriptions.Item label="绑定分区">{snapshot.zoneCode}{snapshot.zoneName ? ` · ${snapshot.zoneName}` : ''}</Descriptions.Item>
      <Descriptions.Item label="灌溉计划">{snapshot.planCode}{snapshot.planName ? ` · ${snapshot.planName}` : ''}{snapshot.planStatus ? `（${snapshot.planStatus}）` : ''}</Descriptions.Item>
      <Descriptions.Item label={<><StopOutlined /> 计划停灌线</>}>
        {snapshot.stopMoistureLine ? <Text strong>{snapshot.stopMoistureLine.toFixed(1)}%</Text> : <Text type="warning">计划缺失，无法读取停灌线</Text>}
      </Descriptions.Item>
      <Descriptions.Item label={<><ThunderboltOutlined /> 同分区运行中任务</>}>
        {snapshot.runningTasks.length === 0 ? <Tag color="green">无</Tag> :
          <Space direction="vertical" size={4}>
            {snapshot.runningTasks.map((task) => <Tag key={task.id} color="volcano">{task.code} · {task.name}（{task.status}{task.startedAt ? ` · ${formatDate(task.startedAt)}` : ''}）</Tag>)}
          </Space>}
      </Descriptions.Item>
      <Descriptions.Item label={<><ClockCircleOutlined /> 最近一次已校验读数</>}>
        {snapshot.latestReading ? <Space direction="vertical" size={2}>
          <Space><Text strong>{snapshot.latestReading.code}</Text><Tag color={snapshot.latestReading.ageMinutes > snapshot.freshnessWindowMinutes ? 'volcano' : 'green'}>{snapshot.latestReading.ageMinutes} 分钟前</Tag></Space>
          <Text>含水率 <Text strong>{snapshot.latestReading.moisture.toFixed(1)}%</Text></Text>
          <Text type="secondary">读数时间 {formatDate(snapshot.latestReading.measuredAt)} · 状态 {snapshot.latestReading.status}</Text>
        </Space> : <Tag color="volcano">分区内无已校验读数</Tag>}
        <div><Text type="secondary">读数有效窗口 {snapshot.freshnessWindowMinutes} 分钟，超时不得用于启动</Text></div>
      </Descriptions.Item>
    </Descriptions>

    {blocked && <Alert type="error" showIcon message={`复核发现 ${snapshot.conflicts.length} 项冲突，状态保持待启动，请处理后重新复核`} />}

    {snapshot.conflicts.length > 0 && <Table<ControlCheckSnapshot['conflicts'][number]>
      size="small"
      rowKey="code"
      pagination={false}
      dataSource={snapshot.conflicts}
      columns={[
        { title: '冲突编号', dataIndex: 'code', width: 190, render: (code: string) => <Text copyable className="conflict-no">{code}</Text> },
        {
          title: '冲突说明', dataIndex: 'message',
          render: (message: string, record) => <Space direction="vertical" size={2}>
            <Text>{message}</Text>
            {record.readingTime && <Text type="secondary">读数时间 {formatDate(record.readingTime)} · 含水率 {(record.moisture ?? 0).toFixed(1)}%{typeof record.stopLine === 'number' ? ` · 停灌线 ${record.stopLine.toFixed(1)}%` : ''}</Text>}
          </Space>,
        },
        { title: '类型', dataIndex: 'kind', width: 170, render: (kind: string) => <Tag color={conflictTone[kind] || 'default'}>{kind}</Tag> },
      ]}
    />}
  </section>;
}
