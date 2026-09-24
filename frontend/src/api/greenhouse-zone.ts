
import { request } from './client';
import type { DomainRecord } from '../types/domain';

export async function listGreenhouseZone(page = 1, pageSize = 20, search = '') {
  return request<DomainRecord[]>(`/zones?page=${page}&pageSize=${pageSize}&search=${encodeURIComponent(search)}`);
}
export async function createGreenhouseZone(input: Partial<DomainRecord>) {
  return request<DomainRecord>('/zones', { method: 'POST', body: JSON.stringify(input) });
}
export async function transitionGreenhouseZone(id: number, status: string, expectedVersion: number, reason: string) {
  return request<DomainRecord>(`/zones/${id}/transition`, {
    method: 'POST', body: JSON.stringify({ status, expectedVersion, reason }),
  });
}
