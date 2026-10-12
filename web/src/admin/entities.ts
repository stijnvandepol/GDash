/** How many matches the entity search lists; a home can have thousands. */
export const MAX_MATCHES = 30;

/** Something to find with the search: a Home Assistant entity or a Homey value. */
export interface Searchable {
  id: string;
  name: string;
  /** The room it is in, which counts as part of its name. */
  zone?: string;
}

/**
 * The entities whose name, zone or ID contain every word of the query,
 * leaving out those already chosen. Names come first, so "kitchen temp"
 * finds "Kitchen temperature" before an entity that only has it in its ID.
 */
export function matchEntities<T extends Searchable>(entities: T[], query: string, chosen: string[]): T[] {
  const words = query.toLowerCase().split(/\s+/).filter(Boolean);
  if (words.length === 0) return [];
  const inName: T[] = [];
  const inId: T[] = [];
  for (const e of entities) {
    if (chosen.includes(e.id)) continue;
    const name = `${e.zone ?? ''} ${e.name}`.toLowerCase();
    const text = `${name} ${e.id}`;
    if (!words.every((w) => text.includes(w))) continue;
    (words.every((w) => name.includes(w)) ? inName : inId).push(e);
  }
  return [...inName, ...inId].slice(0, MAX_MATCHES);
}
