<script lang="ts">
  import { api, ApiError, type Calendar, type CalendarEvent, type CalendarTest, type SourceStatus } from './api';
  import { parseLocalDate } from '../widgets/dates';
  import { checkCalendarAddress, explainCalendarFailure } from './calendarAddress';
  import SourceList from './SourceList.svelte';

  let {
    calendars,
    status,
    max,
    onchanged,
    onerror,
  }: {
    calendars: Calendar[];
    status: SourceStatus | undefined;
    max: number;
    onchanged: () => Promise<void>;
    onerror: (err: unknown) => string;
  } = $props();

  let testing = $state(false);
  let tested = $state<CalendarTest | null>(null);
  let testError = $state('');
  let testAdvice = $state('');

  const day = new Intl.DateTimeFormat('en-GB', { weekday: 'short', day: 'numeric', month: 'short' });
  const time = new Intl.DateTimeFormat('en-GB', { hour: '2-digit', minute: '2-digit' });

  function when(e: CalendarEvent): string {
    if (e.allDay) {
      const d = parseLocalDate(e.start);
      return d ? `${day.format(d)}, all day` : e.start;
    }
    const d = new Date(e.start);
    return Number.isNaN(d.getTime()) ? e.start : `${day.format(d)}, ${time.format(d)}`;
  }

  async function runTest(url: string) {
    tested = null;
    testError = '';
    testAdvice = '';
    if (url === '') {
      testError = 'Paste the address first.';
      return;
    }
    testing = true;
    try {
      tested = await api.testCalendar(url);
    } catch (err) {
      if (err instanceof ApiError) {
        testError = err.fieldMessage('url') ?? err.message;
        // What is wrong with the address itself already shows under the field.
        testAdvice = checkCalendarAddress(url).problem ? '' : explainCalendarFailure(url, err.code);
      } else {
        testError = onerror(err);
      }
    } finally {
      testing = false;
    }
  }
</script>

<SourceList
  title="Calendars"
  intro="For the Calendar widget: paste the iCal address of a Google, Outlook or iCloud calendar. Anyone with it can read the calendar, so GDash stores it encrypted and never shows it again. A waste collection calendar goes under Waste collection."
  items={calendars.map((c) => ({
    id: c.id,
    name: c.name,
    detail: c.host,
    warning: c.unreadable ? 'This address cannot be read on this server. Remove it and add it again.' : undefined,
    color: c.color,
  }))}
  {status}
  {max}
  secret
  namePlaceholder="Family"
  urlPlaceholder="https://calendar.google.com/calendar/ical/…/basic.ics"
  onadd={async (name, url) => {
    await api.addCalendar(name, url);
    tested = null;
    await onchanged();
  }}
  onremove={async (id) => {
    await api.deleteCalendar(id);
    await onchanged();
  }}
  oncolor={async (id, color) => {
    await api.setCalendarColor(id, color);
    await onchanged();
  }}
  {onerror}
>
  {#snippet help()}
    <details class="where">
      <summary>Google Calendar</summary>
      <ol>
        <li>Open <a href="https://calendar.google.com" rel="noopener" target="_blank">Google Calendar</a> on a computer; the phone app does not show the address.</li>
        <li>Open Settings (the gear), and under “Settings for my calendars” click the calendar.</li>
        <li>Under “Integrate calendar”, copy the <strong>Secret address in iCal format</strong>. It ends in <code>basic.ics</code>.</li>
      </ol>
      <p class="small muted">
        The secret address works while the calendar stays private; anyone who has it can read the calendar, and “Reset” beside it
        makes a new one. The public address only works for a calendar made public, and the address of the page or of “Embed code”
        is no calendar. No secret address in a work or school account (Google Workspace)? Its administrator changed the sharing
        settings; ask them.
      </p>
    </details>
    <details class="where">
      <summary>Outlook and Microsoft 365</summary>
      <ol>
        <li>Open <a href="https://outlook.live.com/calendar/" rel="noopener" target="_blank">Outlook on the web</a> (for Microsoft 365, your organisation’s Outlook).</li>
        <li>Open Settings, Calendar, Shared calendars, and under “Publish a calendar” choose the calendar and “Can view all details”.</li>
        <li>Click Publish and copy the <strong>ICS</strong> link, not the HTML link.</li>
      </ol>
      <p class="small muted">An organisation can turn publishing off; then the section is missing.</p>
    </details>
    <details class="where">
      <summary>iCloud</summary>
      <ol>
        <li>On an iPhone or iPad, open Calendar, tap Calendars and the ⓘ beside the calendar. On a Mac, Control-click the calendar.</li>
        <li>Turn on <strong>Public Calendar</strong> and copy the link it shows. It starts with <code>webcal://</code>, which is fine.</li>
      </ol>
      <p class="small muted">A public iCloud calendar can be read by anyone who has the link; turn Public Calendar off to stop it.</p>
    </details>
    <p class="small muted">Other calendars, such as Nextcloud or Proton: look for a link named iCal, ICS, webcal or subscribe.</p>
  {/snippet}
  {#snippet hint(url: string)}
    {@const check = checkCalendarAddress(url)}
    {#if check.problem}
      <p class="field-error" role="status">{check.problem}</p>
    {:else if check.note}
      <p class="small muted" role="status">{check.note}</p>
    {/if}
  {/snippet}
  {#snippet test(url: string)}
    <button type="button" class="secondary" disabled={testing} onclick={() => runTest(url)}>{testing ? 'Testing…' : 'Test'}</button>
  {/snippet}
  {#snippet results()}
    {#if testError}
      <div class="error" role="alert">
        <p>{testError}</p>
        {#if testAdvice}<p class="small">{testAdvice}</p>{/if}
      </div>
    {:else if tested}
      <div class="found" role="status">
        {#if tested.count === 0}
          <p>The calendar{tested.name ? ` “${tested.name}”` : ''} was read, but has nothing in the next 30 days.</p>
        {:else}
          <p>
            <strong>{tested.count === 1 ? '1 event' : `${tested.count} events`}</strong> in the next 30 days{tested.name
              ? ` in “${tested.name}”`
              : ''}. The first:
          </p>
          <ul class="events">
            {#each tested.events as e, i (i)}
              <li><span class="when muted small num">{when(e)}</span> <span>{e.title}</span></li>
            {/each}
          </ul>
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
  .where ol {
    display: grid;
    gap: var(--s-2);
    margin: var(--s-1) 0 var(--s-2);
    padding-left: 1.4em;
    font-size: var(--fs-sm);
    max-width: 70ch;
  }
  .where p {
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
  .found p,
  .error p {
    margin: 0;
  }
  .error {
    display: grid;
    gap: var(--s-1);
  }
  .events {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: var(--s-1);
  }
  .when {
    display: inline-block;
    min-width: 11em;
  }
</style>
