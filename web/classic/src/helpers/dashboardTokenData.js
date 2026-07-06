const numberValue = (value) => Number(value) || 0;

const tokenLabel = (item, tokenId, unknownLabel) => {
  const explicitName = String(item.token_name || item.auth_name || item.auth_index || '').trim();
  if (explicitName) return explicitName;
  if (tokenId > 0) return `Token #${tokenId}`;
  return unknownLabel;
};

export function processTokenData(data, options = {}) {
  const limit = Math.max(0, Number(options.limit ?? 10) || 0);
  const unknownLabel = options.unknownLabel ?? 'Unknown token';
  const otherLabel = options.otherLabel ?? 'Other';
  const tokenTotals = new Map();

  (Array.isArray(data) ? data : []).forEach((item) => {
    const tokenId = numberValue(item.token_id);
    const key = String(tokenId);
    const prev = tokenTotals.get(key) || {
      Token: tokenLabel(item, tokenId, unknownLabel),
      TokenId: tokenId,
      Tokens: 0,
      Quota: 0,
      Count: 0,
    };

    tokenTotals.set(key, {
      ...prev,
      Token: prev.Token || tokenLabel(item, tokenId, unknownLabel),
      Tokens: prev.Tokens + numberValue(item.token_used),
      Quota: prev.Quota + numberValue(item.quota),
      Count: prev.Count + numberValue(item.count),
    });
  });

  const sorted = Array.from(tokenTotals.values()).sort(
    (a, b) => b.Tokens - a.Tokens,
  );
  const ranked = sorted.slice(0, limit);
  const overflow = sorted.slice(limit);

  if (overflow.length > 0) {
    ranked.push({
      Token: otherLabel,
      TokenId: null,
      Tokens: overflow.reduce((sum, item) => sum + item.Tokens, 0),
      Quota: overflow.reduce((sum, item) => sum + item.Quota, 0),
      Count: overflow.reduce((sum, item) => sum + item.Count, 0),
    });
  }

  return ranked;
}
