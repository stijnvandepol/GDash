<script lang="ts">
  import './admin.css';
  import { onMount } from 'svelte';
  import { api, ApiError, type SystemInfo, type User } from './api';
  import Setup from './Setup.svelte';
  import Login from './Login.svelte';
  import Displays from './Displays.svelte';
  import Dashboards from './Dashboards.svelte';
  import Settings from './Settings.svelte';
  import GettingStarted from './GettingStarted.svelte';
  import Icon, { type IconName } from './Icon.svelte';
  import Logo from './Logo.svelte';
  import Toasts from './Toasts.svelte';
  import UpdateNotice from './UpdateNotice.svelte';
  import ConfirmDialog from './ConfirmDialog.svelte';
  import { toast } from './toast.svelte';

  type Screen = { name: 'loading' } | { name: 'error'; message: string } | { name: 'setup'; tokenRequired: boolean } | { name: 'login' } | { name: 'app'; user: User };
  type Tab = 'displays' | 'dashboards' | 'settings';

  const TABS: { id: Tab; label: string; icon: IconName }[] = [
    { id: 'displays', label: 'Displays', icon: 'display' },
    { id: 'dashboards', label: 'Dashboards', icon: 'dashboards' },
    { id: 'settings', label: 'Settings', icon: 'settings' },
  ];

  let screen = $state<Screen>({ name: 'loading' });
  let tab = $state<Tab>('displays');
  let system = $state<SystemInfo | null>(null);
  let menuOpen = $state(false);
  let menuElement = $state<HTMLElement>();

  // The version and a newer release are only for logged-in admins; the
  // footer shows the version once known. A failure leaves both out: they
  // are information, not a reason to stop.
  $effect(() => {
    if (screen.name !== 'app') return;
    api
      .system()
      .then((s) => (system = s))
      .catch(() => (system = null));
  });

  onMount(async () => {
    try {
      const setup = await api.setupState();
      if (setup.needed) {
        screen = { name: 'setup', tokenRequired: setup.tokenRequired };
        return;
      }
      screen = { name: 'app', user: await api.me() };
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        screen = { name: 'login' };
      } else {
        screen = { name: 'error', message: err instanceof Error ? err.message : 'Unknown error' };
      }
    }
  });

  async function logout() {
    menuOpen = false;
    await api.logout();
    screen = { name: 'login' };
    toast.success('You are logged out.');
  }

  function sessionEnded() {
    screen = { name: 'login' };
    toast.info('Your session has ended. Log in again.');
  }

  function created(user: User) {
    screen = { name: 'app', user };
    toast.success('Account created. Welcome to GDash.');
  }

  function closeMenuOnOutsideClick(event: MouseEvent) {
    if (menuOpen && menuElement && !menuElement.contains(event.target as Node)) menuOpen = false;
  }

  function closeMenuOnEscape(event: KeyboardEvent) {
    if (event.key === 'Escape' && menuOpen) {
      menuOpen = false;
      menuElement?.querySelector('button')?.focus();
    }
  }
</script>

<svelte:window onclick={closeMenuOnOutsideClick} onkeydown={closeMenuOnEscape} />

<div class="shell" class:with-tabs={screen.name === 'app'}>
  {#if screen.name === 'app'}
    <header class="topbar">
      <div class="topbar-inner">
        <span class="brand"><Logo size={30} /> GDash</span>
        <nav class="tabs" aria-label="Main">
          {#each TABS as t (t.id)}
            <button class="tab" aria-current={tab === t.id ? 'page' : undefined} onclick={() => (tab = t.id)}>
              <Icon name={t.icon} />
              {t.label}
            </button>
          {/each}
        </nav>
        <span class="spacer"></span>
        <div class="user" bind:this={menuElement}>
          <button class="user-button" aria-haspopup="menu" aria-expanded={menuOpen} onclick={() => (menuOpen = !menuOpen)}>
            <span class="avatar" aria-hidden="true">{screen.user.username.charAt(0)}</span>
            <span class="user-name">{screen.user.username}</span>
            <span class="sr-only">Account menu</span>
            <Icon name="chevron" size={16} />
          </button>
          {#if menuOpen}
            <div class="menu" role="menu">
              <p class="menu-label">Signed in as <strong>{screen.user.username}</strong></p>
              <button class="menu-item" role="menuitem" onclick={logout}><Icon name="logout" /> Log out</button>
            </div>
          {/if}
        </div>
      </div>
    </header>
  {/if}

  {#if screen.name === 'setup' || screen.name === 'login'}
    <main class="auth">
      {#if screen.name === 'setup'}
        <Setup tokenRequired={screen.tokenRequired} onDone={created} />
      {:else}
        <Login onDone={(user) => (screen = { name: 'app', user })} />
      {/if}
    </main>
  {:else}
    <main class="content">
      {#if screen.name === 'loading'}
        <p class="loading muted" role="status">Loading…</p>
      {:else if screen.name === 'error'}
        <p class="error">GDash cannot be reached: {screen.message}</p>
      {:else}
        {#if system?.update}<UpdateNotice release={system.update} />{/if}
        <GettingStarted onopen={(next) => (tab = next)} />
        {#if tab === 'displays'}
          <Displays onSessionEnded={sessionEnded} />
        {:else if tab === 'dashboards'}
          <Dashboards onSessionEnded={sessionEnded} />
        {:else}
          <Settings onSessionEnded={sessionEnded} />
        {/if}
      {/if}
    </main>
  {/if}

  <footer class="footer muted small">
    GDash{system ? ` ${system.version}` : ''} is free software under the
    <a href="https://www.gnu.org/licenses/agpl-3.0.html" rel="noopener">AGPL-3.0</a>.
    <a href="https://github.com/stijnvandepol/gdash" rel="noopener">Source code</a>
  </footer>

  <Toasts />
  <ConfirmDialog />
</div>
