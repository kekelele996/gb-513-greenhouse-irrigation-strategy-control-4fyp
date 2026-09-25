
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
  stopMoistureLine?: number;
  planCode?: string;
  controlRequestedBy?: string;
  controlRequestedAt?: string | null;
  controlConfirmedBy?: string;
  controlConfirmedAt?: string | null;
  controlCheckStatus?: 'pending' | 'passed' | 'blocked' | string;
  controlConflictNo?: string;
  controlDetail?: string;
  controlCheckSnapshot?: string;
  createdAt: string;
  updatedAt: string;
}

export interface ControlConflict {
  code: string;
  kind: string;
  message: string;
  readingCode?: string;
  readingTime?: string;
  moisture?: number;
  executionId?: number;
  zoneCode?: string;
  planCode?: string;
  stopLine?: number;
}

export interface ControlRunningTask {
  id: number;
  code: string;
  name: string;
  status: string;
  startedAt?: string;
  controlConfirmedBy?: string;
}

export interface ControlReadingEvidence {
  id: number;
  code: string;
  moisture: number;
  measuredAt: string;
  status: string;
  ageMinutes: number;
}

export interface ControlCheckSnapshot {
  checkedAt: string;
  checkedBy: string;
  status: 'pending' | 'passed' | 'blocked' | string;
  zoneCode: string;
  zoneName?: string;
  planCode: string;
  planName?: string;
  planStatus?: string;
  stopMoistureLine?: number;
  runningTasks: ControlRunningTask[];
  latestReading?: ControlReadingEvidence;
  freshnessWindowMinutes: number;
  conflicts: ControlConflict[];
}

export interface ControlDetail {
  execution: DomainRecord;
  snapshot: ControlCheckSnapshot | null;
  live: boolean;
}

export interface PageMeta { page: number; pageSize: number; total: number }
export interface ApiEnvelope<T> { data: T; error?: string; message?: string; meta?: PageMeta }
export interface UserSession { token: string; username: string; displayName: string; role: string; expiresIn: number }
export interface AuditLog {
  id: number; requestId: string; actor: string; action: string; entityType: string;
  entityId: number; beforeState: string; afterState: string; detail: string; createdAt: string;
}
export interface EntityConfig {
  key: string;
  path: string;
  label: string;
  statuses: readonly string[];
  /** Optional hook so a page can supply the extra demo fields its API requires. */
  resolveDemoFields?: (session: UserSession | null) => Partial<DomainRecord>;
}
