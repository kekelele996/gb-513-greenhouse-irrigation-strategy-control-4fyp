
import { NavLink, Outlet } from 'react-router-dom';
import { useAuth } from './hooks/useAuth';
const navigation = [{ to: '/zones', label: '温室分区' }, { to: '/readings', label: '土壤读数' }, { to: '/plans', label: '灌溉计划' }, { to: '/executions', label: '阀门执行' }, { to: '/audit', label: '审计记录' }];
export default function App() {
  const { session, loading, logout, switchAccount } = useAuth();
  if (loading) return <div className="app-loading">正在建立安全会话…</div>;
  return <div className="app-shell"><aside><div className="brand"><span>CONTROL DESK</span><strong>温室灌溉策略执行控制</strong></div><nav>{navigation.map((item) => <NavLink key={item.to} to={item.to}>{item.label}</NavLink>)}</nav><div className="user-panel"><span>{session?.displayName || '系统管理员'}</span><small>{session?.role || 'admin'}</small><label htmlFor="demo-account">演示账号</label><select id="demo-account" aria-label="切换演示账号" value={session?.username || 'admin'} onChange={(event) => void switchAccount(event.target.value)}><option value="admin">admin / 管理员</option><option value="operator">operator / 操作员</option><option value="reviewer">reviewer / 复核员</option><option value="viewer">viewer / 只读</option></select><button onClick={() => void logout()}>恢复管理员会话</button></div></aside><section className="content"><header className="topbar"><span>运行态势 · {session?.role}</span><span className="live-dot">服务已连接</span></header><Outlet /></section></div>;
}
