<script lang="ts">
  import { api, ApiError, type User } from './api';
  import Logo from './Logo.svelte';
  import { isPendingLogin } from './twoStep';

  let { onDone }: { onDone: (user: User) => void } = $props();

  let username = $state('');
  let password = $state('');
  // Set when the password was right and the account has two-step login on:
  // the login waits for a code.
  let pending = $state('');
  let code = $state('');
  let useRecoveryCode = $state(false);
  let error = $state('');
  let busy = $state(false);

  async function submit(event: SubmitEvent) {
    event.preventDefault();
    busy = true;
    error = '';
    try {
      if (pending) {
        onDone(await api.finishLogin(pending, code));
        return;
      }
      const answer = await api.login(username, password);
      if (isPendingLogin(answer)) {
        pending = answer.pending;
        password = '';
      } else {
        onDone(answer);
      }
    } catch (err) {
      if (err instanceof ApiError && err.code === 'login_expired') startOver();
      code = '';
      error = err instanceof ApiError ? err.message : 'The server cannot be reached.';
    } finally {
      busy = false;
    }
  }

  function startOver() {
    pending = '';
    code = '';
    error = '';
    useRecoveryCode = false;
  }

  function toggleRecoveryCode() {
    useRecoveryCode = !useRecoveryCode;
    code = '';
    error = '';
  }

  function focus(input: HTMLInputElement) {
    input.focus();
  }
</script>

<div class="auth-card">
  <div class="auth-brand">
    <Logo size={56} />
    <span class="name">GDash</span>
    <p>Your home dashboard for the TV.</p>
  </div>

  <form class="card auth-form" onsubmit={submit}>
    {#if pending}
      <h1>Two-step login</h1>
      {#if useRecoveryCode}
        <p class="muted">Enter one of the recovery codes you saved when you turned on two-step login. Each works once.</p>
        <label>
          Recovery code
          <input bind:value={code} use:focus autocomplete="off" spellcheck="false" required />
        </label>
      {:else}
        <p class="muted">Enter the 6-digit code that your authenticator app shows for GDash.</p>
        <label>
          Code
          <input bind:value={code} use:focus autocomplete="one-time-code" inputmode="numeric" spellcheck="false" required />
        </label>
      {/if}
      {#if error}<p class="error" role="alert">{error}</p>{/if}
      <button type="submit" class:busy aria-busy={busy} disabled={busy}>{busy ? 'Working…' : 'Log in'}</button>
      <div class="other-ways">
        <button type="button" class="link" onclick={toggleRecoveryCode}>
          {useRecoveryCode ? 'Use a code from the app' : 'Use a recovery code'}
        </button>
        <button type="button" class="link" onclick={startOver}>Start over</button>
      </div>
    {:else}
      <h1>Log in</h1>
      <label>
        Username
        <input bind:value={username} autocomplete="username" required />
      </label>
      <label>
        Password
        <input type="password" bind:value={password} autocomplete="current-password" required />
      </label>
      {#if error}<p class="error" role="alert">{error}</p>{/if}
      <button type="submit" class:busy aria-busy={busy} disabled={busy}>{busy ? 'Working…' : 'Log in'}</button>
    {/if}
  </form>
</div>

<style>
  .other-ways {
    display: flex;
    flex-wrap: wrap;
    justify-content: space-between;
    gap: var(--s-3);
  }
</style>
