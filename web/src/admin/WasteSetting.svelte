<script lang="ts">
  import { api, ApiError, type Calendar, type SourceStatus } from './api';
  import { CALENDAR_COLORS } from '../colors';
  import { parseLocalDate } from '../widgets/dates';
  import type { Pickup } from '../widgets/waste';
  import { WASTE_TEST_WEEKS, summarisePickups } from './waste';
  import SourceList from './SourceList.svelte';

  let {
    calendars,
    status,
    onchanged,
    onerror,
  }: {
    calendars: Calendar[];
    /** Present once a dashboard shows waste collection. */
    status: SourceStatus | undefined;
    onchanged: () => Promise<void>;
    onerror: (err: unknown) => string;
  } = $props();

  // The server allows this many (internal/settings MaxWasteCalendars).
  const MAX_WASTE_CALENDARS = 5;

  let testing = $state(false);
  let tested = $state<Pickup[] | null>(null);
  let testError = $state('');
  const found = $derived(tested ? summarisePickups(tested) : null);
  const day = new Intl.DateTimeFormat('en-GB', { weekday: 'short', day: 'numeric', month: 'short' });

  function formatDay(date: string): string {
    const d = parseLocalDate(date);
    return d ? day.format(d) : date;
  }

  async function runTest(url: string) {
    tested = null;
    testError = '';
    if (url === '') {
      testError = 'Paste the link first.';
      return;
    }
    testing = true;
    try {
      tested = await api.testWasteCalendar(url);
    } catch (err) {
      testError = err instanceof ApiError ? (err.fieldMessage('url') ?? err.message) : onerror(err);
    } finally {
      testing = false;
    }
  }
</script>

<SourceList
  title="Waste collection"
  intro="Paste the calendar link of your waste collection, for the Waste collection widget. GDash reads it again every half hour, so a moved pickup shows up by itself. Add one per address, such as home and a holiday house."
  items={calendars.map((c) => ({
    id: c.id,
    name: c.name,
    detail: c.host,
    warning: c.unreadable ? 'This link cannot be read on this server. Remove it and add it again.' : undefined,
  }))}
  {status}
  max={MAX_WASTE_CALENDARS}
  secret
  namePlaceholder="Home"
  urlPlaceholder="webcal://… or https://…/afvalkalender.ics"
  onadd={async (name, url) => {
    await api.addWasteCalendar(name, url);
    tested = null;
    await onchanged();
  }}
  onremove={async (id) => {
    await api.deleteCalendar(id);
    await onchanged();
  }}
  {onerror}
>
  {#snippet help()}
    <details class="where">
      <summary>Where to find the link</summary>
      <ul>
        <li>
          <a href="https://www.mijnafvalwijzer.nl" rel="noopener" target="_blank">mijnafvalwijzer.nl</a>: look up your postcode and
          house number, and copy the Kalender URL under Notificaties.
        </li>
        <li>
          The afvalkalender of your municipality or its waste company, on its site or in its app: look for a button such as “Zet in
          je agenda”, “Agenda koppelen”, iCal, ics or webcal by your address. Copy the link (right-click, Copy link address) instead
          of downloading the file: a downloaded file does not change when a pickup moves.
        </li>
        <li>No link offered? Then GDash cannot read your pickups; a PDF or a list on a web page does not work.</li>
      </ul>
      <p class="small muted">Anyone with the link can read the calendar, so GDash stores it encrypted and never shows it again.</p>
    </details>
  {/snippet}
  {#snippet test(url: string)}
    <button type="button" class="secondary" disabled={testing} onclick={() => runTest(url)}>{testing ? 'Testing…' : 'Test'}</button>
  {/snippet}
  {#snippet results()}
  {#if testError}
    <p class="error" role="alert">{testError}</p>
  {:else if found && tested}
    <div class="found" role="status">
      {#if tested.length === 0}
        <p>
          The calendar was read, but has no pickups in the next {WASTE_TEST_WEEKS} weeks. Check that it is the calendar of
          your address.
        </p>
      {:else}
        <p>
          <strong>{tested.length} {tested.length === 1 ? 'pickup' : 'pickups'}</strong> in the next {WASTE_TEST_WEEKS} weeks.
          {found.kinds.length > 0 ? 'Recognised:' : 'No kind of waste is recognised; the widget shows the titles as they are.'}
        </p>
        {#if found.kinds.length > 0}
          <ul class="kinds">
            {#each found.kinds as k (k.kind)}
              <li>
                <span class="dot" style:background={k.color ? CALENDAR_COLORS[k.color] : 'var(--muted)'}></span>
                <span class="kind">{k.name}</span>
                <span class="when muted small num">next {formatDay(k.first)} · {k.count} in {WASTE_TEST_WEEKS} weeks</span>
              </li>
            {/each}
          </ul>
        {/if}
        {#if found.others.length > 0}
          <p class="small muted">Shown by their own title: {found.others.join(', ')}.</p>
        {/if}
      {/if}
    </div>
  {/if}
  {/snippet}
</SourceList>

<style>
  summary {
    cursor: pointer;
    color: var(--text-soft);
    font-size: var(--fs-sm);
    font-weight: 550;
    min-height: 32px;
    display: flex;
    align-items: center;
    gap: 6px;
  }
  /* A flex summary loses the browser's triangle, so it draws its own. */
  summary::before {
    content: '▸';
  }
  details[open] > summary::before {
    content: '▾';
  }
  .where ul {
    display: grid;
    gap: var(--s-2);
    margin: var(--s-1) 0 var(--s-2);
    padding-left: 1.2em;
    font-size: var(--fs-sm);
    max-width: 70ch;
  }
  .found {
    display: grid;
    gap: var(--s-2);
    padding: var(--s-3) var(--s-4);
    border: 1px solid var(--border);
    border-radius: var(--r-lg);
    background: var(--bg);
    max-width: 40rem;
  }
  .kinds {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: var(--s-1);
  }
  .kinds li {
    display: grid;
    grid-template-columns: 10px minmax(0, 1fr) auto;
    align-items: baseline;
    gap: 2px var(--s-2);
  }
  .dot {
    align-self: center;
    width: 10px;
    height: 10px;
    border-radius: 50%;
  }
  /* A phone puts when under what. */
  @container page (max-width: 30rem) {
    .kinds li {
      grid-template-columns: 10px minmax(0, 1fr);
    }
    .when {
      grid-column: 2;
    }
  }
</style>
