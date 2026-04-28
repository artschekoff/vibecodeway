import { useState, useEffect, useRef } from "react";
import { useNavigate, useLocation } from "react-router-dom";

export default function HamburgerMenu() {
  const [open, setOpen] = useState(false);
  const navigate = useNavigate();
  const location = useLocation();
  const menuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    };
    if (open) document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, [open]);

  // Close on navigation
  useEffect(() => {
    setOpen(false);
  }, [location.pathname]);

  const go = (path: string) => {
    navigate(path);
    setOpen(false);
  };

  return (
    <div className="app-menu" ref={menuRef}>
      <button
        className="app-menu-btn"
        onClick={() => setOpen((v) => !v)}
        aria-label="Menu"
        aria-expanded={open}
      >
        <span className="app-menu-icon" />
        <span className="app-menu-icon" />
        <span className="app-menu-icon" />
      </button>

      {open && (
        <div className="app-menu-dropdown">
          <button className="app-menu-item" onClick={() => go("/")}>
            Home
          </button>
          <div className="app-menu-divider" />
          <button className="app-menu-item" onClick={() => go("/settings")}>
            Settings
          </button>
        </div>
      )}
    </div>
  );
}
