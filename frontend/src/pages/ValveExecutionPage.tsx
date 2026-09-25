import { useState } from 'react';
import { Tag } from 'antd';
import { EntityPage } from '../components/EntityPage';
import { PlanDrawer } from '../components/PlanDrawer';
import { ENTITY_CONFIGS } from '../types/status';
import { useValveExecutionStore } from '../stores/valve-execution';
import { useAuth } from '../hooks/useAuth';
import { confirmValveControl, getValveControlDetail, requestValveControl } from '../api/valve-execution';
import type { ControlCheckSnapshot, DomainRecord } from '../types/domain';

export default function ValveExecutionPage() {
  const [selected, setSelected] = useState<DomainRecord | null>(null);
  const [snapshot, setSnapshot] = useState<ControlCheckSnapshot | null>(null);
  const [snapshotLive, setSnapshotLive] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const { session } = useAuth();

  const refreshDetail = async (id: number) => {
    try {
      const detail = await getValveControlDetail(id);
      setSelected(detail.data.execution);
      setSnapshot(detail.data.snapshot);
      setSnapshotLive(detail.data.live);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : String(reason));
    }
  };

  const inspect = (item: DomainRecord) => {
    setSelected(item);
    setError('');
    void refreshDetail(item.id);
  };

  const execute = async (record: DomainRecord, mode: 'request' | 'confirm') => {
    setBusy(true);
    setError('');
    try {
      const result = mode === 'request' ? await requestValveControl(record.id, record.version) : await confirmValveControl(record.id, record.version);
      await useValveExecutionStore.getState().load('executions');
      await refreshDetail(result.data.id);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : String(reason));
    } finally {
      setBusy(false);
    }
  };

  return <>
    <EntityPage config={ENTITY_CONFIGS[3]} useStore={useValveExecutionStore} onInspect={inspect} inspectLabel="控制详情" allowTransition={(item, next) => !(item.status === 'planned' && next === 'running')} renderMetric={(item) => <span>{item.metricValue} {item.metricUnit}{item.status === 'planned' && item.controlCheckStatus === 'blocked' && <Tag color="error" style={{ marginLeft: 6 }}>复核拦截</Tag>}{item.status === 'planned' && item.controlRequestedBy && item.controlCheckStatus !== 'blocked' && <Tag color="gold" style={{ marginLeft: 6 }}>待复核</Tag>}</span>} />
    <PlanDrawer open={Boolean(selected)} record={selected} kind="execution" username={session?.username || ''} role={session?.role || 'viewer'} busy={busy} error={error} snapshot={snapshot} snapshotLive={snapshotLive} onClose={() => setSelected(null)} onRequest={(record) => execute(record, 'request')} onConfirm={(record) => execute(record, 'confirm')} />
  </>;
}
