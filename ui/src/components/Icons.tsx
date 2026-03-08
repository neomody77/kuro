type IconProps = {
  size?: number
  className?: string
  style?: React.CSSProperties
}

function icon(d: string) {
  return function Icon({ size = 24, className = '', style }: IconProps) {
    return (
      <svg
        width={size}
        height={size}
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth={2}
        strokeLinecap="round"
        strokeLinejoin="round"
        className={className}
        style={style}
      >
        <path d={d} />
      </svg>
    )
  }
}

function multiPathIcon(...paths: string[]) {
  return function Icon({ size = 24, className = '', style }: IconProps) {
    return (
      <svg
        width={size}
        height={size}
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth={2}
        strokeLinecap="round"
        strokeLinejoin="round"
        className={className}
        style={style}
      >
        {paths.map((d, i) => (
          <path key={i} d={d} />
        ))}
      </svg>
    )
  }
}

export const MessageSquare = multiPathIcon(
  'M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z'
)

export const GitBranch = multiPathIcon(
  'M6 3v12',
  'M18 9a3 3 0 1 0 0-6 3 3 0 0 0 0 6z',
  'M6 21a3 3 0 1 0 0-6 3 3 0 0 0 0 6z',
  'M18 9a9 9 0 0 1-9 9'
)

export const Zap = icon('M13 2L3 14h9l-1 8 10-12h-9l1-8z')

export const FileText = multiPathIcon(
  'M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z',
  'M14 2v6h6',
  'M16 13H8',
  'M16 17H8',
  'M10 9H8'
)

export const KeyRound = multiPathIcon(
  'M2 18v3c0 .6.4 1 1 1h4v-3h3v-3h2l1.4-1.4a6.5 6.5 0 1 0-4-4Z',
  'M16.5 7.5a1 1 0 1 0 0-2 1 1 0 0 0 0 2z'
)

export const ScrollText = multiPathIcon(
  'M8 21h12a2 2 0 0 0 2-2v-2H10v2a2 2 0 1 1-4 0V5a2 2 0 1 0-4 0v3h4',
  'M19 17V5a2 2 0 0 0-2-2H4',
  'M15 8h-5',
  'M15 12h-5'
)

export const Settings = multiPathIcon(
  'M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z',
  'M12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6z'
)

export const MoreHorizontal = multiPathIcon(
  'M12 13a1 1 0 1 0 0-2 1 1 0 0 0 0 2z',
  'M19 13a1 1 0 1 0 0-2 1 1 0 0 0 0 2z',
  'M5 13a1 1 0 1 0 0-2 1 1 0 0 0 0 2z'
)

export const Send = icon('M22 2L11 13M22 2l-7 20-4-9-9-4 20-7z')

export const Plus = multiPathIcon('M12 5v14', 'M5 12h14')

export const Search = multiPathIcon(
  'M11 19a8 8 0 1 0 0-16 8 8 0 0 0 0 16z',
  'M21 21l-4.35-4.35'
)

export const Play = icon('M5 3l14 9-14 9V3z')

export const Pause = multiPathIcon('M6 4h4v16H6z', 'M14 4h4v16h-4z')

export const Clock = multiPathIcon(
  'M12 22a10 10 0 1 0 0-20 10 10 0 0 0 0 20z',
  'M12 6v6l4 2'
)

export const CheckCircle = multiPathIcon(
  'M22 11.08V12a10 10 0 1 1-5.93-9.14',
  'M22 4L12 14.01l-3-3'
)

export const XCircle = multiPathIcon(
  'M12 22a10 10 0 1 0 0-20 10 10 0 0 0 0 20z',
  'M15 9l-6 6',
  'M9 9l6 6'
)

export const Folder = icon(
  'M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z'
)

export const File = multiPathIcon(
  'M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z',
  'M14 2v6h6'
)

export const Shield = icon(
  'M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z'
)

export const Eye = multiPathIcon(
  'M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z',
  'M12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6z'
)

export const EyeOff = multiPathIcon(
  'M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94',
  'M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19',
  'M14.12 14.12a3 3 0 1 1-4.24-4.24',
  'M1 1l22 22'
)

export const Trash = multiPathIcon(
  'M3 6h18',
  'M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2'
)

export const Edit2 = icon(
  'M17 3a2.83 2.83 0 1 1 4 4L7.5 20.5 2 22l1.5-5.5L17 3z'
)

export const ChevronRight = icon('M9 18l6-6-6-6')

export const Sun = multiPathIcon(
  'M12 16a4 4 0 1 0 0-8 4 4 0 0 0 0 8z',
  'M12 2v2',
  'M12 20v2',
  'M4.93 4.93l1.41 1.41',
  'M17.66 17.66l1.41 1.41',
  'M2 12h2',
  'M20 12h2',
  'M6.34 17.66l-1.41 1.41',
  'M19.07 4.93l-1.41 1.41'
)

export const Moon = icon(
  'M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z'
)

export const PanelLeftClose = multiPathIcon(
  'M20 4H4a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2V6a2 2 0 0 0-2-2z',
  'M9 4v16',
  'M15 10l-2 2 2 2'
)

export const PanelLeftOpen = multiPathIcon(
  'M20 4H4a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2V6a2 2 0 0 0-2-2z',
  'M9 4v16',
  'M14 10l2 2-2 2'
)

export const Minus = icon('M5 12h14')

export const Maximize2 = multiPathIcon(
  'M8 3H5a2 2 0 0 0-2 2v3',
  'M21 8V5a2 2 0 0 0-2-2h-3',
  'M3 16v3a2 2 0 0 0 2 2h3',
  'M16 21h3a2 2 0 0 0 2-2v-3'
)

export const X = multiPathIcon('M18 6L6 18', 'M6 6l12 12')

export const Monitor = multiPathIcon(
  'M20 3H4a2 2 0 0 0-2 2v10a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2V5a2 2 0 0 0-2-2z',
  'M8 21h8',
  'M12 17v4'
)

export const Bell = multiPathIcon(
  'M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9',
  'M13.73 21a2 2 0 0 1-3.46 0'
)

export const Grid = multiPathIcon(
  'M3 3h7v7H3z',
  'M14 3h7v7h-7z',
  'M14 14h7v7h-7z',
  'M3 14h7v7H3z'
)

export const Layout = multiPathIcon(
  'M19 3H5a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2V5a2 2 0 0 0-2-2z',
  'M3 9h18',
  'M9 21V9'
)
