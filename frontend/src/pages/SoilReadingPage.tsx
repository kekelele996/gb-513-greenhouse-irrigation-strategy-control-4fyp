import { Tag } from 'antd';
import { EntityPage } from '../components/EntityPage';
import { ENTITY_CONFIGS } from '../types/status';
import { useSoilReadingStore } from '../stores/soil-reading';
import { MoistureBadge } from '../components/common/MoistureBadge';

export default function SoilReadingPage() {
  return <EntityPage
    config={ENTITY_CONFIGS[1]}
    useStore={useSoilReadingStore}
    renderMetric={(item) => <span><MoistureBadge value={item.metricValue} status={item.status} />{item.zoneCode && <Tag style={{ marginLeft: 6 }}>{item.zoneCode}</Tag>}</span>}
  />;
}
