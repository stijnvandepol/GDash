<script lang="ts">
  import './admin.css';
  import { onMount } from 'svelte';
  import { api, ApiError, type User } from './api';
  import Setup from './Setup.svelte';
  import Login from './Login.svelte';
  import Displays from './Displays.svelte';
  import Dashboards from './Dashboards.svelte';
  import Settings from './Settings.svelte';
  import GettingStarted from './GettingStarted.svelte';

  type Screen = { name: 'loading' } | { name: 'error'; message: string } | { name: 'setup' } | { name: 'login' } | { name: 'app'; user: User };

  let screen = $state<Screen>({ name: 'loading' });
  let tab = $state<'displays' | 'dashboards' | 'settings'>('displays');
  let version = $state('');

  // The version is only for logged-in admins; the footer shows it once known.
  $effect(() => {
    if (screen.name !== 'app') return;
    api
      .system()
      .then((s) => (version = s.version))
      .catch(() => (version = ''));
  });

  onMount(async () => {
    try {
      if (await api.setupNeeded()) {
        screen = { name: 'setup' };
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
    await api.logout();
    screen = { name: 'login' };
  }
</script>

<header class="topbar">
  <span class="brand">Nookboard</span>
  {#if screen.name === 'app'}
    <nav class="tabs">
      <button class="tab" class:active={tab === 'displays'} onclick={() => (tab = 'displays')}>Displays</button>
      <button class="tab" class:active={tab === 'dashboards'} onclick={() => (tab = 'dashboards')}>Dashboards</button>
      <button class="tab" class:active={tab === 'settings'} onclick={() => (tab = 'settings')}>Settings</button>
    </nav>
    <span class="spacer"></span>
    <span class="muted">{screen.user.username}</span>
    <button class="link" onclick={logout}>Log out</button>
  {/if}
</header>

<main class="content">
  {#if screen.name === 'loading'}
    <p class="muted">Loading…</p>
  {:else if screen.name === 'error'}
    <p class="error">Nookboard cannot be reached: {screen.message}</p>
  {:else if screen.name === 'setup'}
    <Setup onDone={(user) => (screen = { name: 'app', user })} />
  {:else if screen.name === 'login'}
    <Login onDone={(user) => (screen = { name: 'app', user })} />
  {:else}
    <GettingStarted onopen={(next) => (tab = next)} />
    {#if tab === 'displays'}
      <Displays onSessionEnded={() => (screen = { name: 'login' })} />
    {:else if tab === 'dashboards'}
      <Dashboards onSessionEnded={() => (screen = { name: 'login' })} />
    {:else}
      <Settings onSessionEnded={() => (screen = { name: 'login' })} />
    {/if}
  {/if}
</main>

<footer class="footer muted small">
  Nookboard{version ? ` ${version}` : ''} is free software under the
  <a href="https://www.gnu.org/licenses/agpl-3.0.html" rel="noopener">AGPL-3.0</a>.
  <a href="https://github.com/stijnvandepol/nookboard" rel="noopener">Source code</a>
</footer>
