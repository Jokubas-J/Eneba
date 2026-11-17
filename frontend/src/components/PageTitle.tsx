// Sets the browser tab title of the current page ("Cart | Eneba assignment"), so tabs
// and screen readers can tell pages apart. React 19 moves a <title> rendered anywhere
// in the tree into the document <head>, and removes it when the page unmounts.
export function PageTitle({ title }: { title?: string }) {
  return <title>{title ? `${title} | Eneba assignment` : 'Eneba assignment: game keys from many sellers'}</title>
}
