<script lang="ts">
  import { TOKEN_PLACEHOLDER } from './customSources';
  import { homeyScript, snapshotURL } from './homey';
  import CopyText from './CopyText.svelte';
  import PushToken from './PushToken.svelte';

  // How a Homey without API keys sends its data to GDash: a HomeyScript that
  // a Flow runs every minute.
  let {
    onclose,
    onmanagetokens,
    onerror,
  }: {
    onclose: () => void;
    onmanagetokens: () => void;
    onerror: (err: unknown) => string;
  } = $props();

  let token = $state(TOKEN_PLACEHOLDER);
  const url = snapshotURL(location.origin);
  const script = $derived(homeyScript(url, token));
</script>

<div class="push">
  <h3>Let your Homey send its data</h3>
  <ol class="small">
    <li>On your Homey, install <strong>HomeyScript</strong> from the Homey App Store.</li>
    <li>Make an API token for the script here. GDash fills it into the script below.</li>
    <li>At my.homey.app, open HomeyScript, make a new script, paste the script below and save it.</li>
    <li>Make a Flow: When <strong>Every 1 minute</strong> (Date &amp; time), Then <strong>Run a script</strong> (HomeyScript) with that script.</li>
  </ol>
  <PushToken name="HomeyScript" ontoken={(t) => (token = t)} {onmanagetokens} {onerror} />
  {#if token === TOKEN_PLACEHOLDER}
    <p class="badge warn" role="note">
      The script below still says {TOKEN_PLACEHOLDER}. Make a token above first and copy it after, or put a token
      from API tokens in its place; otherwise GDash answers 401.
    </p>
  {/if}
  <CopyText label="HomeyScript" text={script} />
  <p class="small muted">
    The script sends every device and who is home to <span class="mono">{url}</span>, with PUT and the API token.
    HomeyScript can do anything on your Homey, and the token sits in the script: use a token for this script alone,
    so you can revoke it without touching anything else. Not every Homey version lets a script read who is home; the
    script then sends the devices only.
  </p>
  <div><button type="button" class="link" onclick={onclose}>Close</button></div>
</div>

<style>
  .push {
    display: grid;
    gap: var(--s-3);
    min-width: 0;
  }
  .push p,
  .push ol {
    margin: 0;
  }
  .push ol {
    padding-left: 1.2em;
    display: grid;
    gap: var(--s-1);
  }
  .mono {
    overflow-wrap: anywhere;
  }
</style>
