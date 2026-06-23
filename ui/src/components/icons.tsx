type IconProps = { className?: string }

/** The GameNest hex mark — matches the website. Stroke uses currentColor;
 *  the emerald core is fixed so the brand color is consistent. */
export function Logo({ className }: IconProps) {
  return (
    <svg viewBox="0 0 32 32" fill="none" className={className ?? "h-7 w-7"} aria-hidden>
      <path
        d="M16 3 27 9.3v13.4L16 29 5 22.7V9.3z"
        stroke="currentColor"
        strokeWidth={1.8}
        opacity={0.9}
      />
      <circle cx="16" cy="16" r="3.6" fill="#34d399" />
      <circle cx="16" cy="16" r="6.4" stroke="#34d399" strokeOpacity={0.4} />
    </svg>
  )
}
