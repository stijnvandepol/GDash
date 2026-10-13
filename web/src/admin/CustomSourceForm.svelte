<script lang="ts">
  import { api, ApiError, type CustomAuthKind, type CustomSource, type CustomSourceInput } from './api';
  import {
    AUTH_KINDS,
    INTERVALS,
    MAX_HEADERS,
    TOKEN_PLACES,
    emptyInput,
    newLogin,
    parseSample,
    pushInput,
    sampleText,
    secretPlaceholder,
    toInput,
    withLogin,
  } from './customSources';

  let {
    source,
    onsaved,
    oncancel,
    onerror,
  }: {
    /** The source to change, or undefined for a new one. */
    source?: CustomSource;
    onsaved: (saved: CustomSource) => Promise<void>;
    oncancel: () => void;
    onerror: (err: unknown) => string;
  } = $props();

  // The form starts from the source once; later changes to the prop (a
  // reload of the list) must not overwrite what is being typed.
  // svelte-ignore state_referenced_locally
  let form = $state<CustomSourceInput>(source ? toInput(source) : emptyInput());
  // svelte-ignore state_referenced_locally
  let sample = $state(sampleText(source?.sample));
  let fieldErrors = $state<Record<string, string>>({});
  let error = $state('');
  let busy = $state(false);

  const auth = $derived(AUTH_KINDS.find((a) => a.kind === form.auth.kind) ?? AUTH_KINDS[0]!);
  const place = $derived(TOKEN_PLACES.find((p) => p.place === form.auth.login?.place));
  // A header or parameter name: the API key's, or where a login's token goes.
  const nameLabel = $derived(form.auth.kind === 'login' ? place?.nameLabel : auth.nameLabel);
  const namePlaceholder = $derived(
    form.auth.kind === 'login' ? (place?.placeholder ?? '') : form.auth.kind === 'header' ? 'X-API-Key' : form.auth.kind === 'query' ? 'appid' : '',
  );

  function chooseAuth(kind: CustomAuthKind) {
    form.auth.kind = kind;
    if (kind === 'login' && !form.auth.login) {
      form.auth.login = newLogin(form.url);
      form.auth.name ||= 'X-FTL-SID';
    }
  }

  async function save(event: SubmitEvent) {
    event.preventDefault();
    error = '';
    fieldErrors = {};
    const parsed = parseSample(sample);
    if ('error' in parsed) {
      fieldErrors = { sample: parsed.error };
      return;
    }
    busy = true;
    try {
      const input =
        form.mode === 'push'
          ? pushInput({ ...form, sample: parsed.sample })
          : withLogin({ ...form, body: form.method === 'POST' ? form.body : '', sample: parsed.sample });
      const saved = source ? await api.saveCustomSource(source.id, input) : await api.addCustomSource(input);
      await onsaved(saved);
    } catch (err) {
      if (err instanceof ApiError && err.fields.length > 0) {
        fieldErrors = Object.fromEntries(err.fields.map((f) => [f.field, f.message]));
      } else {
        error = onerror(err);
      }
    } finally {
      busy = false;
    }
  }

  function chooseMode(mode: 'fetch' | 'push') {
    form.mode = mode;
    // A script sends every minute; an API is often asked less.
    if (!source) form.intervalSeconds = mode === 'push' ? 60 : 300;
  }

  function addHeader() {
    form.headers = [...form.headers, { name: '', value: '', secret: false }];
  }

  function removeHeader(i: number) {
    form.headers = form.headers.filter((_, j) => j !== i);
  }
</script>

