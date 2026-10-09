<script lang="ts">
  import { onMount } from 'svelte';
  import { api, type Settings, type SourceStatus } from './api';
  import { errorMessage } from './errors';
  import { newestOnly } from './newest';
  import AccountSetting from './AccountSetting.svelte';
  import ActivityLog from './ActivityLog.svelte';
  import LocationSetting from './LocationSetting.svelte';
  import PhotoSetting from './PhotoSetting.svelte';
  import SourceList from './SourceList.svelte';

  let { onSessionEnded }: { onSessionEnded: () => void } = $props();

  // The server allows these numbers; internal/settings holds the same.
  const MAX_NEWS_FEEDS = 5;
  const MAX_CALENDARS = 10;
  // A new source is fetched within seconds; reloading shows its status.
  const STATUS_REFRESH_MS = 5_000;

  let settings = $state<Settings | null>(null);
  let status = $state<Record<string, SourceStatus>>({});
  let loadError = $state('');

  const handle = (err: unknown) => errorMessage(err, onSessionEnded);

  let languageError = $state('');

  async function chooseLanguage(language: string) {
    languageError = '';
    try {
      await api.setLanguage(language);
      await load();
    } catch (err) {
      languageError = handle(err);
    }
  }

  onMount(() => {
    void load();
    const timer = setInterval(() => void load(), STATUS_REFRESH_MS);
    return () => clearInterval(timer);
  });

  const fetchSettings = newestOnly(() => api.settings());

  async function load() {
    try {
      const newest = await fetchSettings();
      if (!newest) return;
      settings = newest.value.settings;
      status = newest.value.status;
      loadError = '';
    } catch (err) {
      loadError = handle(err);
    }
  }
</script>

{#if loadError}<p class="error">{loadError}</p>{/if}
{#if settings}
  <LocationSetting location={settings.location} status={status['weather.current']} onchanged={load} onerror={handle} />

  <SourceList
    title="News"
    intro="RSS or Atom feeds. With more than one, the newest headlines of all feeds are shown together."
    items={settings.newsFeeds.map((f) => ({ id: f.id, name: f.name, detail: f.url }))}
    status={status['news.default']}
    max={MAX_NEWS_FEEDS}
    namePlaceholder="NOS"
    urlPlaceholder="https://feeds.nos.nl/nosnieuwsalgemeen"
    onadd={async (name, url) => {
      await api.addNewsFeed(name, url);
      await load();
    }}
    onremove={async (id) => {
      await api.deleteNewsFeed(id);
      await load();
    }}
    onerror={handle}
  />

  <SourceList
    title="Calendars"
    intro="Paste the secret iCal address of a Google, Outlook or iCloud calendar. Anyone with it can read the calendar, so Daycast stores it encrypted and never shows it again."
    items={settings.calendars.map((c) => ({
      id: c.id,
      name: c.name,
      detail: c.host,
      warning: c.unreadable ? 'This address cannot be read on this server. Remove it and add it again.' : undefined,
      color: c.color,
    }))}
    status={status['calendar.default']}
    max={MAX_CALENDARS}
    secret
    namePlaceholder="Family"
    urlPlaceholder="https://calendar.google.com/calendar/ical/…/basic.ics"
    onadd={async (name, url) => {
      await api.addCalendar(name, url);
      await load();
    }}
    onremove={async (id) => {
      await api.deleteCalendar(id);
      await load();
    }}
    oncolor={async (id, color) => {
      await api.setCalendarColor(id, color);
      await load();
    }}
    onerror={handle}
  />

  <section class="card">
    <h2>Language on the TVs</h2>
    <p class="muted">Days, dates, weather and other texts on the screens. This web interface stays in English.</p>
    <select
      aria-label="Language on the TVs"
      value={settings.language}
      onchange={(e) => chooseLanguage((e.target as HTMLSelectElement).value)}
    >
      <option value="en">English</option>
      <option value="nl">Nederlands</option>
    </select>
    {#if languageError}<p class="error">{languageError}</p>{/if}
  </section>
{:else if !loadError}
  <p class="muted">Loading…</p>
{/if}

<PhotoSetting onerror={handle} />

<AccountSetting onerror={handle} />

<ActivityLog onerror={handle} />
