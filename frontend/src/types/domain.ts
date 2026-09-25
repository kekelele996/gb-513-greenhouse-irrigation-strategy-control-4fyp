
export interface DomainRecord {
  id: number;
  code: string;
  name: string;
  status: string;
  version: number;
  description: string;
  facility: string;
  owner: string;
  category: string;
  riskLevel: 'low' | 'medium' | 'high' | 'critical';
  metricValue: number;
  metricUnit: string;
  effectiveAt: string;
  evidence: string;
  relatedCode: string;
  zoneCode?: string;
  planCode?: string;
  stopMoisture?: number;
  controlRequestedBy?: string;
  controlRequestedAt?: string | null;
  controlConfirmedBy?: string;
  controlConfirmedAt?: string | null;
  controlCheckResult?: '' | 'passed' | 'blocked';
  controlCheckBy?: string;
  controlCheckAt?: string | null;
  controlConflictNo?: string;
  controlDetail?: string;
  controlCheckSnapshot?: string;
  createdAt: string;
  updatedAt: string;
}

export interface PreStartConflict {
  code: string;
  kind: string;
  message: string;
  readingTime?: string | null;
  moisture?: number | null;
}

export interface PreStartRunningTask {
  id: number;
  code: string;
  name: string;
  zoneCode: string;
  status: string;
  startedRequest: string;
  updatedAt: string;
}

export interface PreStartCheck {
  executionId: number;
  executionCode: string;
  zoneCode: string;
  planCode: string;
  status: string;
  result: '' | 'passed' | 'blocked';
  conflictNo: string;
  detail: string;
  checkedBy: string;
  checkedAt: string | null;
  zone: { code: string; name: string; status: string } | null;
  plan: { code: string; name: string; status: string; zoneCode: string; stopMoisture: number } | null;
  reading: { code: string; status: string; moisture: number; measuredAt: string; zoneCode: string; ageMinutes: number } | null;
  readingFresh: boolean;
  runningTasks: PreStartRunningTask[];
  conflicts: PreStartConflict[];
  snapshot: string;
}

export interface PageMeta { page: number; pageSize: number; total: number }
export interface ApiEnvelope<T> { data: T; error?: string; message?: string; meta?: PageMeta }
export interface UserSession { token: string; username: string; displayName: string; role: string; expiresIn: number }
export interface AuditLog {
  id: number; requestId: string; actor: string; action: string; entityType: string;
  entityId: number; beforeState: string; afterState: string; detail: string; createdAt: string;
}
export interface EntityConfig { key: string; path: string; label: string; statuses: readonly string[] }