<form onsubmit={save} class="source-form">
  <label>
    Name
    <input bind:value={form.name} maxlength="60" required placeholder="Weather in Oss" />
    {#if fieldErrors.name}<span class="field-error">{fieldErrors.name}</span>{/if}
  </label>
  <fieldset class="mode">
    <legend>How does GDash get the data?</legend>
    <label class="check"><input type="radio" name="mode" checked={form.mode === 'fetch'} onchange={() => chooseMode('fetch')} /> GDash fetches it from an address</label>
    <label class="check"><input type="radio" name="mode" checked={form.mode === 'push'} onchange={() => chooseMode('push')} /> Your system sends it to GDash</label>
    {#if form.mode === 'push'}
      <span class="small muted">For a Homey, Node-RED, a Home Assistant automation or a script that cannot answer requests itself. After saving, GDash shows the address to send to and examples.</span>
    {/if}
    {#if fieldErrors.mode}<span class="field-error">{fieldErrors.mode}</span>{/if}
  </fieldset>
  {#if form.mode === 'push'}
    <label>
      Expected every
      <select bind:value={form.intervalSeconds}>
        {#each INTERVALS as i (i.seconds)}<option value={i.seconds}>{i.label.replace(/^Every /, '').replace('Once a day', 'day')}</option>{/each}
      </select>
      <span class="small muted">When no data arrives for twice this and 30 seconds, the screens mark it as old.</span>
      {#if fieldErrors.intervalSeconds}<span class="field-error">{fieldErrors.intervalSeconds}</span>{/if}
    </label>
  {:else}
    <label>
      Address
      <input bind:value={form.url} type="url" required autocomplete="off" spellcheck="false" placeholder="https://api.open-meteo.com/v1/forecast?latitude=51.77&longitude=5.52&current=temperature_2m" />
      <span class="small muted">Put an API key under Authentication, not in the address: the address is shown here.</span>
      {#if fieldErrors.url}<span class="field-error">{fieldErrors.url}</span>{/if}
    </label>
    <div class="row">
      <label>
        Method
        <select bind:value={form.method}>
          <option value="GET">GET</option>
          <option value="POST">POST</option>
        </select>
        {#if fieldErrors.method}<span class="field-error">{fieldErrors.method}</span>{/if}
      </label>
      <label>
        Refresh
        <select bind:value={form.intervalSeconds}>
          {#each INTERVALS as i (i.seconds)}<option value={i.seconds}>{i.label}</option>{/each}
        </select>
        {#if fieldErrors.intervalSeconds}<span class="field-error">{fieldErrors.intervalSeconds}</span>{/if}
      </label>
    </div>
    {#if form.method === 'POST'}
      <label>
        <span>JSON body <span class="muted">(optional)</span></span>
        <textarea bind:value={form.body} rows="3" spellcheck="false" placeholder={'{"query": "…"}'}></textarea>
        {#if fieldErrors.body}<span class="field-error">{fieldErrors.body}</span>{/if}
      </label>
    {/if}
    <label>
      Authentication
      <select value={form.auth.kind} onchange={(e) => chooseAuth((e.currentTarget as HTMLSelectElement).value as CustomAuthKind)}>
        {#each AUTH_KINDS as a (a.kind)}<option value={a.kind}>{a.label}</option>{/each}
      </select>
      {#if fieldErrors['auth.kind']}<span class="field-error">{fieldErrors['auth.kind']}</span>{/if}
    </label>
    {#if form.auth.kind === 'login' && form.auth.login}
      <p class="small muted">
        For an API that gives a session token for a password, such as Pi-hole 6: GDash posts the body to the login
        address, reads the token from the answer and sends it with every request. It logs in again when the token runs
        out or the API refuses it. The token is kept encrypted in memory only.
      </p>
      <label>
        Login address
        <input bind:value={form.auth.login.url} type="url" required autocomplete="off" spellcheck="false" placeholder="http://pi.hole/api/auth" />
        <span class="small muted">On the same host as the address above.</span>
        {#if fieldErrors['auth.login.url']}<span class="field-error">{fieldErrors['auth.login.url']}</span>{/if}
      </label>
      <label>
        Login body (JSON)
        <textarea bind:value={form.auth.login.body} rows="2" spellcheck="false" placeholder={'{"password": "{secret}"}'}></textarea>
        <span class="small muted">Write {'{secret}'} where the password goes; GDash fills it in when it logs in.</span>
        {#if fieldErrors['auth.login.body']}<span class="field-error">{fieldErrors['auth.login.body']}</span>{/if}
      </label>
      <div class="row">
        <label>
          Token in the answer
          <input bind:value={form.auth.login.tokenPath} required autocomplete="off" spellcheck="false" placeholder="session.sid" />
          {#if fieldErrors['auth.login.tokenPath']}<span class="field-error">{fieldErrors['auth.login.tokenPath']}</span>{/if}
        </label>
        <label>
          <span>Seconds it lasts <span class="muted">(optional)</span></span>
          <input bind:value={form.auth.login.validityPath} autocomplete="off" spellcheck="false" placeholder="session.validity" />
          {#if fieldErrors['auth.login.validityPath']}<span class="field-error">{fieldErrors['auth.login.validityPath']}</span>{/if}
        </label>
      </div>
      <label>
        Send the token
        <select bind:value={form.auth.login.place}>
          {#each TOKEN_PLACES as p (p.place)}<option value={p.place}>{p.label}</option>{/each}
        </select>
        {#if fieldErrors['auth.login.place']}<span class="field-error">{fieldErrors['auth.login.place']}</span>{/if}
      </label>
    {/if}
    {#if fieldErrors['auth.login']}<span class="field-error">{fieldErrors['auth.login']}</span>{/if}
    {#if nameLabel}
      <label>
        {nameLabel}
        <input bind:value={form.auth.name} autocomplete="off" spellcheck="false" placeholder={namePlaceholder} />
        {#if fieldErrors['auth.name']}<span class="field-error">{fieldErrors['auth.name']}</span>{/if}
      </label>
    {/if}
    {#if auth.secretLabel}
      <label>
        {auth.secretLabel}
        <!-- Not type="password", so browsers do not offer to save it. -->
        <input bind:value={form.auth.secret} class="masked" autocomplete="off" spellcheck="false" placeholder={secretPlaceholder(source, form.auth.kind)} />
        <span class="small muted">Stored encrypted and never shown again.</span>
        {#if fieldErrors['auth.secret']}<span class="field-error">{fieldErrors['auth.secret']}</span>{/if}
      </label>
    {/if}

    <fieldset>
      <legend>Extra headers <span class="muted">(optional)</span></legend>
      {#each form.headers as header, i (i)}
        <div class="header-row">
          <input bind:value={header.name} aria-label="Header name" placeholder="Accept-Language" autocomplete="off" spellcheck="false" />
          <input
            bind:value={header.value}
            class:masked={header.secret}
            aria-label="Header value"
            autocomplete="off"
            spellcheck="false"
            placeholder={header.secret && source?.headers.some((h) => h.secret && h.name.toLowerCase() === header.name.toLowerCase()) ? 'Stored; leave empty to keep it' : 'nl'}
          />
          <label class="check"><input type="checkbox" bind:checked={header.secret} /> Secret</label>
          <button type="button" class="link danger" onclick={() => removeHeader(i)}>Remove</button>
        </div>
        {#if fieldErrors[`headers[${i}].name`] || fieldErrors[`headers[${i}].value`]}
          <span class="field-error">{fieldErrors[`headers[${i}].name`] ?? fieldErrors[`headers[${i}].value`]}</span>
        {/if}
      {/each}
      {#if fieldErrors.headers}<span class="field-error">{fieldErrors.headers}</span>{/if}
      {#if form.headers.length < MAX_HEADERS}
        <button type="button" class="link" onclick={addHeader}>Add a header</button>
      {/if}
    </fieldset>
  {/if}

  <label>
    <span>Sample answer <span class="muted">(optional)</span></span>
    <textarea bind:value={sample} rows="5" spellcheck="false" placeholder={'{"current": {"temperature_2m": 12.4}}'}></textarea>
    <span class="small muted">{form.mode === 'push' ? 'Paste what your system will send to design a widget before it sends anything.' : "Paste an answer from the API's documentation to design a widget without calling the API."}</span>
    {#if fieldErrors.sample}<span class="field-error">{fieldErrors.sample}</span>{/if}
  </label>

  <div class="inline">
    <button type="submit" disabled={busy}>{busy ? 'Saving…' : 'Save'}</button>
    <button type="button" class="link" onclick={oncancel}>Cancel</button>
  </div>
  {#if error}<p class="error">{error}</p>{/if}
</form>

<style>
  .row {
    display: flex;
    flex-wrap: wrap;
    gap: var(--s-4);
    max-width: 40rem;
  }
  .row label {
    flex: 1 1 10rem;
  }
  fieldset {
    display: grid;
    gap: var(--s-2);
    margin: 0;
    padding: 0;
    border: 0;
    max-width: 40rem;
  }
  legend {
    padding: 0;
    margin-bottom: var(--s-2);
    font-size: var(--fs-sm);
    font-weight: 550;
    color: var(--text-soft);
  }
  .header-row {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--s-2);
  }
  .header-row input:not([type='checkbox']) {
    flex: 1 1 9rem;
  }
  .check {
    display: flex;
    align-items: center;
    gap: 6px;
    font-weight: 400;
  }
  .masked {
    -webkit-text-security: disc;
  }
</style>
