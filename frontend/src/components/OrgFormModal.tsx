import { useEffect, useRef, useState, type FormEvent } from "react";
import { X } from "lucide-react";
import {
  CreateOrganization,
  UpdateOrganization,
} from "../../wailsjs/go/main/App";
import type { Organization } from "../types";

interface Props {
  /** When provided, the modal is in edit (rename) mode. */
  org?: Organization;
  onSave: (org: Organization) => void;
  onClose: () => void;
}

export default function OrgFormModal({ org, onSave, onClose }: Props) {
  const [name, setName] = useState(org?.name ?? "");
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    inputRef.current?.focus();
  }, []);

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    const trimmed = name.trim();
    if (!trimmed) {
      setError("Name is required.");
      return;
    }
    setSaving(true);
    setError("");
    try {
      let saved: Organization;
      if (org) {
        saved = (await UpdateOrganization(org.id, trimmed)) as Organization;
      } else {
        saved = (await CreateOrganization(trimmed)) as Organization;
      }
      onSave(saved);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
      setSaving(false);
    }
  };

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-card" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h2 className="modal-title">{org ? "Rename Organization" : "New Organization"}</h2>
          <button className="modal-close" onClick={onClose}><X size={16} /></button>
        </div>
        <form onSubmit={(e) => void handleSubmit(e)}>
          <div className="form-field">
            <label className="form-label">Name</label>
            <input
              ref={inputRef}
              className="form-input"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g. Jiro Health"
              maxLength={80}
              disabled={saving}
            />
            {error && <div className="field-error">{error}</div>}
          </div>
          <div className="form-actions">
            <button type="button" className="btn-ghost" onClick={onClose} disabled={saving}>
              Cancel
            </button>
            <button type="submit" className="btn-primary" disabled={saving}>
              {saving ? "Saving…" : org ? "Save" : "Create"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
