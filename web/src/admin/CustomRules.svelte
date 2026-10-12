<script lang="ts">
  import {
    MAX_RULES,
    OPERATORS,
    TONES,
    buildCondition,
    parseCondition,
    type Condition,
    type Rule,
    type Subject,
  } from './customWidget';

  // A widget's rules as sentences: "When Outside is below 0, colour Bad and
  // say Frost". The condition is written for the server in its expression
  // language; one that the builder cannot show stays in Advanced.
  let {
    rules,
    mode,
    subjects,
    onchange,
    oncheck,
  }: {
    rules: Rule[];
    mode: string;
    /** What a condition can test: the value, each item's columns, the named values. */
    subjects: Subject[];
    onchange: (rules: Rule[]) => void;
    /** Evaluates a condition against the sample or a fresh answer. */
    oncheck: (when: string) => Promise<string>;
  } = $props();

  // The builder's choices per rule while they are incomplete, such as a
  // comparison without its number yet; a complete one is in the rule.
  let drafts = $state<Record<number, Condition>>({});
  let checks = $state<Record<number, string>>({});

  // A new rule tests the first thing it can: is it on, for a badge, or is
  // it above a number.
  const fresh = (): Condition => ({ subject: subjects[0]?.path ?? '', op: mode === 'badges' ? 'on' : 'above', operand: '' });

  function conditionOf(index: number): Condition | null {
    const rule = rules[index]!;
    if (drafts[index]) return drafts[index]!;
    if (rule.when === '') return fresh();
    return parseCondition(rule.when);
  }

  // A choice that is already complete, such as "Anna is on", counts at once.
  function add() {
    const condition = fresh();
    drafts = { ...drafts, [rules.length]: condition };
    onchange([...rules, { when: buildCondition(condition), tone: 'warn' }]);
  }

  function update(index: number, change: Partial<Rule>) {
    const next = rules.map((r, i) => (i === index ? { ...r, ...change } : r));
    // Unset parts are left out, as the server stores them.
    const rule = next[index]!;
    if (!rule.tone) delete rule.tone;
    if (!rule.message) delete rule.message;
    if (!rule.hide) delete rule.hide;
    if (!rule.announce) delete rule.announce;
    onchange(next);
  }

  function choose(index: number, change: Partial<Condition>) {
    const condition = { ...conditionOf(index)!, ...change };
    drafts = { ...drafts, [index]: condition };
    update(index, { when: buildCondition(condition) });
  }

  // Writing the condition by hand: the builder reads it again.
  function write(index: number, when: string) {
    const { [index]: _, ...rest } = drafts;
    drafts = rest;
    update(index, { when });
  }

  function remove(index: number) {
    const shifted: Record<number, Condition> = {};
    for (const [key, draft] of Object.entries(drafts)) {
      const i = Number(key);
      if (i !== index) shifted[i > index ? i - 1 : i] = draft;
    }
    drafts = shifted;
    checks = {};
    onchange(rules.filter((_, j) => j !== index));
  }

  async function check(index: number) {
    checks = { ...checks, [index]: 'Checking…' };
    checks = { ...checks, [index]: await oncheck(rules[index]!.when) };
  }

  const operandOf = (op: string) => OPERATORS.find((o) => o.op === op)?.operand ?? 'none';
</script>

