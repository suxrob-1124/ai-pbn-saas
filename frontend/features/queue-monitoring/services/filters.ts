type DateRange = {
  from?: string;
  to?: string;
};

const toDateRange = ({ from, to }: DateRange) => ({
  fromDate: from ? new Date(`${from}T00:00:00`) : null,
  toDate: to ? new Date(`${to}T23:59:59`) : null
});

export function matchesDateRange(value?: string | null, range: DateRange = {}): boolean {
  if (!value) {
    return true;
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return false;
  }
  const { fromDate, toDate } = toDateRange(range);
  if (fromDate && date < fromDate) {
    return false;
  }
  if (toDate && date > toDate) {
    return false;
  }
  return true;
}

export function matchesSearch(value: string | null | undefined, search: string): boolean {
  const term = search.trim().toLowerCase();
  if (!term) {
    return true;
  }
  return String(value || "").toLowerCase().includes(term);
}
