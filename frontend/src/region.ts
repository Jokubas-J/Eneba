// Eneba colors a region label green when the key can be activated in the visitor's
// country and pink when it cannot. This project has no country detection, so it
// assumes a visitor in the EU: Global and Europe keys work, others do not.
const regionsUsableInEU = new Set(['global', 'europe'])

export function isUsableInEU(regionSlug: string): boolean {
  return regionsUsableInEU.has(regionSlug)
}
