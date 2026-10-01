export function isInvoiceMonth(value) {
  return typeof value === 'string' && /^(?!0000)\d{4}-(?:0[1-9]|1[0-2])$/.test(value);
}

// Accept the calendar dates and timestamp strings returned by the supported databases.
// Extract the written month without converting through the browser's time zone.
export function getInvoiceMonth(value) {
  if (isInvoiceMonth(value)) return value;
  if (typeof value !== 'string') return null;
  const match =
    /^(\d{4})-(\d{2})-(\d{2})(?:[T ](?:[01]\d|2[0-3]):[0-5]\d:[0-5]\d(?:\.\d{1,9})?(?:Z|[+-](?:[01]\d|2[0-3]):[0-5]\d)?)?$/.exec(value);
  if (!match) return null;
  const [, year, month, day] = match;
  const invoiceMonth = `${year}-${month}`;
  if (!isInvoiceMonth(invoiceMonth)) return null;
  const leap = Number(year) % 4 === 0 && (Number(year) % 100 !== 0 || Number(year) % 400 === 0);
  const monthDays = [31, leap ? 29 : 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31];
  if (Number(day) < 1 || Number(day) > monthDays[Number(month) - 1]) return null;
  return invoiceMonth;
}
