import test from 'node:test';
import assert from 'node:assert/strict';

import { createDashboardDimensionTooltipUpdater } from './dashboardTooltip.js';

test('dimension tooltip updater formats each hover from fresh immutable items', () => {
  const updateContent = createDashboardDimensionTooltipUpdater({
    totalLabel: '总计',
    formatValue: (value) => `$${value}`,
    getTotalValue: (item) => item.datum?.TimeSum,
  });

  const firstHover = [
    { key: 'model-a', value: 2, datum: { TimeSum: 5 } },
    { key: 'model-b', value: 3, datum: { TimeSum: 5 } },
  ];
  const secondHover = [
    { key: 'model-a', value: 7, datum: { TimeSum: 11 } },
    { key: 'model-b', value: 4, datum: { TimeSum: 11 } },
  ];

  const firstResult = updateContent(firstHover);
  const secondResult = updateContent(secondHover);

  assert.deepEqual(
    firstHover.map((item) => item.value),
    [2, 3],
  );
  assert.deepEqual(
    secondHover.map((item) => item.value),
    [7, 4],
  );
  assert.notEqual(firstResult, firstHover);
  assert.notEqual(firstResult[1], firstHover[1]);
  assert.deepEqual(
    firstResult.map((item) => [item.key, item.value]),
    [
      ['总计', '$5'],
      ['model-b', '$3'],
      ['model-a', '$2'],
    ],
  );
  assert.deepEqual(
    secondResult.map((item) => [item.key, item.value]),
    [
      ['总计', '$11'],
      ['model-a', '$7'],
      ['model-b', '$4'],
    ],
  );
});

test('dimension tooltip updater sums item values when no explicit total exists', () => {
  const updateContent = createDashboardDimensionTooltipUpdater({
    totalLabel: '总计',
    formatValue: (value) => `${value}次`,
  });

  const result = updateContent([
    { key: 'model-a', value: 2 },
    { key: 'model-b', value: 8 },
  ]);

  assert.deepEqual(
    result.map((item) => [item.key, item.value]),
    [
      ['总计', '10次'],
      ['model-b', '8次'],
      ['model-a', '2次'],
    ],
  );
});
