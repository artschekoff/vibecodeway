import { ChevronDown, GitBranch, X } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import DatePicker from "react-datepicker";
import "react-datepicker/dist/react-datepicker.css";
import Spinner from "./Spinner";
import { ListSlackWorkspaces, PickAndExtractFiles, PickAndExtractFilesPreview, PreviewSlackDMs, SearchJQL, SearchSlackChannels, DeleteUploadedFile, DiscardPendingUpload, GetSupportedPlatforms, ListRepoBranches, ListTrackedRepos } from "../../wailsjs/go/main/App";
import type { ContextText, Context, Issue, PendingUploadPreview, SearchResult, SlackChannel, SlackChannelInput, SlackChannelResult, SlackDMConfig, SlackDMConfigInput, SlackSavePayload, SlackWorkspaceResult, TextFilesPayload, UploadBatchResult, UploadPreviewBatchResult, UploadedFile, VCSBranch, VCSPlatform, VCSPlatformInfo, VCSRepo } from "../types";

export interface VCSBranchInput {
  platform: VCSPlatform;
  ownerRepo: string;
  branch: string;
}

interface Props {
  initial?: Context;
  initialJqlQuery?: string;
  initialJiraIssues?: Issue[];
  initialSlackChannels?: SlackChannel[];
  initialSlackDMConfigs?: SlackDMConfig[];
  initialContextTexts?: ContextText[];
  initialUploadedFiles?: UploadedFile[];
  initialVCSBranches?: VCSBranch[];
  onSave: (
    key: string,
    searchQuery: string,
    status: string | undefined,
    slackPayload?: SlackSavePayload,
    selectedJiraKeys?: string[],
    removedJiraIssueIds?: string[],
    textFilesPayload?: TextFilesPayload,
    removedUploadedFileIds?: string[],
    pendingUploadTempIds?: string[],
    newVCSBranches?: VCSBranchInput[],
    removedVCSBranchIds?: string[]
  ) => Promise<void>;
  onCancel: () => void;
}

interface JqlTemplate {
  label: string;
  query: string;
}

const JQL_TEMPLATES: JqlTemplate[] = [
  { label: "Single issue",       query: 'key = PROJ-123' },
  { label: "Multiple issues",    query: 'key in (PROJ-1, PROJ-2, PROJ-3)' },
  { label: "Open sprint",        query: 'project = PROJ AND sprint in openSprints() ORDER BY priority DESC' },
  { label: "My open issues",     query: 'project = PROJ AND assignee = currentUser() AND status != Done ORDER BY updated DESC' },
  { label: "In Progress",        query: 'project = PROJ AND status = "In Progress" ORDER BY updated DESC' },
  { label: "Updated this week",  query: 'project = PROJ AND updated >= -7d ORDER BY updated DESC' },
  { label: "Fix version",        query: 'project = PROJ AND fixVersion = "v1.0" ORDER BY priority DESC' },
  { label: "Unassigned bugs",    query: 'project = PROJ AND issuetype = Bug AND assignee is EMPTY ORDER BY priority DESC' },
];

interface ChannelResultItem extends SlackChannelResult {
  workspace: string;
}

interface SelectedChannelItem extends SlackChannelInput {
  workspace: string;
  startFrom: string;
}

interface DMPreviewResult {
  workspace: string;
  startFrom: string;
  content: string;
}

interface EditableFile extends ContextText {
  dirty: boolean;
}

interface NewFile {
  tempId: string;
  name: string;
  content: string;
}

type ActiveTab = "jira" | "slack" | "texts" | "vcs";

