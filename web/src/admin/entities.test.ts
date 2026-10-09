import { describe, expect, it } from 'vitest';
import { MAX_MATCHES, matchEntities } from './entities';

const entity = (id: string, name: string) => ({ id, name, state: 'on' });

describe('matchEntities', () => {
  const all = [
    entity('sensor.kitchen_temperature', 'Kitchen temperature'),
    entity('sensor.t1', 'Temperature hall'),
    entity('binary_sensor.kitchen_door', 'Back door'),
  ];

  it('needs every word and puts name matches first', () => {
    expect(matchEntities(all, 'temp', []).map((e) => e.id)).toEqual(['sensor.kitchen_temperature', 'sensor.t1']);
    expect(matchEntities(all, 'kitchen door', []).map((e) => e.id)).toEqual(['binary_sensor.kitchen_door']);
    expect(matchEntities(all, 'KITCHEN', []).map((e) => e.id)).toEqual([
      'sensor.kitchen_temperature',
      'binary_sensor.kitchen_door',
    ]);
  });

  it('leaves out chosen entities and an empty query', () => {
    expect(matchEntities(all, 'temp', ['sensor.t1']).map((e) => e.id)).toEqual(['sensor.kitchen_temperature']);
    expect(matchEntities(all, '  ', [])).toEqual([]);
  });

  it('lists a bounded number', () => {
    const many = Array.from({ length: 100 }, (_, i) => entity(`light.l${i}`, `Light ${i}`));
    expect(matchEntities(many, 'light', [])).toHaveLength(MAX_MATCHES);
  });
});
