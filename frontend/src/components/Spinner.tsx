interface SpinnerProps {
  size?: number;
  color?: string;
}

export default function Spinner({ size = 14, color = "currentColor" }: SpinnerProps) {
  const r = (size - 2) / 2;
  const cx = size / 2;
  const circumference = 2 * Math.PI * r;

  return (
    <svg
      width={size}
      height={size}
      viewBox={`0 0 ${size} ${size}`}
      style={{ flexShrink: 0, verticalAlign: "middle" }}
    >
      <circle
        cx={cx}
        cy={cx}
        r={r}
        fill="none"
        stroke={color}
        strokeWidth="2"
        strokeOpacity="0.3"
      />
      <circle
        cx={cx}
        cy={cx}
        r={r}
        fill="none"
        stroke={color}
        strokeWidth="2"
        strokeDasharray={`${circumference * 0.25} ${circumference * 0.75}`}
        strokeLinecap="round"
      >
        <animateTransform
          attributeName="transform"
          type="rotate"
          from={`0 ${cx} ${cx}`}
          to={`360 ${cx} ${cx}`}
          dur="0.7s"
          repeatCount="indefinite"
        />
      </circle>
    </svg>
  );
}
