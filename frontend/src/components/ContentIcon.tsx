import {
  BadgePercent,
  CalendarPlus,
  CarFront,
  Compass,
  Crown,
  Earth,
  Euro,
  Gamepad2,
  Globe,
  LayoutGrid,
  MapPin,
  Monitor,
  Plane,
  Puzzle,
  Shield,
  Sparkles,
  Swords,
  Tag,
  TrendingUp,
  Trophy,
  Users,
  type LucideIcon,
} from 'lucide-react'

// Icon keys come from the database (taxonomy_icons, menu_sections, menu_links); this
// map is the only place that turns a key into a drawing. Keys are lucide's own names
// in kebab-case, so adding an icon means one import and one line here.
//
// Why an explicit list rather than lucide's <DynamicIcon name={key}>: DynamicIcon can
// draw any of ~1600 icons, which means the build ships (and the browser may fetch)
// code for all of them. Listing the ones we use keeps only those in the bundle.
const icons: Record<string, LucideIcon> = {
  'badge-percent': BadgePercent,
  'calendar-plus': CalendarPlus,
  'car-front': CarFront,
  compass: Compass,
  crown: Crown,
  earth: Earth,
  euro: Euro,
  'gamepad-2': Gamepad2,
  globe: Globe,
  'layout-grid': LayoutGrid,
  'map-pin': MapPin,
  monitor: Monitor,
  plane: Plane,
  puzzle: Puzzle,
  shield: Shield,
  sparkles: Sparkles,
  swords: Swords,
  tag: Tag,
  'trending-up': TrendingUp,
  trophy: Trophy,
  users: Users,
}

// An unknown or missing key draws this, so a typo or a newly added key in the
// database never leaves a hole or breaks the page.
const DefaultIcon = Gamepad2

export function ContentIcon({ name, className }: { name: string | null | undefined; className?: string }) {
  const Icon = (name && icons[name]) || DefaultIcon
  // Decorative: the text next to every icon already says what it is.
  return <Icon className={className} aria-hidden="true" strokeWidth={1.75} />
}
