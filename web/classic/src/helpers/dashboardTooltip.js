/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

const toFiniteNumber = (value) => {
  const number = Number(value);
  return Number.isFinite(number) ? number : 0;
};

const hasUsableNumber = (value) =>
  value !== undefined &&
  value !== null &&
  value !== '' &&
  Number.isFinite(Number(value));

export const createDashboardDimensionTooltipUpdater = ({
  totalLabel,
  formatValue,
  getTotalValue,
}) => {
  const formatter =
    typeof formatValue === 'function' ? formatValue : (value) => value;

  return (array = []) => {
    const rows = (Array.isArray(array) ? array : [])
      .map((item) => ({
        ...item,
        value: toFiniteNumber(item?.value),
      }))
      .sort((a, b) => toFiniteNumber(b.value) - toFiniteNumber(a.value));

    const explicitTotal =
      typeof getTotalValue === 'function'
        ? rows.map((item) => getTotalValue(item)).find(hasUsableNumber)
        : undefined;
    const total = hasUsableNumber(explicitTotal)
      ? toFiniteNumber(explicitTotal)
      : rows.reduce((sum, item) => sum + toFiniteNumber(item.value), 0);

    return [
      {
        key: totalLabel,
        value: formatter(total),
      },
      ...rows.map((item) => ({
        ...item,
        value: formatter(toFiniteNumber(item.value)),
      })),
    ];
  };
};
