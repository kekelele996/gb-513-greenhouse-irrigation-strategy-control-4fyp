
import { request } from './client';
import type { DomainRecord, PreStartCheck } from '../types/domain';

export async function listValveExecution(page = 1, pageSize = 20, search = '') {
  return request<DomainRecord[]>(`/executions?page=${page}&pageSize=${pageSize}&search=${encodeURIComponent(search)}`);
}
export async function createValveExecution(input: Partial<DomainRecord>) {
  return request<DomainRecord>('/executions', { method: 'POST', body: JSON.stringify(input) });
}
export async function transitionValveExecution(id: number, status: string, expectedVersion: number, reason: string) {
  return request<DomainRecord>(`/executions/${id}/transition`, {
    method: 'POST', body: JSON.stringify({ status, expectedVersion, reason }),
  });
}
export async function requestValveControl(id: number, expectedVersion: number) {
  return request<DomainRecord>(`/executions/${id}/control-request`, {
    method: 'POST', body: JSON.stringify({ expectedVersion, confirmed: true, reason: '已核对分区、计划版本与阀门连通性' }),
  });
}
export async function confirmValveControl(id: number, expectedVersion: number) {
  return request<DomainRecord>(`/executions/${id}/control-confirm`, {
    method: 'POST', body: JSON.stringify({ expectedVersion, confirmed: true, reason: '独立复核通过，批准远程启动' }),
  });
}
export async function fetchPreStartCheck(id: number) {
  return request<PreStartCheck>(`/executions/${id}/control-check`);
}
