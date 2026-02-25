type UsageCostValueProps = {
  value?: number | null;
  precision?: number;
  naTooltip?: string;
};

export function UsageCostValue({
  value,
  precision = 6,
  naTooltip = "нет активного тарифа модели на момент запроса",
}: UsageCostValueProps) {
  if (value == null) {
    return <span title={naTooltip}>n/a</span>;
  }
  return <span>{value.toFixed(precision)}</span>;
}

