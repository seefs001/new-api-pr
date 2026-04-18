export function renderNumber(num) {
  if (num >= 1000000000) {
    return (num / 1000000000).toFixed(1) + 'B';
  } else if (num >= 1000000) {
    return (num / 1000000).toFixed(1) + 'M';
  } else if (num >= 10000) {
    return (num / 1000).toFixed(1) + 'k';
  } else {
    return num;
  }
}

export function renderQuotaNumberWithDigit(num, digits = 2) {
  if (typeof num !== 'number' || isNaN(num)) {
    return 0;
  }
  const quotaDisplayType = localStorage.getItem('quota_display_type') || 'USD';
  num = num.toFixed(digits);
  if (quotaDisplayType === 'CNY') {
    return '¥' + num;
  } else if (quotaDisplayType === 'USD') {
    return '$' + num;
  } else if (quotaDisplayType === 'CUSTOM') {
    const statusStr = localStorage.getItem('status');
    let symbol = '¤';
    try {
      if (statusStr) {
        const s = JSON.parse(statusStr);
        symbol = s?.custom_currency_symbol || symbol;
      }
    } catch (e) {}
    return symbol + num;
  } else {
    return num;
  }
}

export function renderNumberWithPoint(num) {
  if (num === undefined) return '';
  num = num.toFixed(2);
  if (num >= 100000) {
    const numStr = num.toString();
    const decimalPointIndex = numStr.indexOf('.');

    let wholePart = numStr;
    let decimalPart = '';

    if (decimalPointIndex !== -1) {
      wholePart = numStr.slice(0, decimalPointIndex);
      decimalPart = numStr.slice(decimalPointIndex);
    }

    const shortenedWholePart =
      wholePart.slice(0, 2) + '..' + wholePart.slice(-2);
    return shortenedWholePart + decimalPart;
  }

  return num;
}

export function getQuotaPerUnit() {
  let quotaPerUnit = localStorage.getItem('quota_per_unit');
  quotaPerUnit = parseFloat(quotaPerUnit);
  return quotaPerUnit;
}

export function renderUnitWithQuota(quota) {
  let quotaPerUnit = localStorage.getItem('quota_per_unit');
  quotaPerUnit = parseFloat(quotaPerUnit);
  quota = parseFloat(quota);
  return quotaPerUnit * quota;
}

export function getQuotaWithUnit(quota, digits = 6) {
  let quotaPerUnit = localStorage.getItem('quota_per_unit');
  quotaPerUnit = parseFloat(quotaPerUnit);
  return (quota / quotaPerUnit).toFixed(digits);
}

export function renderQuotaWithAmount(amount) {
  const quotaDisplayType = localStorage.getItem('quota_display_type') || 'USD';
  if (quotaDisplayType === 'TOKENS') {
    return renderNumber(renderUnitWithQuota(amount));
  }

  const numericAmount = Number(amount);
  const formattedAmount = Number.isFinite(numericAmount)
    ? numericAmount.toFixed(2)
    : amount;

  if (quotaDisplayType === 'CNY') {
    return '¥' + formattedAmount;
  } else if (quotaDisplayType === 'CUSTOM') {
    const statusStr = localStorage.getItem('status');
    let symbol = '¤';
    try {
      if (statusStr) {
        const s = JSON.parse(statusStr);
        symbol = s?.custom_currency_symbol || symbol;
      }
    } catch (e) {}
    return symbol + formattedAmount;
  }
  return '$' + formattedAmount;
}

export function getCurrencyConfig() {
  const quotaDisplayType = localStorage.getItem('quota_display_type') || 'USD';
  const statusStr = localStorage.getItem('status');

  let symbol = '$';
  let rate = 1;

  if (quotaDisplayType === 'CNY') {
    symbol = '¥';
    try {
      if (statusStr) {
        const s = JSON.parse(statusStr);
        rate = s?.usd_exchange_rate || 7;
      }
    } catch (e) {}
  } else if (quotaDisplayType === 'CUSTOM') {
    try {
      if (statusStr) {
        const s = JSON.parse(statusStr);
        symbol = s?.custom_currency_symbol || '¤';
        rate = s?.custom_currency_exchange_rate || 1;
      }
    } catch (e) {}
  }

  return { symbol, rate, type: quotaDisplayType };
}

export function convertUSDToCurrency(usdAmount, digits = 2) {
  const { symbol, rate } = getCurrencyConfig();
  const convertedAmount = usdAmount * rate;
  return symbol + convertedAmount.toFixed(digits);
}

export function renderQuota(quota, digits = 2) {
  let quotaPerUnit = localStorage.getItem('quota_per_unit');
  const quotaDisplayType = localStorage.getItem('quota_display_type') || 'USD';
  quotaPerUnit = parseFloat(quotaPerUnit);
  if (quotaDisplayType === 'TOKENS') {
    return renderNumber(quota);
  }
  const resultUSD = quota / quotaPerUnit;
  let symbol = '$';
  let value = resultUSD;
  if (quotaDisplayType === 'CNY') {
    const statusStr = localStorage.getItem('status');
    let usdRate = 1;
    try {
      if (statusStr) {
        const s = JSON.parse(statusStr);
        usdRate = s?.usd_exchange_rate || 1;
      }
    } catch (e) {}
    value = resultUSD * usdRate;
    symbol = '¥';
  } else if (quotaDisplayType === 'CUSTOM') {
    const statusStr = localStorage.getItem('status');
    let symbolCustom = '¤';
    let rate = 1;
    try {
      if (statusStr) {
        const s = JSON.parse(statusStr);
        symbolCustom = s?.custom_currency_symbol || symbolCustom;
        rate = s?.custom_currency_exchange_rate || rate;
      }
    } catch (e) {}
    value = resultUSD * rate;
    symbol = symbolCustom;
  }
  const fixedResult = value.toFixed(digits);
  if (parseFloat(fixedResult) === 0 && quota > 0 && value > 0) {
    const minValue = Math.pow(10, -digits);
    return symbol + minValue.toFixed(digits);
  }
  return symbol + fixedResult;
}
