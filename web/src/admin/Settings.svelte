<script lang="ts">
  import { onMount } from 'svelte';
  import { api, type Settings } from './api';
  import { errorMessage } from './errors';
  import AccountSetting from './AccountSetting.svelte';
  import ActivityLog from './ActivityLog.svelte';
  import BackupSetting from './BackupSetting.svelte';
  import AnnouncementSetting from './AnnouncementSetting.svelte';
  import TwoStepSetting from './TwoStepSetting.svelte';
  import UpdateSetting from './UpdateSetting.svelte';

  let { onSessionEnded }: { onSessionEnded: () => void } = $props();

  let settings = $state<Settings | null>(null);
  let loadError = $state('');
  let languageError = $state('');

  const handle = (err: unknown) => errorMessage(err, onSessionEnded);

  // The data sources live under Integrations; what is left is shown in two
  // groups, one at a time.
  const SECTIONS = [
    { id: 'screens', label: 'Screens', hint: 'The language on the TVs and a test announcement' },
    { id: 'account', label: 'Account', hint: 'Password, two-step login, backup, updates and recent activity' },
  ] as const;
  type Section = (typeof SECTIONS)[number]['id'];
  let section = $state<Section>('screens');

  async function chooseLanguage(language: string) {
    languageError = '';
    try {
      await api.setLanguage(language);
      await load();
    } catch (err) {
      languageError = handle(err);
    }
  }

  onMount(() => void load());

  async function load() {
    try {
      settings = (await api.settings()).settings;
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
  <!-- A selector, like the mode switch on a device: one position pressed in. -->
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
{#if section === 'screens'}
  <section class="card">
    <h2>Language on the TVs</h2>
    <p class="muted">Days, dates, weather and other texts on the screens. This web interface stays in English.</p>
    {#if settings}
      <select
        aria-label="Language on the TVs"
        value={settings.language}
        onchange={(e) => chooseLanguage((e.target as HTMLSelectElement).value)}
      >
        <option value="en">English</option>
        <option value="nl">Nederlands</option>
        <option value="de">Deutsch</option>
        <option value="fr">Français</option>
      </select>
    {:else if !loadError}
      <p class="muted">Loading…</p>
    {/if}
    {#if languageError}<p class="error">{languageError}</p>{/if}
  </section>
  <AnnouncementSetting onerror={handle} />
{:else}
  <AccountSetting onerror={handle} />
  <TwoStepSetting onerror={handle} />
  <BackupSetting onerror={handle} />
  <UpdateSetting onerror={handle} />
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
