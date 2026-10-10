<script lang="ts">
  import { onMount } from 'svelte';
  import { api, type Settings, type SourceStatus } from './api';
  import { errorMessage } from './errors';
  import { newestOnly } from './newest';
  import AccountSetting from './AccountSetting.svelte';
  import ActivityLog from './ActivityLog.svelte';
  import BackupSetting from './BackupSetting.svelte';
  import HomeAssistantSetting from './HomeAssistantSetting.svelte';
  import LocationSetting from './LocationSetting.svelte';
  import PhotoSetting from './PhotoSetting.svelte';
  import AnnouncementSetting from './AnnouncementSetting.svelte';
  import ApiTokenSetting from './ApiTokenSetting.svelte';
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

  // Settings is long; it is shown in four groups, one at a time, so a
  // household finds a setting by what it is about.
  const SECTIONS = [
    { id: 'sources', label: 'Data sources', hint: 'Weather, news, calendars and Home Assistant' },
    { id: 'screens', label: 'Screens', hint: 'Language on the TVs and photos' },
    { id: 'automations', label: 'Automations', hint: 'API tokens and announcements' },
    { id: 'account', label: 'Account', hint: 'Password, backup and recent activity' },
  ] as const;
  type Section = (typeof SECTIONS)[number]['id'];
  let section = $state<Section>('sources');

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

<header class="page-head settings-head">
  <div>
    <h1 class="page-title">Settings</h1>
    <p class="page-lede">{SECTIONS.find((s) => s.id === section)?.hint}.</p>
  </div>
  <!-- A selector, like the mode switch on a device: four positions, one
       pressed in. -->
  <nav class="subnav" aria-label="Settings">
    {#each SECTIONS as s, i (s.id)}
      <button class="subtab" aria-current={section === s.id ? 'page' : undefined} onclick={() => (section = s.id)}>
        <span class="index mono" aria-hidden="true">0{i + 1}</span>
        {s.label}
      </button>
    {/each}
  </nav>
</header>

{#if loadError}<p class="error">{loadError}</p>{/if}
{#if section === 'sources' || section === 'screens'}
  {#if settings}
    {#if section === 'sources'}
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
      intro="Paste the secret iCal address of a Google, Outlook or iCloud calendar. Anyone with it can read the calendar, so GDash stores it encrypted and never shows it again."
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

    <HomeAssistantSetting
      connection={settings.homeAssistant}
      status={status['homeassistant.default']}
      onchanged={load}
      onerror={handle}
    />

    {:else}
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
    {/if}
  {:else if !loadError}
    <p class="muted">Loading…</p>
  {/if}
{/if}

{#if section === 'screens'}
  <PhotoSetting onerror={handle} />
{:else if section === 'automations'}
  <ApiTokenSetting onerror={handle} />
  <AnnouncementSetting onerror={handle} />
{:else if section === 'account'}
  <AccountSetting onerror={handle} />
  <BackupSetting onerror={handle} />
  <ActivityLog onerror={handle} />
{/if}

<style>
  .settings-head {
    align-items: flex-end;
  }
  .subnav {
    display: flex;
    flex-wrap: wrap;
    gap: 2px;
    padding: 3px;
    border: 1px solid var(--border-strong);
    border-radius: var(--r-md);
    max-width: 100%;
  }
  .subtab {
    min-height: 34px;
    padding: 0 var(--s-3);
    border: 0;
    border-radius: var(--r-sm);
    background: transparent;
    color: var(--text-soft);
    font-weight: 550;
    /* A switch has no in-between position. */
    transition: none;
  }
  .subtab:hover:not(:disabled) {
    background: var(--neutral-soft);
  }
  .index {
    color: var(--muted);
    font-size: var(--fs-xs);
  }
  .subtab[aria-current='page'],
  .subtab[aria-current='page']:hover:not(:disabled) {
    background: var(--primary);
    color: var(--on-primary);
  }
  .subtab[aria-current='page'] .index {
    color: var(--spark-on-primary);
  }
  .subtab:active:not(:disabled) {
    transform: none;
  }
</style>
