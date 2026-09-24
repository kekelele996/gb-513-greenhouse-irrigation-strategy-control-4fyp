
import { request } from './client';
import type { DomainRecord } from '../types/domain';

export async function listSoilReading(page = 1, pageSize = 20, search = '') {
  return request<DomainRecord[]>(`/readings?page=${page}&pageSize=${pageSize}&search=${encodeURIComponent(search)}`);
}
export async function createSoilReading(input: Partial<DomainRecord>) {
  return request<DomainRecord>('/readings', { method: 'POST', body: JSON.stringify(input) });
}
export async function transitionSoilReading(id: number, status: string, expectedVersion: number, reason: string) {
  return request<DomainRecord>(`/readings/${id}/transition`, {
    method: 'POST', body: JSON.stringify({ status, expectedVersion, reason }),
  });
}
