<script lang="ts">
  import { onMount } from 'svelte';
  import { api, ApiError, type TwoStepEnrolment, type TwoStepStatus } from './api';
  import { toast } from './toast.svelte';
  import { groupSecret, qrImageSource, recoveryCodesText } from './twoStep';

  let { onerror }: { onerror: (err: unknown) => string } = $props();

  // Below this many recovery codes the card suggests making new ones.
  const FEW_RECOVERY_CODES = 3;

  let status = $state<TwoStepStatus | null>(null);
  let username = $state('');
  // The secret of a setup that waits for its first code.
  let enrolment = $state<TwoStepEnrolment | null>(null);
  // Shown once, right after turning two-step login on; the server keeps
  // only their hashes.
  let recoveryCodes = $state<string[] | null>(null);
  let password = $state('');
  let code = $state('');
  let fieldErrors = $state<{ password?: string; code?: string }>({});
  let error = $state('');
  let busy = $state(false);

  onMount(() => void load());

  async function load() {
    try {
      [status, { username }] = await Promise.all([api.twoStep(), api.me()]);
    } catch (err) {
      error = onerror(err);
    }
  }

  /** Runs one change, showing field errors at their field and others below the card. */
  async function run(event: SubmitEvent, change: () => Promise<void>) {
    event.preventDefault();
    fieldErrors = {};
    error = '';
    busy = true;
    try {
      await change();
    } catch (err) {
      if (err instanceof ApiError && err.fields.length > 0) {
        fieldErrors = { password: err.fieldMessage('password'), code: err.fieldMessage('code') };
      } else {
        error = onerror(err);
      }
    } finally {
      busy = false;
    }
  }

  const start = (event: SubmitEvent) =>
    run(event, async () => {
      enrolment = await api.startTwoStep(password);
      password = '';
    });

  const enable = (event: SubmitEvent) =>
    run(event, async () => {
      recoveryCodes = await api.enableTwoStep(code);
      enrolment = null;
      code = '';
      await load();
    });

  const disable = (event: SubmitEvent) =>
    run(event, async () => {
      await api.disableTwoStep(password, code);
      password = code = '';
      toast.success('Two-step login is off.');
      await load();
    });

  function cancel() {
    enrolment = null;
    code = '';
    fieldErrors = {};
  }

  function download(codes: string[]) {
    const file = new Blob([recoveryCodesText(username, codes, new Date())], { type: 'text/plain' });
    const link = document.createElement('a');
    link.href = URL.createObjectURL(file);
    link.download = 'gdash-recovery-codes.txt';
    link.click();
    // Firefox may still be reading the address a moment later.
    setTimeout(() => URL.revokeObjectURL(link.href), 10_000);
  }
</script>

<section class="card">
  <h2>Two-step login</h2>
  <p class="muted">
    After your password, GDash also asks for a code from an authenticator app on your phone, such as Google
    Authenticator, Microsoft Authenticator or the passwords app of your phone. Someone who learns your password still
    cannot log in.
  </p>

  {#if recoveryCodes}
    <div class="fresh">
      <p><strong>Two-step login is on.</strong> Save these recovery codes now: they are not shown again.</p>
      <p class="small muted">
        Each code logs in once when your phone is not at hand. Keep them away from your phone, for example printed or in a
        password manager. Other browsers that were logged in must log in again.
      </p>
      <ol class="codes">
        {#each recoveryCodes as recoveryCode (recoveryCode)}<li>{recoveryCode}</li>{/each}
      </ol>
      <div class="actions">
        <button type="button" class="secondary" onclick={() => recoveryCodes && download(recoveryCodes)}>Download as text</button>
        <button type="button" class="link" onclick={() => (recoveryCodes = null)}>I have saved them</button>
      </div>
    </div>
  {:else if status?.enabled}
    <p class="badge ok">On</p>
    <p class="muted">
      {status.recoveryCodesLeft === 1 ? '1 recovery code' : `${status.recoveryCodesLeft} recovery codes`} left.
      {#if status.recoveryCodesLeft <= FEW_RECOVERY_CODES}Turn two-step login off and on again for ten new ones.{/if}
    </p>
    <form onsubmit={disable}>
      <label>
        Your password
        <input type="password" bind:value={password} autocomplete="current-password" required />
        {#if fieldErrors.password}<span class="field-error">{fieldErrors.password}</span>{/if}
      </label>
      <label>
        Code from your app, or a recovery code
        <input bind:value={code} autocomplete="one-time-code" spellcheck="false" required />
        {#if fieldErrors.code}<span class="field-error">{fieldErrors.code}</span>{/if}
      </label>
      <button type="submit" class="danger" class:busy disabled={busy}>Turn off two-step login</button>
    </form>
  {:else if enrolment}
    <ol class="steps">
      <li>
        Scan this QR code with your authenticator app.
        <img class="qr" src={qrImageSource(enrolment.qr)} alt="QR code with the two-step login key for GDash" />
      </li>
      <li>
        Or add an account by hand, with this key (time-based):
        <span class="secret">{groupSecret(enrolment.secret)}</span>
      </li>
      <li>Enter the 6-digit code the app shows for GDash.</li>
    </ol>
    <form onsubmit={enable}>
      <label>
        Code
        <input bind:value={code} autocomplete="one-time-code" inputmode="numeric" spellcheck="false" required />
        {#if fieldErrors.code}<span class="field-error">{fieldErrors.code}</span>{/if}
      </label>
      <div class="actions">
        <button type="submit" class:busy disabled={busy}>Turn on two-step login</button>
        <button type="button" class="link" onclick={cancel}>Cancel</button>
      </div>
    </form>
  {:else if status}
    <p class="badge">Off</p>
    <form onsubmit={start}>
      <label>
        Your password
        <input type="password" bind:value={password} autocomplete="current-password" required />
        {#if fieldErrors.password}<span class="field-error">{fieldErrors.password}</span>{/if}
      </label>
      <button type="submit" class="secondary" class:busy disabled={busy}>Set up two-step login</button>
    </form>
  {:else if !error}
    <p class="muted">Loading…</p>
  {/if}
  {#if error}<p class="error">{error}</p>{/if}
</section>

<style>
  .fresh {
    display: grid;
    gap: var(--s-3);
    padding: var(--s-4);
    border: 1px solid var(--border-strong);
    border-radius: var(--r-lg);
    background: var(--surface);
  }
  .codes {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(12rem, 1fr));
    gap: var(--s-2) var(--s-5);
    margin: 0;
    padding-left: var(--s-5);
    font-family: var(--mono);
    font-size: var(--fs-sm);
  }
  .actions {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--s-4);
  }
  .steps {
    display: grid;
    gap: var(--s-4);
    margin: 0;
    padding-left: var(--s-5);
  }
  .steps li {
    display: grid;
    justify-items: start;
    gap: var(--s-2);
  }
  /* The code stays black on white in dark mode too (see internal/qr), so
     it gets the border of a field to sit on the dark page. */
  .qr {
    width: 200px;
    height: 200px;
    border: 1px solid var(--border);
    border-radius: var(--r-sm);
    image-rendering: pixelated;
  }
  .secret {
    font-family: var(--mono);
    font-size: var(--fs-md);
    letter-spacing: 0.04em;
    overflow-wrap: anywhere;
    user-select: all;
  }
</style>
