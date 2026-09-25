import type { DomainRecord, EntityConfig, UserSession } from './domain';

export type ZoneState = 'active' | 'dry' | 'wet' | 'locked';
export const ALL_ZONE_STATE: readonly ZoneState[] = ['active', 'dry', 'wet', 'locked'];
export type ExecutionState = 'planned' | 'running' | 'succeeded' | 'failed';
export const ALL_EXECUTION_STATE: readonly ExecutionState[] = ['planned', 'running', 'succeeded', 'failed'];

// Demo records bind to the seeded zone/plan so the pre-start control checks
// have real evidence to evaluate.
function demoFields(path: string, session: UserSession | null): Partial<DomainRecord> {
  const base = { owner: session?.username || 'operator' };
  if (path === 'readings') {
    return { ...base, zoneCode: 'GZ-001', metricUnit: '%' };
  }
  if (path === 'plans') {
    return { ...base, zoneCode: 'GZ-001', stopMoistureLine: 35, metricUnit: 'L/min' };
  }
  if (path === 'executions') {
    return { ...base, zoneCode: 'GZ-001', planCode: 'IP-001', metricUnit: 'L/min' };
  }
  return { ...base, metricUnit: '%' };
}

export const ENTITY_CONFIGS: readonly EntityConfig[] = [
  { key: 'greenhouseZone', path: 'zones', label: '温室分区', statuses: ['active', 'dry', 'wet', 'locked'] as const, resolveDemoFields: (session) => demoFields('zones', session) },
  { key: 'soilReading', path: 'readings', label: '土壤读数', statuses: ['fresh', 'validated', 'anomalous', 'expired'] as const, resolveDemoFields: (session) => demoFields('readings', session) },
  { key: 'irrigationPlan', path: 'plans', label: '灌溉计划', statuses: ['draft', 'approved', 'scheduled', 'completed'] as const, resolveDemoFields: (session) => demoFields('plans', session) },
  { key: 'valveExecution', path: 'executions', label: '阀门执行', statuses: ['planned', 'running', 'succeeded', 'failed'] as const, resolveDemoFields: (session) => demoFields('executions', session) }
];
