import { Navigate, createBrowserRouter } from 'react-router-dom';
import App from '../App';
import GreenhouseZonePage from '../pages/GreenhouseZonePage';
import SoilReadingPage from '../pages/SoilReadingPage';
import IrrigationPlanPage from '../pages/IrrigationPlanPage';
import ValveExecutionPage from '../pages/ValveExecutionPage';
import AuditPage from '../pages/AuditPage';
import { useAuth } from '../hooks/useAuth';

function RequireAuth({ children }: { children: React.ReactNode }) {
  const { loading, authenticated, switchAccount } = useAuth();
  if (loading) return <div className="app-loading">正在建立安全会话...</div>;
  if (!authenticated) return <main className="auth-gate"><h1>需要安全会话</h1><p>登录后才能访问灌溉控制工作台。</p><button onClick={() => void switchAccount('admin')}>使用演示管理员登录</button></main>;
  return <>{children}</>;
}

export const router = createBrowserRouter([{ path: '/', element: <RequireAuth><App /></RequireAuth>, children: [
  { index: true, element: <Navigate to="/zones" replace /> },
  { path: 'zones', element: <GreenhouseZonePage /> }, { path: 'readings', element: <SoilReadingPage /> }, { path: 'plans', element: <IrrigationPlanPage /> }, { path: 'executions', element: <ValveExecutionPage /> },
  { path: 'audit', element: <AuditPage /> },
] }], { future: { v7_fetcherPersist: true, v7_normalizeFormMethod: true, v7_partialHydration: true, v7_relativeSplatPath: true, v7_skipActionErrorRevalidation: true } });
