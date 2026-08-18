import { useState, useEffect, useRef, useCallback } from "react";
import { Plus, Pencil, Trash2 } from "lucide-react";
import { useTabContext } from "../contexts/TabContext";
import OrgFormModal from "./OrgFormModal";
import {
  ListOrganizations,
  DeleteOrganization,
} from "../../wailsjs/go/main/App";
import type { Organization } from "../types";

export default function HamburgerMenu() {
  const [open, setOpen] = useState(false);
  const [orgs, setOrgs] = useState<Organization[]>([]);
  const [showNewOrg, setShowNewOrg] = useState(false);
  const [editOrg, setEditOrg] = useState<Organization | null>(null);
  const [confirmDeleteOrgId, setConfirmDeleteOrgId] = useState<string | null>(null);
  const confirmTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const menuRef = useRef<HTMLDivElement>(null);
  const { openOrgTab, openSettingsTab, openLogsTab, openPromptsTab, openBoardTab, openAskTab, openTimelineTab, openActivityFeedTab, openProductivityTab, tabs, closeTab } = useTabContext();

  const loadOrgs = useCallback(async () => {
    try {
      const list = await ListOrganizations();
      setOrgs(list ?? []);
    } catch {
      /* ignore */
    }
  }, []);

  useEffect(() => {
    void loadOrgs();
  }, [loadOrgs]);

  useEffect(() => {
    if (!open) return;
    void loadOrgs();
  }, [open, loadOrgs]);

  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) setOpen(false);
    };
    if (open) document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, [open]);

  const handleOrgClick = (org: Organization) => {
    openOrgTab(org.id, org.name);
    setOpen(false);
  };

  const requestDeleteOrg = (orgId: string) => {
    if (confirmTimerRef.current) clearTimeout(confirmTimerRef.current);
    setConfirmDeleteOrgId(orgId);
    confirmTimerRef.current = setTimeout(() => setConfirmDeleteOrgId(null), 3000);
  };

  const handleDelete = async (org: Organization) => {
    setConfirmDeleteOrgId(null);
    if (confirmTimerRef.current) clearTimeout(confirmTimerRef.current);
    try {
      await DeleteOrganization(org.id);
      const openTab = tabs.find((t) => t.type === "org" && t.orgId === org.id);
      if (openTab) closeTab(openTab.id);
      await loadOrgs();
    } catch {
      /* ignore */
    }
  };

  return (
    <>
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
            <button
              className="app-menu-item"
              onClick={() => { openBoardTab(); setOpen(false); }}
            >
              My Board
            </button>
            <button
              className="app-menu-item"
              onClick={() => { openAskTab(); setOpen(false); }}
            >
              Ask AI
            </button>
            <button
              className="app-menu-item"
              onClick={() => { openTimelineTab(); setOpen(false); }}
            >
              Activity Timeline
            </button>
            <button
              className="app-menu-item"
              onClick={() => { openActivityFeedTab(); setOpen(false); }}
            >
              Activity Feed
            </button>
            <button
              className="app-menu-item"
              onClick={() => { openProductivityTab(); setOpen(false); }}
            >
              Closed Tasks
            </button>
            <div className="app-menu-divider" />

            {/* Organizations section */}
            <div className="app-menu-section-header">
              <span>Organizations</span>
              <button
                onClick={() => { setShowNewOrg(true); setOpen(false); }}
                title="New organization"
              >
                <Plus size={12} />
              </button>
            </div>

            {orgs.length === 0 && (
              <div style={{ padding: "4px 12px", fontSize: 12, color: "var(--text-muted)" }}>
                No organizations yet
              </div>
            )}

            {orgs.map((org) => (
              <div
                key={org.id}
                className="app-menu-org-item"
                onClick={() => handleOrgClick(org)}
              >
                <span className="app-menu-org-item__name">{org.name}</span>
                <div
                  style={{ display: "flex", gap: 2 }}
                  onClick={(e) => e.stopPropagation()}
                >
                  <button
                    style={{ background: "none", border: "none", cursor: "pointer", color: "var(--text-muted)", display: "flex", alignItems: "center", padding: "2px" }}
                    title="Rename"
                    onClick={() => { setEditOrg(org); setOpen(false); }}
                  >
                    <Pencil size={12} />
                  </button>
                  <button
                    style={{
                      background: "none", border: "none", cursor: "pointer",
                      color: confirmDeleteOrgId === org.id ? "var(--danger)" : "var(--text-muted)",
                      display: "flex", alignItems: "center", padding: "2px",
                      fontSize: confirmDeleteOrgId === org.id ? 10 : undefined,
                    }}
                    title={confirmDeleteOrgId === org.id ? "Confirm delete" : "Delete"}
                    onClick={() => {
                      if (confirmDeleteOrgId === org.id) void handleDelete(org);
                      else requestDeleteOrg(org.id);
                    }}
                  >
                    {confirmDeleteOrgId === org.id ? "Confirm" : <Trash2 size={12} />}
                  </button>
                </div>
              </div>
            ))}

            <div className="app-menu-divider" />

            <button
              className="app-menu-item"
              onClick={() => { openSettingsTab(); setOpen(false); }}
            >
              Settings
            </button>
            <button
              className="app-menu-item app-menu-item--sub"
              onClick={() => { openSettingsTab(); setOpen(false); }}
            >
              Hotkeys
            </button>
            <button
              className="app-menu-item"
              onClick={() => { openLogsTab(); setOpen(false); }}
            >
              App Logs
            </button>
            <button
              className="app-menu-item"
              onClick={() => { openPromptsTab(); setOpen(false); }}
            >
              AI Prompts
            </button>
          </div>
        )}
      </div>

      {showNewOrg && (
        <OrgFormModal
          onSave={(saved) => {
            setOrgs((prev) => [...prev, saved]);
            setShowNewOrg(false);
          }}
          onClose={() => setShowNewOrg(false)}
        />
      )}

      {editOrg && (
        <OrgFormModal
          org={editOrg}
          onSave={(saved) => {
            setOrgs((prev) => prev.map((o) => (o.id === saved.id ? saved : o)));
            setEditOrg(null);
          }}
          onClose={() => setEditOrg(null)}
        />
      )}
    </>
  );
}
