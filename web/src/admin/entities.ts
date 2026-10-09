import type { HomeAssistantEntity } from './api';

/** How many matches the entity search lists; a home can have thousands. */
export const MAX_MATCHES = 30;

/**
 * The entities whose name or ID contain every word of the query, leaving
 * out those already chosen. Names come first, so "kitchen temp" finds
 * "Kitchen temperature" before an entity that only has it in its ID.
 */
export function matchEntities(
  entities: HomeAssistantEntity[],
  query: string,
  chosen: string[],
): HomeAssistantEntity[] {
  const words = query.toLowerCase().split(/\s+/).filter(Boolean);
  if (words.length === 0) return [];
  const inName: HomeAssistantEntity[] = [];
  const inId: HomeAssistantEntity[] = [];
  for (const e of entities) {
    if (chosen.includes(e.id)) continue;
    const name = e.name.toLowerCase();
    const text = `${name} ${e.id}`;
    if (!words.every((w) => text.includes(w))) continue;
    (words.every((w) => name.includes(w)) ? inName : inId).push(e);
  }
  return [...inName, ...inId].slice(0, MAX_MATCHES);
}
