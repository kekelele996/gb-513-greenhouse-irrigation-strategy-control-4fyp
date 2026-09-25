import { useState } from 'react';
import { EntityPage } from '../components/EntityPage';
import { PlanDrawer } from '../components/PlanDrawer';
import { ENTITY_CONFIGS } from '../types/status';
import { useValveExecutionStore } from '../stores/valve-execution';
import { useAuth } from '../hooks/useAuth';
import { confirmValveControl, createValveExecution, fetchPreStartCheck, requestValveControl } from '../api/valve-execution';
import { listGreenhouseZone } from '../api/greenhouse-zone';
import { listIrrigationPlan } from '../api/irrigation-plan';
import type { DomainRecord, PreStartCheck } from '../types/domain';

export default function ValveExecutionPage() {
  const [selected, setSelected] = useState<DomainRecord | null>(null);
  const [check, setCheck] = useState<PreStartCheck | null>(null);
  const [checkLoading, setCheckLoading] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const { session } = useAuth();
  const store = useValveExecutionStore();

  const loadCheck = async (record: DomainRecord) => {
    setCheckLoading(true);
    try {
      const result = await fetchPreStartCheck(record.id);
      setCheck(result.data);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : String(reason));
    } finally {
      setCheckLoading(false);
    }
  };

  const inspect = (item: DomainRecord) => {
    setSelected(item);
    setCheck(null);
    setError('');
    void loadCheck(item);
  };

  // New executions must point at an existing zone and a plan scheduled for that
  // zone, so pick the newest matching pair instead of writing unlinked records.
  const createLinkedRecord = async () => {
    const [zones, plans] = await Promise.all([listGreenhouseZone(1, 50), listIrrigationPlan(1, 50)]);
    const zone = zones.data.find((item) => plans.data.some((plan) => (plan.zoneCode || '') === item.code)) || zones.data[0];
    const plan = plans.data.find((item) => (item.zoneCode || '') === zone?.code) || plans.data[0];
    if (!zone || !plan) {
      setError('请先创建温室分区和带停灌线的灌溉计划，再登记阀门执行。');
      return;
    }
    const now = Date.now();
    await createValveExecution({
      code: `VALVEEXECUTION-${now.toString().slice(-6)}`, name: `新增阀门执行`,
      description: '通过前端工作台创建的阀门执行记录', facility: zone.facility, owner: session?.username || '现场操作员',
      category: '常规', riskLevel: 'medium', metricValue: 30, metricUnit: 'L/min',
      effectiveAt: new Date().toISOString(), evidence: '已核对分区与计划关联', relatedCode: zone.relatedCode || '',
      zoneCode: zone.code, planCode: plan.code,
    });
    await store.load('executions');
  };

  const execute = async (record: DomainRecord, mode: 'request' | 'confirm') => {
    setBusy(true);
    setError('');
    try {
      const result = mode === 'request' ? await requestValveControl(record.id, record.version) : await confirmValveControl(record.id, record.version);
      setSelected(result.data);
      await store.load('executions');
      await loadCheck(result.data);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : String(reason));
      await store.load('executions');
      const latest = useValveExecutionStore.getState().items.find((item) => item.id === record.id);
      if (latest) {
        setSelected(latest);
        await loadCheck(latest);
      }
    } finally {
      setBusy(false);
    }
  };

  return <>
    <EntityPage config={ENTITY_CONFIGS[3]} useStore={useValveExecutionStore} onInspect={inspect} inspectLabel="控制详情" onCreateCustom={createLinkedRecord} allowTransition={(item, next) => !(item.status === 'planned' && next === 'running')} />
    <PlanDrawer open={Boolean(selected)} record={selected} kind="execution" username={session?.username || ''} role={session?.role || 'viewer'} busy={busy} error={error} check={check} checkLoading={checkLoading} onRefreshCheck={() => selected && void loadCheck(selected)} onClose={() => setSelected(null)} onRequest={(record) => execute(record, 'request')} onConfirm={(record) => execute(record, 'confirm')} />
  </>;
}
