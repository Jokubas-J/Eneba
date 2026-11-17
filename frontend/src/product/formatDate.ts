// "2015-05-18" -> "18 May 2015". The date is read as UTC so the day cannot shift by
// the visitor's time zone.
export function formatDate(isoDate: string): string {
  return new Date(`${isoDate}T00:00:00Z`).toLocaleDateString('en-IE', {
    day: 'numeric',
    month: 'long',
    year: 'numeric',
    timeZone: 'UTC',
  })
}
