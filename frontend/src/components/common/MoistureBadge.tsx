export function MoistureBadge({ value, status }: { value: number; status: string }) {
  const tone = status === 'dry' || value < 25 ? 'dry' : status === 'wet' || value > 70 ? 'wet' : 'balanced';
  const label = tone === 'dry' ? '偏干' : tone === 'wet' ? '偏湿' : '适宜';
  return <span className={`moisture moisture--${tone}`}><strong>{value.toFixed(1)}%</strong><small>{label}</small></span>;
}
