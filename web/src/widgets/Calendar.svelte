<script lang="ts">
  import type { Entry } from '../protocol';
  import { waitingText } from './waiting';
  import { fitItems } from './fit';
  import { clock } from '../state.svelte';
  import { locale, t } from '../i18n.svelte';

  interface CalendarEvent {
    title: string;
    start: string;
    end: string;
    allDay: boolean;
    calendar: string;
  }
  interface Day {
    key: string;
    label: string;
    events: CalendarEvent[];
  }

  const DEFAULT_ITEMS = 8;

  let { entry, settings }: { entry: Entry | undefined; settings: Record<string, unknown> | undefined } = $props();

  const events = $derived(isEventList(entry?.value) ? entry.value : undefined);
  const maxItems = $derived(typeof settings?.maxItems === 'number' ? settings.maxItems : DEFAULT_ITEMS);
  const stale = $derived(entry !== undefined && clock.now.getTime() > Date.parse(entry.staleAfter));
  const days = $derived(events ? groupByDay(events.slice(0, maxItems), clock.now) : []);

  const time = $derived(new Intl.DateTimeFormat(locale(), { hour: '2-digit', minute: '2-digit' }));
  const weekday = $derived(new Intl.DateTimeFormat(locale(), { weekday: 'long', day: 'numeric', month: 'long' }));

  function isEventList(v: unknown): v is CalendarEvent[] {
    return (
      Array.isArray(v) &&
      v.every((e) => typeof e === 'object' && e !== null && typeof e.title === 'string' && typeof e.start === 'string')
    );
  }

  /** Local calendar date of an event start, as YYYY-MM-DD. */
  function dateKey(d: Date): string {
    const pad = (n: number) => String(n).padStart(2, '0');
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
  }

  /** All-day dates have no zone; read them as local dates, not UTC. */
  function parseDate(value: string, allDay: boolean): Date {
    if (allDay) {
      const [y, m, d] = value.split('-').map(Number);
      return new Date(y ?? 0, (m ?? 1) - 1, d ?? 1);
    }
    return new Date(value);
  }

  function groupByDay(list: CalendarEvent[], now: Date): Day[] {
    const today = dateKey(now);
    const tomorrow = dateKey(new Date(now.getFullYear(), now.getMonth(), now.getDate() + 1));
    const byDay = new Map<string, Day>();
    for (const e of list) {
      // An event that began before today but still runs is listed today.
      let key = dateKey(parseDate(e.start, e.allDay));
      if (key < today) key = today;
      const label = key === today ? t('today') : key === tomorrow ? t('tomorrow') : weekday.format(parseDate(key, true));
      const day = byDay.get(key) ?? { key, label, events: [] };
      day.events.push(e);
      byDay.set(key, day);
    }
    return [...byDay.values()].sort((a, b) => (a.key < b.key ? -1 : 1));
  }

  function when(e: CalendarEvent): string {
    return e.allDay ? t('allDay') : time.format(new Date(e.start));
  }

  function isPast(e: CalendarEvent, now: Date): boolean {
    return parseDate(e.end, e.allDay).getTime() <= now.getTime();
  }
</script>

<div class="calendar" use:fitItems>
  {#if events && entry}
    {#if days.length === 0}
      <p class="empty" class:stale>{t('nothingPlanned')}</p>
    {/if}
    {#each days as day (day.key)}
      <section class="day" data-fit-group>
        <h2 class:stale>{day.label}</h2>
        <ul>
          {#each day.events as e, i (i)}
            <li class:past={isPast(e, clock.now)}>
              <span class="when">{when(e)}</span>
              <span class="title">{e.title}</span>
            </li>
          {/each}
        </ul>
      </section>
    {/each}
  {:else}
    <span class="empty">{waitingText(entry, 'calendar')}</span>
  {/if}
</div>

<style>
  .calendar {
    display: flex;
    flex-direction: column;
    gap: 18px;
    width: 100%;
    min-width: 0;
    overflow: hidden;
  }
  h2 {
    margin: 0 0 6px;
    font-size: 26px;
    font-weight: 600;
    letter-spacing: 0.04em;
    text-transform: uppercase;
    color: var(--text-muted);
  }
  h2.stale,
  .empty.stale {
    color: var(--warning);
  }
  ul {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: 6px;
  }
  li {
    display: flex;
    gap: 24px;
    align-items: baseline;
    min-width: 0;
  }
  li.past {
    opacity: 0.45;
  }
  .when {
    flex: none;
    width: 4.2em;
    font-size: 28px;
    color: var(--text-muted);
    font-variant-numeric: tabular-nums;
  }
  .title {
    font-size: 36px;
    color: var(--text-primary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    min-width: 0;
  }
  .empty {
    margin: 0;
    font-size: 38px;
    color: var(--text-muted);
  }
</style>
