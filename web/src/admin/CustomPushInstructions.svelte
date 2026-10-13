<script lang="ts">
  import type { CustomSource } from './api';
  import { TOKEN_PLACEHOLDER, describeInterval, describePush, pushExamples, pushURL } from './customSources';
  import CopyText from './CopyText.svelte';
  import PushToken from './PushToken.svelte';

  // How a household's system sends the data of one source to GDash: the
  // address, the token and examples to copy.
  let {
    source,
    lastPushAt,
    onclose,
    onmanagetokens,
    onerror,
  }: {
    source: CustomSource;
    lastPushAt: string | undefined;
    onclose: () => void;
    onmanagetokens: () => void;
    onerror: (err: unknown) => string;
  } = $props();

  let token = $state(TOKEN_PLACEHOLDER);
  const url = $derived(pushURL(location.origin, source.id));
  const examples = $derived(pushExamples(url, token));
</script>

<div class="push">
  <h3>Send data to {source.name}</h3>
  <p class="small muted">
    Let your system, such as a HomeyScript, Node-RED or a Home Assistant automation, send the JSON to this address
    {describeInterval(source.intervalSeconds).toLowerCase()} or more often. GDash keeps the last data it received; when
    none arrives for two intervals and 30 seconds, the screens mark it as old.
  </p>
  <dl class="request">
    <dt>Address</dt>
    <dd><input readonly value={url} aria-label="Address to send to" onfocus={(e) => (e.target as HTMLInputElement).select()} /></dd>
    <dt>Method</dt>
    <dd class="mono">PUT</dd>
    <dt>Headers</dt>
    <dd class="mono">Authorization: Bearer &lt;API token&gt;<br />Content-Type: application/json</dd>
    <dt>Body</dt>
    <dd>Any JSON of at most 1 MB, at most once every 5 seconds.</dd>
    <dt>Status</dt>
    <dd>{describePush(lastPushAt, source.intervalSeconds)}</dd>
  </dl>
  <PushToken name={source.name} ontoken={(t) => (token = t)} {onmanagetokens} {onerror} />
  <CopyText label="curl" text={examples.curl} />
  <CopyText label="HomeyScript (Homey Pro)" text={examples.homeyScript} />
  <p class="small muted">
    On the Homey, make a Flow: When Every 1 minute (Date &amp; time), Then Run a script (HomeyScript) with this script.
    Not every Homey version lets a script read who is home; presence can also come from a Logic variable or a Flow tag.
  </p>
  <CopyText label="Home Assistant" text={examples.homeAssistant} />
  <div><button type="button" class="link" onclick={onclose}>Close</button></div>
</div>

<style>
  .push {
    display: grid;
    gap: var(--s-3);
    min-width: 0;
  }
  .push p {
    margin: 0;
  }
  .request {
    display: grid;
    grid-template-columns: max-content minmax(0, 1fr);
    gap: var(--s-2) var(--s-4);
    margin: 0;
    font-size: var(--fs-sm);
  }
  dt {
    color: var(--muted);
  }
  dd {
    margin: 0;
    min-width: 0;
    overflow-wrap: anywhere;
  }
  dd input {
    width: 100%;
    font-family: var(--mono);
    font-size: var(--fs-sm);
  }
</style>
