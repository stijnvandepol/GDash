import type { CastDevice, Display } from './api';

/** The model a speaker group announces. */
const GROUP_MODEL = 'Google Cast Group';

/** One device in the Now playing widget's device list. */
export interface DeviceChoice {
  id: string;
  name: string;
  /** The model, or that it is a speaker group. */
  detail: string;
  /** Why it cannot be chosen: GDash shows a dashboard on it. */
  blockedBy?: string;
  /** Chosen earlier, but not found on the network now. */
  missing?: boolean;
}

/**
 * The devices a Now playing widget can follow: every Cast device found,
 * groups first, with the TVs that displays show a dashboard on blocked
 * (GDash never reads what plays there), and a chosen device that is not
 * found now kept, so it can be removed.
 */
export function deviceChoices(devices: CastDevice[], displays: Display[], selected: string[]): DeviceChoice[] {
  const owner = new Map(displays.filter((d) => d.castDeviceId).map((d) => [d.castDeviceId, d.name]));
  const found: DeviceChoice[] = devices.map((d) => {
    const choice: DeviceChoice = { id: d.id, name: d.name || d.id, detail: d.model === GROUP_MODEL ? 'Speaker group' : d.model };
    const blockedBy = owner.get(d.id);
    if (blockedBy !== undefined) choice.blockedBy = blockedBy;
    return choice;
  });
  const isGroup = (c: DeviceChoice) => (c.detail === 'Speaker group' ? 0 : 1);
  found.sort((a, b) => isGroup(a) - isGroup(b) || a.name.localeCompare(b.name));
  const missing = selected
    .filter((id) => !devices.some((d) => d.id === id))
    .map((id): DeviceChoice => ({ id, name: 'Not found on the network', detail: id, missing: true }));
  return [...missing, ...found];
}
