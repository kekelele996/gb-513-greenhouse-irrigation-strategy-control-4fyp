import { useState } from 'react';
import { EntityPage } from '../components/EntityPage';
import { PlanDrawer } from '../components/PlanDrawer';
import { ENTITY_CONFIGS } from '../types/status';
import { useValveExecutionStore } from '../stores/valve-execution';
import { useAuth } from '../hooks/useAuth';
import { confirmValveControl, requestValveControl } from '../api/valve-execution';
import type { DomainRecord } from '../types/domain';

export default function ValveExecutionPage() {
  const [selected, setSelected] = useState<DomainRecord | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const { session } = useAuth();

  const execute = async (record: DomainRecord, mode: 'request' | 'confirm') => {
    setBusy(true);
    setError('');
    try {
      const result = mode === 'request' ? await requestValveControl(record.id, record.version) : await confirmValveControl(record.id, record.version);
      setSelected(result.data);
      await useValveExecutionStore.getState().load('executions');
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : String(reason));
    } finally {
      setBusy(false);
    }
  };

  return <>
    <EntityPage config={ENTITY_CONFIGS[3]} useStore={useValveExecutionStore} onInspect={(item) => { setSelected(item); setError(''); }} inspectLabel="控制详情" allowTransition={(item, next) => !(item.status === 'planned' && next === 'running')} />
    <PlanDrawer open={Boolean(selected)} record={selected} kind="execution" username={session?.username || ''} role={session?.role || 'viewer'} busy={busy} error={error} onClose={() => setSelected(null)} onRequest={(record) => execute(record, 'request')} onConfirm={(record) => execute(record, 'confirm')} />
  </>;
}
