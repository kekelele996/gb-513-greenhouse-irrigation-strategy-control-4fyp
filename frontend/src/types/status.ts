import type { EntityConfig } from './domain';

export type ZoneState = 'active' | 'dry' | 'wet' | 'locked';
export const ALL_ZONE_STATE: readonly ZoneState[] = ['active', 'dry', 'wet', 'locked'];
export type ExecutionState = 'planned' | 'running' | 'succeeded' | 'failed';
export const ALL_EXECUTION_STATE: readonly ExecutionState[] = ['planned', 'running', 'succeeded', 'failed'];

export const ENTITY_CONFIGS: readonly EntityConfig[] = [
  { key: 'greenhouseZone', path: 'zones', label: '温室分区', statuses: ['active', 'dry', 'wet', 'locked'] as const },
  { key: 'soilReading', path: 'readings', label: '土壤读数', statuses: ['fresh', 'validated', 'anomalous', 'expired'] as const },
  { key: 'irrigationPlan', path: 'plans', label: '灌溉计划', statuses: ['draft', 'approved', 'scheduled', 'completed'] as const },
  { key: 'valveExecution', path: 'executions', label: '阀门执行', statuses: ['planned', 'running', 'succeeded', 'failed'] as const }
];