<fieldset class="rules">
  <legend>Rules <span class="muted">(optional)</span></legend>
  <p class="small muted">
    Colour the data, show a message or hide it while something holds. The most serious colour of the matching rules
    wins; the first matching message is shown.
  </p>
  {#each rules as rule, i (i)}
    {@const condition = conditionOf(i)}
    <div class="rule">
      {#if condition}
        <div class="when">
          <label>
            When
            <select value={condition.subject} onchange={(e) => choose(i, { subject: (e.target as HTMLSelectElement).value })}>
              {#if !subjects.some((s) => s.path === condition.subject)}
                <option value={condition.subject}>{condition.subject ? '(custom path)' : 'Choose…'}</option>
              {/if}
              {#each subjects as s (s.path)}<option value={s.path}>{s.label}</option>{/each}
            </select>
          </label>
          <div class="test">
            <select aria-label="Test of rule {i + 1}" value={condition.op} onchange={(e) => choose(i, { op: (e.target as HTMLSelectElement).value as Condition['op'] })}>
              {#each OPERATORS as o (o.op)}<option value={o.op}>{o.label}</option>{/each}
            </select>
            {#if operandOf(condition.op) !== 'none'}
              <input
                aria-label="Compared with, rule {i + 1}"
                value={condition.operand}
                inputmode={operandOf(condition.op) === 'number' ? 'decimal' : 'text'}
                placeholder={operandOf(condition.op) === 'number' ? '25' : 'text'}
                maxlength="100"
                oninput={(e) => choose(i, { operand: (e.target as HTMLInputElement).value })}
              />
            {/if}
          </div>
        </div>
      {:else}
        <p class="small">When a written condition holds (see Advanced).</p>
      {/if}
      <div class="then">
        <label>
          Colour
          <select value={rule.tone ?? ''} onchange={(e) => update(i, { tone: (e.target as HTMLSelectElement).value })}>
            {#each TONES as t (t.tone)}<option value={t.tone}>{t.label}</option>{/each}
          </select>
        </label>
        <label class="grow-field">
          Message
          <input value={rule.message ?? ''} maxlength="120" oninput={(e) => update(i, { message: (e.target as HTMLInputElement).value })} />
        </label>
      </div>
      <label class="check">
        <input type="checkbox" checked={rule.hide ?? false} onchange={(e) => update(i, { hide: (e.target as HTMLInputElement).checked })} />
        {mode === 'list' ? 'Hide the item' : 'Hide the data'}
      </label>
      <label class="check">
        <input type="checkbox" checked={rule.announce ?? false} onchange={(e) => update(i, { announce: (e.target as HTMLInputElement).checked })} />
        Announce the message on every display when this starts
      </label>
      <details open={!condition}>
        <summary>Advanced: write the condition</summary>
        <div class="advanced">
          <input
            aria-label="Condition of rule {i + 1}"
            class="mono"
            value={rule.when}
            maxlength="500"
            spellcheck="false"
            autocomplete="off"
            oninput={(e) => write(i, (e.target as HTMLInputElement).value)}
          />
          <p class="small muted">In the expression language of the guide, such as <code>value &gt; 25 and value &lt; 30</code>.</p>
          <button type="button" class="link" disabled={rule.when === ''} onclick={() => check(i)}>Check against the answer</button>
          {#if checks[i]}<p class="small mono">{checks[i]}</p>{/if}
        </div>
      </details>
      <button type="button" class="link danger" onclick={() => remove(i)}>Remove rule</button>
    </div>
  {/each}
  {#if rules.length < MAX_RULES}
    <button type="button" class="link" onclick={add}>Add a rule</button>
  {/if}
</fieldset>

<style>
  .rules {
    display: grid;
    gap: var(--s-3);
    margin: 0;
    padding: 0;
    border: 0;
    min-width: 0;
  }
  legend {
    padding: 0;
    margin-bottom: var(--s-2);
    font-weight: 550;
  }
  .rule {
    display: grid;
    gap: var(--s-2);
    padding: var(--s-3) 0;
    border-top: 1px solid var(--border);
    min-width: 0;
  }
  .when,
  .then {
    display: flex;
    flex-wrap: wrap;
    align-items: end;
    gap: var(--s-2);
  }
  /* What is tested on one line, how it is tested on the next. */
  .when > label,
  .test {
    flex: 1 1 100%;
  }
  .test {
    display: flex;
    gap: var(--s-2);
  }
  .test select {
    flex: none;
    width: auto;
  }
  .test input {
    flex: 1 1 4rem;
  }
  .grow-field {
    flex: 1 1 10rem;
  }
  .check {
    display: flex;
    align-items: center;
    gap: 6px;
    font-weight: 400;
  }
  summary {
    cursor: pointer;
    color: var(--muted);
    font-size: var(--fs-sm);
    min-height: 32px;
    display: flex;
    align-items: center;
    gap: 6px;
  }
  /* A flex summary loses the browser's triangle, so it draws its own. */
  summary::before {
    content: '▸';
  }
  details[open] > summary::before {
    content: '▾';
  }
  .advanced {
    display: grid;
    gap: var(--s-2);
    padding-top: var(--s-1);
  }
  .mono {
    font-family: var(--mono);
    font-size: var(--fs-sm);
  }
  code {
    font-family: var(--mono);
  }
  .danger {
    justify-self: start;
  }
</style>
