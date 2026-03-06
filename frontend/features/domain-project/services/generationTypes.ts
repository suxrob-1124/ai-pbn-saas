export const GENERATION_TYPES = {
  single_page: { label: 'Одностраничный', available: true },
  webarchive_single: { label: 'Вебархив', available: true },
  webarchive_multi: { label: 'Многостраничник', available: false },
  webarchive_eeat: { label: 'EEAT', available: false },
  branded: { label: 'Брендовый', available: false },
  branded_content: { label: 'Бренд + контент', available: false },
} as const;

export type GenerationType = keyof typeof GENERATION_TYPES;

export function isGenerationTypeAvailable(type: string): boolean {
  const entry = GENERATION_TYPES[type as GenerationType];
  return !!entry && entry.available;
}

export function getGenerationTypeLabel(type: string): string {
  const entry = GENERATION_TYPES[type as GenerationType];
  return entry ? entry.label : type;
}