export default function ContextForm({ initial, initialJqlQuery, initialJiraIssues, initialSlackChannels, initialSlackDMConfigs, initialContextTexts, initialUploadedFiles, initialVCSBranches, onSave, onCancel }: Props) {
  const [key, setKey] = useState(initial?.key ?? "");
  const [searchQuery, setSearchQuery] = useState(initialJqlQuery ?? "");
  const [status, setStatus] = useState(initial?.status ?? "active");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [activeTab, setActiveTab] = useState<ActiveTab>("jira");

  const [jqlFocused, setJqlFocused] = useState(false);
  const [jqlResults, setJqlResults] = useState<SearchResult[]>([]);
  const [jqlSearching, setJqlSearching] = useState(false);
  const [jqlSearchError, setJqlSearchError] = useState("");
  const [existingJiraIssues, setExistingJiraIssues] = useState<Issue[]>(initialJiraIssues ?? []);
  const [selectedJiraIssues, setSelectedJiraIssues] = useState<SearchResult[]>([]);

  const [workspaces, setWorkspaces] = useState<SlackWorkspaceResult[]>([]);
  const [workspacesLoading, setWorkspacesLoading] = useState(false);
  const [workspacesError, setWorkspacesError] = useState("");

  const [slackWorkspace, setSlackWorkspace] = useState(initialSlackChannels?.[0]?.workspace ?? "");
  const [slackPattern, setSlackPattern] = useState("");
  const [channelStartFrom, setChannelStartFrom] = useState("");

  const [channelResults, setChannelResults] = useState<ChannelResultItem[]>([]);
  const [existingChannels, setExistingChannels] = useState<SlackChannel[]>(initialSlackChannels ?? []);
  const [selectedChannels, setSelectedChannels] = useState<SelectedChannelItem[]>([]);
  const [searching, setSearching] = useState(false);
  const [channelError, setChannelError] = useState("");

  // DM configs state
  const [existingDMConfigs, setExistingDMConfigs] = useState<SlackDMConfig[]>(initialSlackDMConfigs ?? []);
  const [newDMConfigs, setNewDMConfigs] = useState<SlackDMConfigInput[]>([]);
  const [removedDMConfigIds, setRemovedDMConfigIds] = useState<string[]>([]);
  const [dmWorkspace, setDmWorkspace] = useState("");
  const [dmStartFrom, setDmStartFrom] = useState("");
  const [dmLoading, setDmLoading] = useState(false);
  const [dmLoadError, setDmLoadError] = useState("");
  const [dmPreviewResults, setDmPreviewResults] = useState<DMPreviewResult[]>([]);
  const [selectedDMPreviews, setSelectedDMPreviews] = useState<DMPreviewResult[]>([]);

  // Text files state
  const [existingFiles, setExistingFiles] = useState<EditableFile[]>(
    (initialContextTexts ?? []).map((f) => ({ ...f, dirty: false }))
  );
  const [newFiles, setNewFiles] = useState<NewFile[]>([]);
  const [removedFileIds, setRemovedFileIds] = useState<string[]>([]);

  // Uploaded document files state
  const [uploadedFiles, setUploadedFiles] = useState<UploadedFile[]>(initialUploadedFiles ?? []);
  const [removedUploadedIds, setRemovedUploadedIds] = useState<string[]>([]);
  const [pendingUploads, setPendingUploads] = useState<PendingUploadPreview[]>([]);
  const [uploading, setUploading] = useState(false);
  const [uploadError, setUploadError] = useState("");

  // VCS branches state
  const [existingVCSBranches, setExistingVCSBranches] = useState<VCSBranch[]>(initialVCSBranches ?? []);
  const [newVCSBranches, setNewVCSBranches] = useState<VCSBranchInput[]>([]);
  const [removedVCSBranchIds, setRemovedVCSBranchIds] = useState<string[]>([]);
  const [vcsPlatforms, setVCSPlatforms] = useState<VCSPlatformInfo[]>([]);
  const [vcsPlatform, setVCSPlatform] = useState<VCSPlatform>("github");
  const [vcsOwnerRepo, setVCSOwnerRepo] = useState("");
  const [vcsBranch, setVCSBranch] = useState("");
  const [vcsAddError, setVCSAddError] = useState("");
  // Repo picker
  const [trackedRepos, setTrackedRepos] = useState<VCSRepo[]>([]);
  const [repoPickerOpen, setRepoPickerOpen] = useState(false);
  const [repoFilter, setRepoFilter] = useState("");
  const repoPickerRef = useRef<HTMLDivElement>(null);
  const repoFilterRef = useRef<HTMLInputElement>(null);
  // Branch picker
  const [repoBranches, setRepoBranches] = useState<string[]>([]);
  const [branchPickerOpen, setBranchPickerOpen] = useState(false);
  const [branchPickerLoading, setBranchPickerLoading] = useState(false);
  const [branchPickerError, setBranchPickerError] = useState("");
  const [branchFilter, setBranchFilter] = useState("");
  const branchPickerRef = useRef<HTMLDivElement>(null);
  const branchFilterRef = useRef<HTMLInputElement>(null);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    setWorkspacesLoading(true);
    ListSlackWorkspaces()
      .then((results) => {
        const ws = results ?? [];
        setWorkspaces(ws);
        if (ws.length > 0) {
          setSlackWorkspace((prev) => prev || ws[0].name);
          setDmWorkspace((prev) => prev || ws[0].name);
        }
      })
      .catch((err: unknown) => setWorkspacesError(String(err)))
      .finally(() => setWorkspacesLoading(false));

    GetSupportedPlatforms()
      .then((platforms) => {
        setVCSPlatforms((platforms ?? []) as VCSPlatformInfo[]);
      })
      .catch(() => {});
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const isEdit = !!initial;

  const handleSearchJQL = async () => {
    if (!searchQuery.trim()) {
      setJqlSearchError("Enter a JQL query to search.");
      return;
    }
    setJqlSearching(true);
    setJqlSearchError("");
    setJqlResults([]);
    try {
      const results = await SearchJQL(searchQuery.trim());
      setJqlResults(results ?? []);
      if (!results || results.length === 0) {
        setJqlSearchError("No issues found matching the query.");
      }
    } catch (err) {
      setJqlSearchError(String(err));
    } finally {
      setJqlSearching(false);
    }
  };

  const isJiraSelected = (key: string) => selectedJiraIssues.some((i) => i.key === key);

  const toggleJiraIssue = (issue: SearchResult) => {
    setSelectedJiraIssues((prev) =>
      prev.some((i) => i.key === issue.key)
        ? prev.filter((i) => i.key !== issue.key)
        : [...prev, issue]
    );
  };

  const removeSelectedJiraIssue = (key: string) => {
    setSelectedJiraIssues((prev) => prev.filter((i) => i.key !== key));
  };

  const removeExistingJiraIssue = (id: string) => {
    setExistingJiraIssues((prev) => prev.filter((i) => i.id !== id));
  };

  const toggleAllJiraIssues = () => {
    const unselected = jqlResults.filter((r) => !isJiraSelected(r.key));
    if (unselected.length === 0) {
      // all already selected — deselect all from current results
      const resultKeys = new Set(jqlResults.map((r) => r.key));
      setSelectedJiraIssues((prev) => prev.filter((i) => !resultKeys.has(i.key)));
    } else {
      setSelectedJiraIssues((prev) => {
        const existing = new Map(prev.map((i) => [i.key, i]));
        for (const r of jqlResults) existing.set(r.key, r);
        return [...existing.values()];
      });
    }
  };


  const handleSearchChannels = async () => {
    if (!slackWorkspace.trim() || !slackPattern.trim()) {
      setChannelError("Select a workspace and enter a pattern to search.");
      return;
    }

    setSearching(true);
    setChannelError("");
    setChannelResults([]);

    const alreadyAdded = new Set([
      ...existingChannels.map((ch) => ch.channelId),
      ...selectedChannels.map((ch) => ch.channelId),
    ]);

    try {
      const res = await SearchSlackChannels(slackWorkspace.trim(), slackPattern.trim());
      const results = (res ?? []).map((r) => ({ ...r, workspace: slackWorkspace.trim() }));
      const filtered = results.filter((r) => !alreadyAdded.has(r.id));
      setChannelResults(filtered);

      if (filtered.length === 0 && results.length > 0) {
        setChannelError("All matching channels are already added.");
      } else if (results.length === 0) {
        setChannelError("No channels found matching the given pattern.");
      }
    } catch (err) {
      setChannelError(String(err));
    } finally {
      setSearching(false);
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!key.trim()) {
      setError("Key is required.");
      return;
    }
    setSaving(true);
    setError("");
    try {
      const removedChannelIds = (initialSlackChannels ?? [])
        .filter((ch) => !existingChannels.some((e) => e.id === ch.id))
        .map((ch) => ch.id);

      const groupMap = new Map<string, SlackChannelInput[]>();
      // Include newly selected channels.
      for (const ch of selectedChannels) {
        const arr = groupMap.get(ch.workspace) ?? [];
        arr.push({ channelId: ch.channelId, channelName: ch.channelName, startFrom: ch.startFrom || undefined, content: ch.content });
        groupMap.set(ch.workspace, arr);
      }
      // Re-upsert existing channels whose startFrom was edited (to persist the new date).
      const initialByID = new Map((initialSlackChannels ?? []).map((ch) => [ch.id, ch]));
      for (const ch of existingChannels) {
        const orig = initialByID.get(ch.id);
        if (orig && ch.startFrom !== (orig.startFrom ?? "")) {
          const arr = groupMap.get(ch.workspace) ?? [];
          arr.push({ channelId: ch.channelId, channelName: ch.channelName, startFrom: ch.startFrom || undefined });
          groupMap.set(ch.workspace, arr);
        }
      }
      const groups = [...groupMap.entries()].map(([workspace, channels]) => ({ workspace, channels }));

      // Collect pending DM configs: date-updated existing ones + newly selected previews
      const pendingDMConfigs: SlackDMConfigInput[] = [
        ...newDMConfigs,
        ...selectedDMPreviews.map((r) => ({ workspace: r.workspace, startFrom: r.startFrom })),
      ];

      const hasChanges = groups.length > 0 || removedChannelIds.length > 0 || pendingDMConfigs.length > 0 || removedDMConfigIds.length > 0;
      const slackPayload: SlackSavePayload | undefined = hasChanges
        ? { groups, removedChannelIds, dmConfigs: pendingDMConfigs.length > 0 ? pendingDMConfigs : undefined, removedDMConfigIds: removedDMConfigIds.length > 0 ? removedDMConfigIds : undefined }
        : undefined;

      const jiraKeys = selectedJiraIssues.length > 0 ? selectedJiraIssues.map((i) => i.key) : undefined;
      const removedJiraIds = (initialJiraIssues ?? [])
        .filter((i) => !existingJiraIssues.some((e) => e.id === i.id))
        .map((i) => i.id);

      const editedFiles = existingFiles
        .filter((f) => f.dirty)
        .map((f) => ({ id: f.id, name: f.name, content: f.content }));
      const hasTextChanges = newFiles.length > 0 || editedFiles.length > 0 || removedFileIds.length > 0;
      const textFilesPayload: TextFilesPayload | undefined = hasTextChanges
        ? {
            newFiles: newFiles.map((f) => ({ name: f.name, content: f.content })),
            editedFiles,
            removedIds: removedFileIds,
          }
        : undefined;

      const pendingTempIds = pendingUploads.length > 0 ? pendingUploads.map((f) => f.tempId) : undefined;
      const removedBranchIds = [
        ...removedVCSBranchIds,
        ...(initialVCSBranches ?? [])
          .filter((b) => !existingVCSBranches.some((e) => e.id === b.id))
          .map((b) => b.id)
          .filter((id) => !removedVCSBranchIds.includes(id)),
      ];
      await onSave(key.trim(), searchQuery.trim(), status, slackPayload, jiraKeys, removedJiraIds.length > 0 ? removedJiraIds : undefined, textFilesPayload, removedUploadedIds.length > 0 ? removedUploadedIds : undefined, pendingTempIds, newVCSBranches.length > 0 ? newVCSBranches : undefined, removedBranchIds.length > 0 ? removedBranchIds : undefined);
    } catch (err) {
      setError(String(err));
    } finally {
      setSaving(false);
    }
  };

  const toggleChannel = (result: ChannelResultItem) => {
    setSelectedChannels((prev) => {
      if (prev.some((ch) => ch.channelId === result.id)) {
        return prev.filter((ch) => ch.channelId !== result.id);
      }
      return [...prev, { channelId: result.id, channelName: result.name, workspace: result.workspace, startFrom: channelStartFrom, content: result.content }];
    });
  };

  const removeSelectedChannel = (channelId: string) => {
    setSelectedChannels((prev) => prev.filter((ch) => ch.channelId !== channelId));
  };

  const removeExistingChannel = (id: string) => {
    setExistingChannels((prev) => prev.filter((ch) => ch.id !== id));
  };

  const isSelected = (id: string) => selectedChannels.some((ch) => ch.channelId === id);

  const removeExistingDMConfig = (id: string) => {
    setExistingDMConfigs((prev) => prev.filter((dm) => dm.id !== id));
    setRemovedDMConfigIds((prev) => [...prev, id]);
  };

  const updateExistingDMConfig = (id: string, startFrom: string) => {
    setExistingDMConfigs((prev) => prev.map((dm) => dm.id === id ? { ...dm, startFrom } : dm));
    // Mark for re-upsert by including it in newDMConfigs with updated date
    setNewDMConfigs((prev) => {
      const existing = prev.find((dm) => dm.workspace === existingDMConfigs.find((e) => e.id === id)?.workspace);
      const ws = existingDMConfigs.find((e) => e.id === id)?.workspace ?? "";
      if (!ws) return prev;
      if (existing) return prev.map((dm) => dm.workspace === ws ? { ...dm, startFrom } : dm);
      return [...prev, { workspace: ws, startFrom }];
    });
    setRemovedDMConfigIds((prev) => [...prev, id]);
    setExistingDMConfigs((prev) => prev.filter((dm) => dm.id !== id));
  };

  const handleLoadDMs = async () => {
    if (!dmWorkspace.trim()) {
      setDmLoadError("Select a workspace first.");
      return;
    }
    setDmLoading(true);
    setDmLoadError("");
    try {
      const content = await PreviewSlackDMs(dmWorkspace.trim(), dmStartFrom.trim());
      const key = `${dmWorkspace.trim()}::${dmStartFrom.trim()}`;
      setDmPreviewResults((prev) => {
        const filtered = prev.filter((r) => `${r.workspace}::${r.startFrom}` !== key);
        return [...filtered, { workspace: dmWorkspace.trim(), startFrom: dmStartFrom.trim(), content: content ?? "" }];
      });
    } catch (err) {
      setDmLoadError(String(err));
    } finally {
      setDmLoading(false);
    }
  };

  const isDMPreviewSelected = (r: DMPreviewResult) =>
    selectedDMPreviews.some((p) => p.workspace === r.workspace && p.startFrom === r.startFrom);

  const toggleDMPreview = (r: DMPreviewResult) => {
    setSelectedDMPreviews((prev) =>
      isDMPreviewSelected(r)
        ? prev.filter((p) => !(p.workspace === r.workspace && p.startFrom === r.startFrom))
        : [...prev, r]
    );
  };

  const handleUploadFiles = async () => {
    setUploading(true);
    setUploadError("");
    try {
      if (initial?.id) {
        const result = (await PickAndExtractFiles(initial.id)) as unknown as UploadBatchResult;
        if (result.files?.length > 0) {
          setUploadedFiles((prev) => [...prev, ...result.files]);
        }
        if (result.errors?.length > 0) {
          setUploadError(result.errors.join("\n"));
        }
      } else {
        const result = (await PickAndExtractFilesPreview()) as unknown as UploadPreviewBatchResult;
        if (result.previews?.length > 0) {
          setPendingUploads((prev) => [...prev, ...result.previews]);
        }
        if (result.errors?.length > 0) {
          setUploadError(result.errors.join("\n"));
        }
      }
    } catch (err) {
      setUploadError(String(err));
    } finally {
      setUploading(false);
    }
  };

  const removePendingUpload = (tempId: string) => {
    setPendingUploads((prev) => prev.filter((f) => f.tempId !== tempId));
    DiscardPendingUpload(tempId).catch(() => {});
  };

  const removeUploadedFile = async (id: string) => {
    // If this is an existing file we mark it for deletion on save.
    // We also eagerly call DeleteUploadedFile since PickAndExtractFiles already persisted it.
    try {
      await DeleteUploadedFile(id);
    } catch {
      // best-effort
    }
    setUploadedFiles((prev) => prev.filter((f) => f.id !== id));
    setRemovedUploadedIds((prev) => [...prev, id]);
  };

  const workspaceSelect = (
    <select
      value={slackWorkspace}
      onChange={(e) => setSlackWorkspace(e.target.value)}
      disabled={workspacesLoading}
      style={{
        flex: "0 0 150px",
        opacity: workspacesLoading ? 0.5 : 1,
        borderColor: workspacesError ? "var(--danger)" : undefined,
      }}
    >
      {workspacesLoading && <option value="">Loading…</option>}
      {!workspacesLoading && workspaces.length === 0 && <option value="">No workspaces</option>}
      {!workspacesLoading && workspaces.length > 0 && !slackWorkspace && (
        <option value="">Select workspace</option>
      )}
      {workspaces.map((ws) => (
        <option key={ws.id} value={ws.name}>{ws.name}</option>
      ))}
    </select>
  );

  const slackChannelCount = existingChannels.length + selectedChannels.length;
  const slackDMCount = existingDMConfigs.length + newDMConfigs.length + selectedDMPreviews.length;
  const uploadedCount = uploadedFiles.length + pendingUploads.length;
  const vcsBranchCount = existingVCSBranches.length + newVCSBranches.length;

  // ownerRepo for which branches are currently loaded — guards against stale fetches
  const loadedForRepo = useRef("");

  const fetchRepoBranches = async (ownerRepo: string) => {
    const trimmed = ownerRepo.trim();
    if (!trimmed.includes("/")) return;
    // Already loaded for this repo — skip
    if (loadedForRepo.current === trimmed && repoBranches.length > 0) return;
    loadedForRepo.current = trimmed;
    setBranchPickerLoading(true);
    setBranchPickerError("");
    setRepoBranches([]);
    try {
      const branches = await ListRepoBranches(vcsPlatform, trimmed);
      setRepoBranches(branches ?? []);
    } catch (e) {
      setBranchPickerError(String(e));
      loadedForRepo.current = "";
    } finally {
      setBranchPickerLoading(false);
    }
  };

  // Called when user selects a repo from the picker — load branches immediately
  const handleRepoSelect = (fullName: string) => {
    if (fullName === vcsOwnerRepo) return;
    if (debounceRef.current) clearTimeout(debounceRef.current);
    loadedForRepo.current = "";
    setVCSOwnerRepo(fullName);
    setVCSBranch("");
    setRepoBranches([]);
    setBranchPickerOpen(false);
    setBranchPickerError("");
    void fetchRepoBranches(fullName);
  };

  // Called when user types in the repo input manually — debounce 2s
  const handleOwnerRepoChange = (value: string) => {
    setVCSOwnerRepo(value);
    setVCSBranch("");
    setRepoBranches([]);
    setBranchPickerOpen(false);
    setBranchPickerError("");
    loadedForRepo.current = "";
    if (debounceRef.current) clearTimeout(debounceRef.current);
    const trimmed = value.trim();
    if (trimmed.includes("/") && trimmed.split("/").every(Boolean)) {
      debounceRef.current = setTimeout(() => {
        void fetchRepoBranches(trimmed);
      }, 2000);
    }
  };

  const handleAddVCSBranch = () => {
    setVCSAddError("");
    const ownerRepo = vcsOwnerRepo.trim();
    const branch = vcsBranch.trim();
    if (!ownerRepo) {
      setVCSAddError("Enter owner/repo (e.g. acme/my-service).");
      return;
    }
    if (!ownerRepo.includes("/")) {
      setVCSAddError("Format must be owner/repo, e.g. acme/my-service");
      return;
    }
    const duplicate = newVCSBranches.some(
      (b) => b.platform === vcsPlatform && b.ownerRepo === ownerRepo && b.branch === branch
    ) || existingVCSBranches.some(
      (b) => b.repo?.platform === vcsPlatform && `${b.repo?.owner}/${b.repo?.name}` === ownerRepo && b.branch === branch
    );
    if (duplicate) {
      setVCSAddError("This branch is already linked.");
      return;
    }
    setNewVCSBranches((prev) => [...prev, { platform: vcsPlatform, ownerRepo, branch }]);
    setVCSOwnerRepo("");
    setVCSBranch("");
    setBranchPickerOpen(false);
    setRepoBranches([]);
    setBranchFilter("");
    if (debounceRef.current) clearTimeout(debounceRef.current);
  };

  // close branch picker when clicking outside
  useEffect(() => {
    if (!branchPickerOpen) return;
    const handler = (e: MouseEvent) => {
      if (branchPickerRef.current && !branchPickerRef.current.contains(e.target as Node)) {
        setBranchPickerOpen(false);
      }
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, [branchPickerOpen]);

  // clear filter and focus when picker opens (not when loading state changes)
  useEffect(() => {
    if (!branchPickerOpen) return;
    setBranchFilter("");
    const t = setTimeout(() => branchFilterRef.current?.focus(), 30);
    return () => clearTimeout(t);
  }, [branchPickerOpen]);

  // auto-open picker once branches finish loading (background fetch)
  const prevLoading = useRef(false);
  useEffect(() => {
    if (prevLoading.current && !branchPickerLoading && repoBranches.length > 0) {
      setBranchPickerOpen(true);
    }
    prevLoading.current = branchPickerLoading;
  }, [branchPickerLoading, repoBranches.length]);

  // load tracked repos when VCS tab is opened
  useEffect(() => {
    if (activeTab !== "vcs") return;
    ListTrackedRepos(vcsPlatform)
      .then((repos) => setTrackedRepos((repos as VCSRepo[]) ?? []))
      .catch(() => setTrackedRepos([]));
  }, [activeTab, vcsPlatform]);

  // close repo picker on outside click
  useEffect(() => {
    if (!repoPickerOpen) return;
    const handler = (e: MouseEvent) => {
      if (repoPickerRef.current && !repoPickerRef.current.contains(e.target as Node)) {
        setRepoPickerOpen(false);
      }
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, [repoPickerOpen]);

  // auto-focus repo filter when picker opens
  useEffect(() => {
    if (repoPickerOpen) {
      setRepoFilter("");
      setTimeout(() => repoFilterRef.current?.focus(), 50);
    }
  }, [repoPickerOpen]);

  return (
    <div className="modal-overlay" onClick={onCancel}>
      <div className="modal modal--wide" onClick={(e) => e.stopPropagation()}>
        <h2>{initial ? "Edit Context" : "New Context"}</h2>

        {error && <div className="error-banner">{error}</div>}

        <form onSubmit={handleSubmit}>
          {/* Always-visible fields */}
          <div className="form-group" style={{ padding: "0 0 0 0" }}>
            <label>Key</label>
            <input
              value={key}
              onChange={(e) => setKey(e.target.value)}
              placeholder="e.g. sprint-42, auth-refactor"
              autoFocus
            />
            <div className="form-hint">A short human-readable name for this context</div>
          </div>

          {initial && (
            <div className="form-group">
              <label>Status</label>
              <select value={status} onChange={(e) => setStatus(e.target.value)}>
                <option value="active">Active</option>
                <option value="archived">Archived</option>
              </select>
            </div>
          )}

          {/* Tab bar — all providers are optional */}
          <div className="ctx-tabs">
            <button
              type="button"
              className={`ctx-tab${activeTab === "jira" ? " ctx-tab--active" : ""}`}
              onClick={() => setActiveTab("jira")}
            >
              Jira
              {(existingJiraIssues.length > 0 || selectedJiraIssues.length > 0) ? (
                <span className="ctx-tab-badge">{existingJiraIssues.length + selectedJiraIssues.length}</span>
              ) : (
                <span className="ctx-tab-optional">optional</span>
              )}
            </button>
            <button
              type="button"
              className={`ctx-tab${activeTab === "slack" ? " ctx-tab--active" : ""}`}
              onClick={() => setActiveTab("slack")}
            >
              Slack
              {(slackChannelCount + slackDMCount) > 0 ? (
                <span className="ctx-tab-badge">{slackChannelCount + slackDMCount}</span>
              ) : (
                <span className="ctx-tab-optional">optional</span>
              )}
            </button>
            <button
              type="button"
              className={`ctx-tab${activeTab === "texts" ? " ctx-tab--active" : ""}`}
              onClick={() => setActiveTab("texts")}
            >
              Texts & Files
              {(existingFiles.length + newFiles.length + uploadedCount) > 0 ? (
                <span className="ctx-tab-badge">{existingFiles.length + newFiles.length + uploadedCount}</span>
              ) : (
                <span className="ctx-tab-optional">optional</span>
              )}
            </button>
            <button
              type="button"
              className={`ctx-tab${activeTab === "vcs" ? " ctx-tab--active" : ""}`}
              onClick={() => setActiveTab("vcs")}
            >
              Git / PRs
              {vcsBranchCount > 0 ? (
                <span className="ctx-tab-badge">{vcsBranchCount}</span>
              ) : (
                <span className="ctx-tab-optional">optional</span>
              )}
            </button>
          </div>

          <div className="modal-scroll-body">
          {/* Jira panel */}
          {activeTab === "jira" && (
            <div className="ctx-panel">
              <div className="form-group" style={{ marginBottom: 0 }}>

                {/* Saved tickets */}
                {existingJiraIssues.length > 0 && (
                  <div className="channel-saved-list" style={{ marginBottom: 12 }}>
                    <div className="channel-saved-header">
                      Saved tickets
                      <span className="channel-saved-count">{existingJiraIssues.length}</span>
                    </div>
                    {existingJiraIssues.map((issue) => (
                      <div key={issue.id} className="channel-saved-row">
                        <span className="jira-saved-key">{issue.jiraKey}</span>
                        <span className="channel-saved-name" style={{ fontWeight: 400 }}>{issue.summary}</span>
                        <span className="jql-result-status" style={{ marginRight: 0 }}>{issue.status}</span>
                        <button
                          type="button"
                          className="channel-remove-btn"
                          title="Remove ticket"
                          onClick={() => removeExistingJiraIssue(issue.id)}
                        ><X size={11} /></button>
                      </div>
                    ))}
                  </div>
                )}

                {/* Selected issues — to be added */}
                {selectedJiraIssues.length > 0 && (
                  <div className="channel-saved-list channel-saved-list--new" style={{ marginBottom: 12 }}>
                    <div className="channel-saved-header">To be added</div>
                    {selectedJiraIssues.map((issue) => (
                      <div key={issue.key} className="channel-saved-row channel-saved-row--new">
                        <span className="jira-saved-key">{issue.key}</span>
                        <span className="channel-saved-name" style={{ fontWeight: 400 }}>{issue.summary}</span>
                        <span className="jql-result-status" style={{ marginRight: 0 }}>{issue.status}</span>
                        <button
                          type="button"
                          className="channel-remove-btn"
                          title="Remove"
                          onClick={() => removeSelectedJiraIssue(issue.key)}
                        ><X size={11} /></button>
                      </div>
                    ))}
                  </div>
                )}

                {/* JQL search section */}
                <div className="channel-search-section">
                  <div className="channel-search-label">Search issues (JQL)</div>
                  <textarea
                    value={searchQuery}
                    onChange={(e) => setSearchQuery(e.target.value)}
                    onFocus={() => setJqlFocused(true)}
                    onBlur={() => setJqlFocused(false)}
                    placeholder='project = PROJ AND sprint in openSprints() ORDER BY updated DESC'
                    rows={3}
                    style={{ marginBottom: 6 }}
                  />
                  {jqlFocused && (
                    <div className="jql-suggestions" style={{ marginTop: 0, marginBottom: 6 }}>
                      {JQL_TEMPLATES.map((t) => (
                        <button
                          key={t.label}
                          type="button"
                          className="jql-chip"
                          onMouseDown={(e) => {
                            e.preventDefault();
                            setSearchQuery(t.query);
                          }}
                        >
                          {t.label}
                        </button>
                      ))}
                    </div>
                  )}
                  <div className="slack-conditions-footer" style={{ marginBottom: 0 }}>
                    <span className="form-hint" style={{ margin: 0 }}>
                      Jira Query Language — click a template above to start
                    </span>
                    <button
                      type="button"
                      className="btn-secondary"
                      onClick={() => void handleSearchJQL()}
                      disabled={jqlSearching}
                    >
                      {jqlSearching ? "Searching…" : "Search"}
                    </button>
                  </div>

                  {jqlSearchError && (
                    <div className="field-error" style={{ marginTop: 6 }}>{jqlSearchError}</div>
                  )}

                  {jqlResults.length > 0 && (
                    <div className="jql-results" style={{ marginTop: 8 }}>
                      <label className="jql-select-all">
                        <input
                          type="checkbox"
                          checked={jqlResults.every((r) => isJiraSelected(r.key))}
                          ref={(el) => {
                            if (el) {
                              const count = jqlResults.filter((r) => isJiraSelected(r.key)).length;
                              el.indeterminate = count > 0 && count < jqlResults.length;
                            }
                          }}
                          onChange={toggleAllJiraIssues}
                        />
                        <span>{jqlResults.every((r) => isJiraSelected(r.key)) ? "Deselect all" : "Select all"}</span>
                        {jqlResults.some((r) => isJiraSelected(r.key)) && (
                          <span className="jql-selection-count">
                            {jqlResults.filter((r) => isJiraSelected(r.key)).length} of {jqlResults.length} selected
                          </span>
                        )}
                      </label>
                      {jqlResults.map((issue) => (
                        <label
                          key={issue.key}
                          className={`jql-result-row${isJiraSelected(issue.key) ? " jql-result-row--selected" : ""}`}
                        >
                          <input
                            type="checkbox"
                            checked={isJiraSelected(issue.key)}
                            onChange={() => toggleJiraIssue(issue)}
                          />
                          <span className="jql-result-key">{issue.key}</span>
                          <span className="jql-result-summary">{issue.summary}</span>
                          <span className="jql-result-status">{issue.status}</span>
                        </label>
                      ))}
                    </div>
                  )}
                </div>
              </div>
            </div>
          )}

          {/* Slack panel */}
          {activeTab === "slack" && (
            <div className="ctx-panel">
              <div className="form-group" style={{ marginBottom: 0 }}>
                <label style={{ display: "flex", alignItems: "center", gap: 6 }}>
                  Channels
                  <span style={{ fontSize: 11, fontWeight: 400, color: "var(--text-muted)" }}>optional</span>
                </label>

                {/* Saved channels */}
                {existingChannels.length > 0 && (
                  <div className="channel-saved-list">
                    <div className="channel-saved-header">
                      Saved channels
                      <span className="channel-saved-count">{existingChannels.length}</span>
                    </div>
                    {existingChannels.map((ch) => (
                      <div key={ch.id} className="channel-saved-row">
                        <span className="channel-saved-name">
                          <span className="channel-hash">#</span>{ch.channelName}
                        </span>
                        <span className="channel-saved-workspace">{ch.workspace}</span>
                        <DatePicker
                          selected={ch.startFrom ? new Date(ch.startFrom) : null}
                          onChange={(date: Date | null) =>
                            setExistingChannels((prev) =>
                              prev.map((x) => x.id === ch.id ? { ...x, startFrom: date ? date.toISOString().slice(0, 10) : "" } : x)
                            )
                          }
                          dateFormat="yyyy-MM-dd"
                          placeholderText="all time"
                          isClearable
                          className="dm-datepicker-input"
                          popperPlacement="bottom-end"
                        />
                        <button
                          type="button"
                          className="channel-remove-btn"
                          title="Remove channel"
                          onClick={() => removeExistingChannel(ch.id)}
                        ><X size={11} /></button>
                      </div>
                    ))}
                  </div>
                )}

                {/* Newly selected channels */}
                {selectedChannels.length > 0 && (
                  <div className="channel-saved-list channel-saved-list--new">
                    <div className="channel-saved-header">To be added</div>
                    {selectedChannels.map((ch) => (
                      <div key={ch.channelId} className="channel-saved-row channel-saved-row--new">
                        <span className="channel-saved-name">
                          <span className="channel-hash">#</span>{ch.channelName}
                        </span>
                        <span className="channel-saved-workspace">{ch.workspace}</span>
                        {ch.startFrom && (
                          <span style={{ fontSize: 11, color: "var(--text-muted)" }}>since {ch.startFrom}</span>
                        )}
                        <button
                          type="button"
                          className="channel-remove-btn"
                          title="Remove"
                          onClick={() => removeSelectedChannel(ch.channelId)}
                        ><X size={11} /></button>
                      </div>
                    ))}
                  </div>
                )}

                {/* Search form */}
                <div className="channel-search-section">
                  <div className="channel-search-label">Search channels</div>
                  <div className="slack-condition-row" style={{ marginBottom: 8 }}>
                    {workspaceSelect}
                    <input
                      value={slackPattern}
                      onChange={(e) => setSlackPattern(e.target.value)}
                      placeholder="*channel-name*"
                      onKeyDown={(e) => {
                        if (e.key === "Enter") { e.preventDefault(); void handleSearchChannels(); }
                      }}
                    />
                    <DatePicker
                      selected={channelStartFrom ? new Date(channelStartFrom) : null}
                      onChange={(date: Date | null) => setChannelStartFrom(date ? date.toISOString().slice(0, 10) : "")}
                      dateFormat="yyyy-MM-dd"
                      placeholderText="since (optional)"
                      isClearable
                      className="dm-datepicker-input"
                      wrapperClassName="dm-datepicker-wrapper"
                      popperPlacement="bottom-end"
                    />
                    <button
                      type="button"
                      className="btn-secondary"
                      onClick={() => void handleSearchChannels()}
                      disabled={searching}
                      style={{ flexShrink: 0 }}
                    >
                      {searching ? "Searching…" : "Search"}
                    </button>
                  </div>

                  {workspacesError && (
                    <div className="field-error" style={{ marginTop: 4 }}>{workspacesError}</div>
                  )}
                  {channelError && (
                    <div className="field-error" style={{ marginTop: 4 }}>{channelError}</div>
                  )}

                  {channelResults.length > 0 && (
                    <div className="channel-results">
                      {channelResults.map((ch) => (
                        <label
                          key={ch.id}
                          className={`channel-result-row${isSelected(ch.id) ? " selected" : ""}`}
                        >
                          <input
                            type="checkbox"
                            checked={isSelected(ch.id)}
                            onChange={() => toggleChannel(ch)}
                          />
                          <div className="channel-result-info">
                            <span className="channel-result-name">
                              <span>#</span>{ch.name}
                            </span>
                            <div className="channel-result-badges">
                              <span className="channel-result-workspace">{ch.workspace}</span>
                              <span className="channel-result-id">{ch.id}</span>
                            </div>
                          </div>
                        </label>
                      ))}
                    </div>
                  )}
                </div>

                <div className="form-hint" style={{ marginTop: 8 }}>
                  Search across workspaces — multiple conditions are merged
                </div>
              </div>

              {/* DM configs section */}
              <div className="form-group" style={{ marginBottom: 0, marginTop: 16, borderTop: "1px solid var(--border)", paddingTop: 16 }}>
                <label style={{ display: "flex", alignItems: "center", gap: 6 }}>
                  Personal messages
                  <span style={{ fontSize: 11, fontWeight: 400, color: "var(--text-muted)" }}>optional</span>
                </label>

                {/* Saved DM configs */}
                {existingDMConfigs.length > 0 && (
                  <div className="channel-saved-list" style={{ marginBottom: 12 }}>
                    <div className="channel-saved-header">
                      Saved DM configs
                      <span className="channel-saved-count">{existingDMConfigs.length}</span>
                    </div>
                    {existingDMConfigs.map((dm) => (
                      <div key={dm.id} className="channel-saved-row">
                        <span className="channel-saved-workspace">{dm.workspace}</span>
                        <span className="channel-saved-name" style={{ flex: 1, fontWeight: 400 }}>
                          {dm.startFrom ? `since ${dm.startFrom}` : "all time"}
                        </span>
                        <DatePicker
                          selected={dm.startFrom ? new Date(dm.startFrom) : null}
                          onChange={(date: Date | null) => updateExistingDMConfig(dm.id, date ? date.toISOString().slice(0, 10) : "")}
                          dateFormat="yyyy-MM-dd"
                          placeholderText="all time"
                          isClearable
                          className="dm-datepicker-input"
                          popperPlacement="bottom-end"
                        />
                        <button
                          type="button"
                          className="channel-remove-btn"
                          title="Remove DM config"
                          onClick={() => removeExistingDMConfig(dm.id)}
                        ><X size={11} /></button>
                      </div>
                    ))}
                  </div>
                )}

                {/* New DM configs to be added */}
                {newDMConfigs.length > 0 && (
                  <div className="channel-saved-list channel-saved-list--new" style={{ marginBottom: 12 }}>
                    <div className="channel-saved-header">To be added</div>
                    {newDMConfigs.map((dm, i) => (
                      <div key={`${dm.workspace}-${i}`} className="channel-saved-row channel-saved-row--new">
                        <span className="channel-saved-workspace">{dm.workspace}</span>
                        <span className="channel-saved-name" style={{ flex: 1, fontWeight: 400 }}>
                          {dm.startFrom ? `since ${dm.startFrom}` : "all time"}
                        </span>
                        <button
                          type="button"
                          className="channel-remove-btn"
                          title="Remove"
                          onClick={() => setNewDMConfigs((prev) => prev.filter((_, idx) => idx !== i))}
                        ><X size={11} /></button>
                      </div>
                    ))}
                  </div>
                )}

                {/* Load button row */}
                <div className="channel-search-section">
                  <div className="channel-search-label">Load personal messages</div>
                  <div className="slack-condition-row" style={{ marginBottom: 8 }}>
                    <select
                      value={dmWorkspace}
                      onChange={(e) => setDmWorkspace(e.target.value)}
                      disabled={workspacesLoading}
                      style={{ flex: "0 0 150px", opacity: workspacesLoading ? 0.5 : 1 }}
                    >
                      {workspacesLoading && <option value="">Loading…</option>}
                      {!workspacesLoading && workspaces.length === 0 && <option value="">No workspaces</option>}
                      {!workspacesLoading && workspaces.length > 0 && !dmWorkspace && (
                        <option value="">Select workspace</option>
                      )}
                      {workspaces.map((ws) => (
                        <option key={ws.id} value={ws.name}>{ws.name}</option>
                      ))}
                    </select>
                    <DatePicker
                      selected={dmStartFrom ? new Date(dmStartFrom) : null}
                      onChange={(date: Date | null) => setDmStartFrom(date ? date.toISOString().slice(0, 10) : "")}
                      dateFormat="yyyy-MM-dd"
                      placeholderText="since (optional)"
                      isClearable
                      className="dm-datepicker-input"
                      wrapperClassName="dm-datepicker-wrapper"
                      popperPlacement="bottom-start"
                    />
                    <button
                      type="button"
                      className="btn-secondary"
                      onClick={() => void handleLoadDMs()}
                      disabled={dmLoading}
                      style={{ flexShrink: 0 }}
                    >
                      {dmLoading ? "Loading…" : "Load"}
                    </button>
                  </div>

                  {dmLoadError && (
                    <div className="field-error" style={{ marginTop: 4 }}>{dmLoadError}</div>
                  )}

                  {dmPreviewResults.length > 0 && (
                    <div className="channel-results">
                      {dmPreviewResults.map((r) => {
                        const selected = isDMPreviewSelected(r);
                        const preview = r.content
                          ? r.content.split("\n").filter(Boolean).slice(0, 2).join(" · ") || r.content.slice(0, 120)
                          : null;
                        return (
                          <label
                            key={`${r.workspace}::${r.startFrom}`}
                            className={`channel-result-row${selected ? " selected" : ""}`}
                          >
                            <input
                              type="checkbox"
                              checked={selected}
                              disabled={!r.content}
                              onChange={() => toggleDMPreview(r)}
                            />
                            <div className="channel-result-info">
                              <span className="channel-result-name">
                                DMs · {r.workspace}{r.startFrom ? ` since ${r.startFrom}` : " (all time)"}
                              </span>
                              <div className="channel-result-badges">
                                {r.content ? (
                                  <span className="channel-result-workspace" style={{ maxWidth: 340, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                                    {preview}
                                  </span>
                                ) : (
                                  <span className="channel-result-id" style={{ color: "var(--text-muted)" }}>No messages found</span>
                                )}
                              </div>
                            </div>
                          </label>
                        );
                      })}
                    </div>
                  )}

                  <div className="form-hint" style={{ marginTop: 8 }}>
                    Loads personal messages via slack-cli list-dms · select results to add to context
                  </div>
                </div>
              </div>
            </div>
          )}

          {/* Texts & Files panel */}
          {activeTab === "texts" && (
            <div className="ctx-panel">
              <div className="form-group" style={{ marginBottom: 0 }}>

                {/* Existing text entries */}
                {existingFiles.map((f) => (
                  <div key={f.id} className="text-file-block">
                    <div className="text-file-header">
                      <input
                        className="text-file-name"
                        value={f.name}
                        onChange={(e) =>
                          setExistingFiles((prev) =>
                            prev.map((x) => x.id === f.id ? { ...x, name: e.target.value, dirty: true } : x)
                          )
                        }
                        placeholder="File name (optional)"
                      />
                      <button
                        type="button"
                        className="channel-remove-btn"
                        title="Remove text file"
                        onClick={() => {
                          setExistingFiles((prev) => prev.filter((x) => x.id !== f.id));
                          setRemovedFileIds((prev) => [...prev, f.id]);
                        }}
                      ><X size={11} /></button>
                    </div>
                    <textarea
                      className="text-file-content"
                      value={f.content}
                      onChange={(e) =>
                        setExistingFiles((prev) =>
                          prev.map((x) => x.id === f.id ? { ...x, content: e.target.value, dirty: true } : x)
                        )
                      }
                      placeholder="Paste or type plain text content…"
                      rows={8}
                    />
                  </div>
                ))}

                {/* New text entries being added */}
                {newFiles.map((f) => (
                  <div key={f.tempId} className="text-file-block text-file-block--new">
                    <div className="text-file-header">
                      <input
                        className="text-file-name"
                        value={f.name}
                        onChange={(e) =>
                          setNewFiles((prev) =>
                            prev.map((x) => x.tempId === f.tempId ? { ...x, name: e.target.value } : x)
                          )
                        }
                        placeholder="File name (optional)"
                      />
                      <button
                        type="button"
                        className="channel-remove-btn"
                        title="Remove"
                        onClick={() => setNewFiles((prev) => prev.filter((x) => x.tempId !== f.tempId))}
                      ><X size={11} /></button>
                    </div>
                    <textarea
                      className="text-file-content"
                      value={f.content}
                      onChange={(e) =>
                        setNewFiles((prev) =>
                          prev.map((x) => x.tempId === f.tempId ? { ...x, content: e.target.value } : x)
                        )
                      }
                      placeholder="Paste or type plain text content…"
                      rows={8}
                    />
                  </div>
                ))}

                <button
                  type="button"
                  className="btn-secondary"
                  style={{ marginTop: 8 }}
                  onClick={() =>
                    setNewFiles((prev) => [
                      ...prev,
                      { tempId: crypto.randomUUID(), name: "", content: "" },
                    ])
                  }
                >
                  + Add text
                </button>
              </div>

              {/* Uploaded documents */}
              <div className="form-group" style={{ marginBottom: 0, marginTop: 16, borderTop: "1px solid var(--border)", paddingTop: 16 }}>
                <label style={{ display: "flex", alignItems: "center", gap: 6 }}>
                  Uploaded files
                  <span style={{ fontSize: 11, fontWeight: 400, color: "var(--text-muted)" }}>PDF, DOCX, images, etc.</span>
                </label>

                {uploadedFiles.map((f) => (
                  <div key={f.id} className="text-file-block">
                    <div className="text-file-header">
                      <span className="text-file-name" style={{ flex: 1, fontWeight: 500 }}>
                        {f.originalName}
                      </span>
                      <span style={{ fontSize: 11, color: "var(--text-muted)", marginRight: 8 }}>
                        {f.fileType.toUpperCase()} · {(f.fileSize / 1024).toFixed(1)} KB
                      </span>
                      <button
                        type="button"
                        className="channel-remove-btn"
                        title="Remove uploaded file"
                        onClick={() => void removeUploadedFile(f.id)}
                      ><X size={11} /></button>
                    </div>
                    <pre className="text-file-content" style={{ maxHeight: 80, overflow: "hidden", fontSize: 11, opacity: 0.7 }}>
                      {f.extractedText.slice(0, 300)}{f.extractedText.length > 300 ? "…" : ""}
                    </pre>
                  </div>
                ))}

                {pendingUploads.map((f) => (
                  <div key={f.tempId} className="text-file-block text-file-block--new">
                    <div className="text-file-header">
                      <span className="text-file-name" style={{ flex: 1, fontWeight: 500 }}>
                        {f.originalName}
                      </span>
                      <span style={{ fontSize: 11, color: "var(--text-muted)", marginRight: 8 }}>
                        {f.fileType.toUpperCase()} · {(f.fileSize / 1024).toFixed(1)} KB
                      </span>
                      <button
                        type="button"
                        className="channel-remove-btn"
                        title="Remove pending file"
                        onClick={() => removePendingUpload(f.tempId)}
                      ><X size={11} /></button>
                    </div>
                    <pre className="text-file-content" style={{ maxHeight: 80, overflow: "hidden", fontSize: 11, opacity: 0.7 }}>
                      {f.extractedText.slice(0, 300)}{f.extractedText.length > 300 ? "…" : ""}
                    </pre>
                  </div>
                ))}

                {uploadError && (
                  <div className="field-error" style={{ marginBottom: 8 }}>{uploadError}</div>
                )}

                <button
                  type="button"
                  className="btn-secondary"
                  style={{ marginTop: (uploadedFiles.length + pendingUploads.length) > 0 ? 8 : 0 }}
                  onClick={() => void handleUploadFiles()}
                  disabled={uploading}
                >
                  {uploading ? (
                    <span className="btn-with-spinner">
                      <Spinner size={14} color="var(--text)" />
                      Extracting…
                    </span>
                  ) : (
                    "↑ Upload files"
                  )}
                </button>

                <div className="form-hint" style={{ marginTop: 8 }}>
                  Supported: PDF, DOCX, RTF, ODT, TXT, PNG, JPG, JPEG, WEBP — text is extracted and stored in the context
                </div>
              </div>
            </div>
          )}

          {/* VCS / Git panel */}
          {activeTab === "vcs" && (
            <div className="ctx-panel">
              <div className="form-group" style={{ marginBottom: 0 }}>

                {/* Existing branches */}
                {existingVCSBranches.length > 0 && (
                  <div className="channel-saved-list" style={{ marginBottom: 12 }}>
                    <div className="channel-saved-header">
                      Linked branches
                      <span className="channel-saved-count">{existingVCSBranches.length}</span>
                    </div>
                    {existingVCSBranches.map((b) => (
                      <div key={b.id} className="channel-saved-row">
                        <span className="platform-badge platform-badge--github">
                          {b.repo?.platform ?? "github"}
                        </span>
                        <span className="channel-saved-name" style={{ fontWeight: 500 }}>
                          {b.repo?.owner}/{b.repo?.name}
                        </span>
                        <span style={{ color: "var(--text-muted)", fontSize: 12 }}>{b.branch ? `:${b.branch}` : " · all PRs"}</span>
                        <button
                          type="button"
                          className="channel-remove-btn"
                          title="Remove branch"
                          onClick={() => {
                            setExistingVCSBranches((prev) => prev.filter((x) => x.id !== b.id));
                            setRemovedVCSBranchIds((prev) => [...prev, b.id]);
                          }}
                        ><X size={11} /></button>
                      </div>
                    ))}
                  </div>
                )}

                {/* Newly added branches */}
                {newVCSBranches.length > 0 && (
                  <div className="channel-saved-list channel-saved-list--new" style={{ marginBottom: 12 }}>
                    <div className="channel-saved-header">To be linked</div>
                    {newVCSBranches.map((b, i) => (
                      <div key={`${b.platform}-${b.ownerRepo}-${b.branch}-${i}`} className="channel-saved-row channel-saved-row--new">
                        <span className="platform-badge platform-badge--github">{b.platform}</span>
                        <span className="channel-saved-name" style={{ fontWeight: 500 }}>{b.ownerRepo}</span>
                        <span style={{ color: "var(--text-muted)", fontSize: 12 }}>{b.branch ? `:${b.branch}` : " · all PRs"}</span>
                        <button
                          type="button"
                          className="channel-remove-btn"
                          title="Remove"
                          onClick={() => setNewVCSBranches((prev) => prev.filter((_, idx) => idx !== i))}
                        ><X size={11} /></button>
                      </div>
                    ))}
                  </div>
                )}

                {/* Add branch form */}
                <div className="channel-search-section">
                  <div className="channel-search-label">Link repository</div>

                  {/* Row 1: platform + repo selector */}
                  <div className="vcs-add-row">
                    <select
                      value={vcsPlatform}
                      onChange={(e) => { setVCSPlatform(e.target.value as VCSPlatform); setRepoBranches([]); setVCSBranch(""); setVCSOwnerRepo(""); setTrackedRepos([]); }}
                      className="vcs-platform-select"
                    >
                      {vcsPlatforms.map((p) => (
                        <option key={p.platform} value={p.platform} disabled={!p.enabled}>
                          {p.label}{!p.enabled ? " (soon)" : ""}
                        </option>
                      ))}
                      {vcsPlatforms.length === 0 && <option value="github">GitHub</option>}
                    </select>

                    {/* Repo combobox */}
                    <div ref={repoPickerRef} className="vcs-branch-selector-wrap">
                      <button
                        type="button"
                        className={`vcs-branch-trigger${repoPickerOpen ? " vcs-branch-trigger--open" : ""}`}
                        onClick={() => setRepoPickerOpen((v) => !v)}
                      >
                        <span className="vcs-branch-trigger-label">
                          {vcsOwnerRepo
                            ? vcsOwnerRepo
                            : <span style={{ color: "var(--text-muted)" }}>Select repository…</span>
                          }
                        </span>
                        {vcsOwnerRepo ? (
                          <span
                            className="vcs-branch-clear"
                            onMouseDown={(e) => { e.stopPropagation(); handleOwnerRepoChange(""); setRepoPickerOpen(false); }}
                            title="Clear"
                          ><X size={11} /></span>
                        ) : (
                          <ChevronDown size={13} className={`vcs-branch-chevron${repoPickerOpen ? " vcs-branch-chevron--open" : ""}`} />
                        )}
                      </button>

                      {repoPickerOpen && (
                        <div className="branch-picker-dropdown">
                          {trackedRepos.length === 0 ? (
                            <div className="branch-picker-empty">No tracked repositories. Add repos in the Code Reviews tab first.</div>
                          ) : (
                            <>
                              <div className="branch-picker-search-row">
                                <input
                                  ref={repoFilterRef}
                                  className="branch-picker-filter"
                                  value={repoFilter}
                                  onChange={(e) => setRepoFilter(e.target.value)}
                                  placeholder="Filter repositories…"
                                  onKeyDown={(e) => { if (e.key === "Escape") setRepoPickerOpen(false); }}
                                />
                              </div>
                              <div className="branch-picker-list">
                                {trackedRepos
                                  .filter((r) => !repoFilter || `${r.owner}/${r.name}`.toLowerCase().includes(repoFilter.toLowerCase()))
                                  .map((r) => {
                                    const fullName = `${r.owner}/${r.name}`;
                                    return (
                                      <button
                                        key={r.id}
                                        type="button"
                                        className={`branch-picker-item${vcsOwnerRepo === fullName ? " branch-picker-item--selected" : ""}`}
                                        onMouseDown={(e) => {
                                          e.preventDefault();
                                          handleRepoSelect(fullName);
                                          setRepoPickerOpen(false);
                                        }}
                                      >
                                        <span>{fullName}</span>
                                        {r.localPath && (
                                          <span className="branch-picker-item__path">{r.localPath}</span>
                                        )}
                                      </button>
                                    );
                                  })
                                }
                                {trackedRepos.filter((r) => !repoFilter || `${r.owner}/${r.name}`.toLowerCase().includes(repoFilter.toLowerCase())).length === 0 && (
                                  <div className="branch-picker-empty">No repositories match</div>
                                )}
                              </div>
                            </>
                          )}
                        </div>
                      )}
                    </div>
                  </div>

                  {/* Row 2: branch selector + add */}
                  <div className="vcs-add-row" style={{ marginTop: 6 }}>
                    <div ref={branchPickerRef} className="vcs-branch-selector-wrap">
                      {/* Trigger button */}
                      <button
                        type="button"
                        className={`vcs-branch-trigger${branchPickerOpen ? " vcs-branch-trigger--open" : ""}${!vcsOwnerRepo.trim().includes("/") ? " vcs-branch-trigger--disabled" : ""}`}
                        onClick={() => {
                          if (!vcsOwnerRepo.trim().includes("/")) return;
                          if (!branchPickerLoading) {
                            // fetch only if not yet loaded for this repo
                            void fetchRepoBranches(vcsOwnerRepo);
                            setBranchPickerOpen((v) => !v);
                          }
                        }}
                        disabled={!vcsOwnerRepo.trim().includes("/")}
                      >
                        <span className="vcs-branch-trigger-label">
                          {branchPickerLoading
                            ? <><Spinner size={11} /> Loading branches…</>
                            : vcsBranch
                              ? <><GitBranch size={11} /> {vcsBranch}</>
                              : <span style={{ color: "var(--text-muted)" }}>Select branch (optional)</span>
                          }
                        </span>
                        {vcsBranch ? (
                          <span
                            className="vcs-branch-clear"
                            onMouseDown={(e) => { e.stopPropagation(); setVCSBranch(""); setBranchPickerOpen(false); }}
                            title="Clear branch"
                          ><X size={11} /></span>
                        ) : (
                          <ChevronDown size={13} className={`vcs-branch-chevron${branchPickerOpen ? " vcs-branch-chevron--open" : ""}`} />
                        )}
                      </button>

                      {/* Dropdown */}
                      {branchPickerOpen && !branchPickerLoading && (
                        <div className="branch-picker-dropdown">
                          {branchPickerError ? (
                            <div className="branch-picker-error">{branchPickerError}</div>
                          ) : (
                            <>
                              <div className="branch-picker-search-row">
                                <input
                                  ref={branchFilterRef}
                                  className="branch-picker-filter"
                                  value={branchFilter}
                                  onChange={(e) => setBranchFilter(e.target.value)}
                                  placeholder="Filter branches…"
                                  onKeyDown={(e) => { if (e.key === "Escape") setBranchPickerOpen(false); }}
                                />
                              </div>
                              <div className="branch-picker-list">
                                {repoBranches
                                  .filter((b) => !branchFilter || b.toLowerCase().includes(branchFilter.toLowerCase()))
                                  .map((b) => (
                                    <button
                                      key={b}
                                      type="button"
                                      className={`branch-picker-item${vcsBranch === b ? " branch-picker-item--selected" : ""}`}
                                      onMouseDown={(e) => {
                                        e.preventDefault();
                                        setVCSBranch(b);
                                        setBranchPickerOpen(false);
                                      }}
                                    >
                                      ⎇ {b}
                                    </button>
                                  ))
                                }
                                {repoBranches.filter((b) => !branchFilter || b.toLowerCase().includes(branchFilter.toLowerCase())).length === 0 && (
                                  <div className="branch-picker-empty">No branches match</div>
                                )}
                              </div>
                            </>
                          )}
                        </div>
                      )}
                    </div>

                    <button
                      type="button"
                      className="btn-secondary"
                      onClick={handleAddVCSBranch}
                      style={{ flexShrink: 0 }}
                    >
                      Add
                    </button>
                  </div>

                  {vcsAddError && (
                    <div className="field-error" style={{ marginTop: 4 }}>{vcsAddError}</div>
                  )}

                  <div className="form-hint" style={{ marginTop: 8 }}>
                    Branches load automatically after you stop typing. Leave branch empty to track all open PRs.
                  </div>
                </div>
              </div>
            </div>
          )}

          </div>{/* end modal-scroll-body */}

          <div className="modal-footer">
            <button type="button" className="btn-secondary" onClick={onCancel}>Cancel</button>
            <button type="submit" className="btn-primary" disabled={saving}>
              {saving ? (
                <span className="btn-with-spinner">
                  <Spinner size={14} color="#fff" />
                  Saving…
                </span>
              ) : (
                initial ? "Save Changes" : "Create"
              )}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
