type Props = {
  active: boolean;
  onToggle: () => void;
  title?: string;
};

// Presentational status switch. Styling comes from the shared
// .active-toggle / .active-indicator classes in style.css.
export default function StatusToggle({ active, onToggle, title }: Props) {
  return (
    <label className="active-toggle" title={title}>
      <input type="checkbox" checked={active} onChange={() => onToggle()} />
      <span className={`active-indicator${active ? " active-indicator--on" : ""}`} />
    </label>
  );
}
