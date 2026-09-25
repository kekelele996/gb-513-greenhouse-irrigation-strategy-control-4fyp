import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react';
import type { EntityConfig, DomainRecord } from '../types/domain';
import type { EntityStore } from '../stores/factory';
import { nextStatus, formatDate } from '../utils/format';
import { useAuth } from '../hooks/useAuth';
import { usePolling } from '../hooks/usePolling';
import { StatusBadge } from './common/StatusBadge';
import { MetricCard } from './common/MetricCard';
import { ConfirmDialog } from './common/ConfirmDialog';
import { UiButton } from './common/UiButton';

interface EntityPageProps {
  config: EntityConfig;
  useStore: EntityStore;
  renderMetric?: (item: DomainRecord) => ReactNode;
  onInspect?: (item: DomainRecord) => void;
  inspectLabel?: string;
  transitionRoles?: string[];
  allowTransition?: (item: DomainRecord, next: string) => boolean;
  onCreateCustom?: () => Promise<void>;
}

export function EntityPage({ config, useStore, renderMetric, onInspect, inspectLabel = '查看详情', transitionRoles = ['operator', 'reviewer', 'admin'], allowTransition = () => true, onCreateCustom }: EntityPageProps) {
  const { session } = useAuth();
  const { items, meta, loading, error, load, createRecord, transition } = useStore();
  const [search, setSearch] = useState('');
  const [showCreate, setShowCreate] = useState(false);
  const [pending, setPending] = useState<{ item: DomainRecord; status: string } | null>(null);
  const role = session?.role || 'viewer';
  const canCreate = ['operator', 'admin'].includes(role);
  const canTransition = transitionRoles.includes(role);
  useEffect(() => { void load(config.path); }, [config.path, load]);
  const refresh = useCallback(() => load(config.path, search), [config.path, load, search]);
  usePolling(refresh, 30_000, !loading);
  const highRisk = useMemo(() => items.filter((item) => ['high', 'critical'].includes(item.riskLevel)).length, [items]);

  const createDemo = async () => {
    const now = Date.now();
    await createRecord(config.path, { code: `${config.key.toUpperCase()}-${now.toString().slice(-6)}`, name: `新增${config.label}`,
      description: '通过前端工作台创建的业务记录', facility: '默认作业区', owner: session?.username || '现场操作员', category: '常规', riskLevel: 'medium',
      metricValue: 42, metricUnit: config.path === 'zones' || config.path === 'readings' ? '%' : 'L/min', effectiveAt: new Date().toISOString(), evidence: '已完成创建前检查', relatedCode: '' });
    setShowCreate(false);
  };

  return <main className="workspace">
    <header className="page-header"><div><p className="eyebrow">业务工作台</p><h1>{config.label}</h1><p>统一管理{config.label}的状态、风险、证据与责任人。</p></div>{canCreate ? <UiButton onClick={() => setShowCreate(true)}>新增{config.label}</UiButton> : <span className="readonly-flag">{role === 'reviewer' ? '复核权限' : '只读权限'}</span>}</header>
    <section className="metrics"><MetricCard label="记录总数" value={meta.total} detail="当前筛选范围"/><MetricCard label="高风险" value={highRisk} detail="需要优先复核"/><MetricCard label="状态种类" value={new Set(items.map((item) => item.status)).size} detail="状态机覆盖"/></section>
    <section className="toolbar"><input aria-label="搜索" placeholder={`搜索${config.label}编码或名称`} value={search} onChange={(event) => setSearch(event.target.value)} /><UiButton onClick={() => void refresh()}>查询</UiButton><button className="link-button" onClick={() => { setSearch(''); void load(config.path); }}>重置</button></section>
    {error && <div className="alert" role="alert">{error}</div>}
    <section className="table-shell" aria-busy={loading}><table><thead><tr><th>编码</th><th>名称</th><th>状态</th><th>风险</th><th>责任人</th><th>指标</th><th>更新时间</th><th>操作</th></tr></thead><tbody>
      {items.map((item) => { const next = nextStatus(item.status, config.statuses); const mayAdvance = next && canTransition && allowTransition(item, next); return <tr key={item.id}><td><strong>{item.code}</strong></td><td>{item.name}<small>{item.facility}</small></td><td><StatusBadge status={item.status}/></td><td>{item.riskLevel}</td><td>{item.owner}</td><td>{renderMetric ? renderMetric(item) : <>{item.metricValue} {item.metricUnit}</>}</td><td>{formatDate(item.updatedAt)}</td><td><div className="table-actions">{onInspect && <button className="table-action" onClick={() => onInspect(item)}>{inspectLabel}</button>}{mayAdvance && <button className="table-action" onClick={() => setPending({ item, status: next })}>推进至 {next}</button>}{!onInspect && !mayAdvance && <span className="muted">{next ? '无操作权限' : '流程结束'}</span>}</div></td></tr>; })}
      {!items.length && !loading && <tr><td colSpan={8} className="empty">暂无记录</td></tr>}
    </tbody></table>{loading && <div className="loading">正在同步业务数据...</div>}</section>
    <ConfirmDialog open={showCreate} title={`新增${config.label}`} onCancel={() => setShowCreate(false)} onConfirm={() => { if (onCreateCustom) { void onCreateCustom().then(() => setShowCreate(false)).catch(() => undefined); } else { void createDemo().catch(() => undefined); } }}><p>{config.path === 'executions' ? '将按现有温室分区和灌溉计划自动建立关联，再创建一条待启动的阀门执行记录。' : '将创建一条包含完整责任人、风险和证据信息的演示记录。'}</p></ConfirmDialog>
    <ConfirmDialog open={Boolean(pending)} title="确认状态迁移" onCancel={() => setPending(null)} onConfirm={() => { if (pending) void transition(config.path, pending.item, pending.status).then(() => setPending(null)).catch(() => undefined); }}><p>状态迁移会写入审计日志，且使用版本号避免并发覆盖。</p><strong>{pending?.item.status} → {pending?.status}</strong></ConfirmDialog>
  </main>;
}
