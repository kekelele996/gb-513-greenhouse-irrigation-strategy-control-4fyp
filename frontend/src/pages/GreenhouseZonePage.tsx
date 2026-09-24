
import { EntityPage } from '../components/EntityPage';
import { ENTITY_CONFIGS } from '../types/status';
import { useGreenhouseZoneStore } from '../stores/greenhouse-zone';
import { MoistureBadge } from '../components/common/MoistureBadge';
export default function GreenhouseZonePage() { return <EntityPage config={ENTITY_CONFIGS[0]} useStore={useGreenhouseZoneStore} renderMetric={(item) => <MoistureBadge value={item.metricValue} status={item.status} />} />; }
