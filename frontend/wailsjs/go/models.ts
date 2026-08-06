export namespace agent {
	
	export class Step {
	    tool: string;
	    args: string;
	    result: string;
	
	    static createFrom(source: any = {}) {
	        return new Step(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tool = source["tool"];
	        this.args = source["args"];
	        this.result = source["result"];
	    }
	}
	export class Answer {
	    content: string;
	    steps: Step[];
	
	    static createFrom(source: any = {}) {
	        return new Answer(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.content = source["content"];
	        this.steps = this.convertValues(source["steps"], Step);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class ToolInfo {
	    name: string;
	    description: string;
	
	    static createFrom(source: any = {}) {
	        return new ToolInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	    }
	}

}

export namespace jira {
	
	export class SearchResult {
	    key: string;
	    summary: string;
	    status: string;
	    assignee: string;
	    issuetype: string;
	    priority: string;
	    created: string;
	
	    static createFrom(source: any = {}) {
	        return new SearchResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.summary = source["summary"];
	        this.status = source["status"];
	        this.assignee = source["assignee"];
	        this.issuetype = source["issuetype"];
	        this.priority = source["priority"];
	        this.created = source["created"];
	    }
	}

}

export namespace jirastore {
	
	export class Comment {
	    id: string;
	    issueId: string;
	    jiraId: string;
	    author: string;
	    body: string;
	    jiraCreated: string;
	    createdAt: string;
	    isActive: boolean;
	    viewedAt?: string;
	    aiProcessedAt?: string;
	    messageId?: string;
	
	    static createFrom(source: any = {}) {
	        return new Comment(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.issueId = source["issueId"];
	        this.jiraId = source["jiraId"];
	        this.author = source["author"];
	        this.body = source["body"];
	        this.jiraCreated = source["jiraCreated"];
	        this.createdAt = source["createdAt"];
	        this.isActive = source["isActive"];
	        this.viewedAt = source["viewedAt"];
	        this.aiProcessedAt = source["aiProcessedAt"];
	        this.messageId = source["messageId"];
	    }
	}
	export class Issue {
	    id: string;
	    contextId: string;
	    jiraKey: string;
	    summary: string;
	    status: string;
	    assignee: string;
	    issueType: string;
	    priority: string;
	    description: string;
	    jiraUpdated: string;
	    parentKey: string;
	    createdAt: string;
	    isActive: boolean;
	    viewedAt?: string;
	    aiProcessedAt?: string;
	    messageId?: string;
	    comments?: Comment[];
	
	    static createFrom(source: any = {}) {
	        return new Issue(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.contextId = source["contextId"];
	        this.jiraKey = source["jiraKey"];
	        this.summary = source["summary"];
	        this.status = source["status"];
	        this.assignee = source["assignee"];
	        this.issueType = source["issueType"];
	        this.priority = source["priority"];
	        this.description = source["description"];
	        this.jiraUpdated = source["jiraUpdated"];
	        this.parentKey = source["parentKey"];
	        this.createdAt = source["createdAt"];
	        this.isActive = source["isActive"];
	        this.viewedAt = source["viewedAt"];
	        this.aiProcessedAt = source["aiProcessedAt"];
	        this.messageId = source["messageId"];
	        this.comments = this.convertValues(source["comments"], Comment);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class JiraProvider {
	    id: string;
	    contextId: string;
	    searchQuery: string;
	    baseUrl: string;
	    createdAt: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new JiraProvider(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.contextId = source["contextId"];
	        this.searchQuery = source["searchQuery"];
	        this.baseUrl = source["baseUrl"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}

}

export namespace main {
	
	export class AISettingsResponse {
	    provider: string;
	    hasKey: boolean;
	    hasOpenAIKey: boolean;
	    hasAnthropicKey: boolean;
	    model: string;
	    baseUrl: string;
	    source: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new AISettingsResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.provider = source["provider"];
	        this.hasKey = source["hasKey"];
	        this.hasOpenAIKey = source["hasOpenAIKey"];
	        this.hasAnthropicKey = source["hasAnthropicKey"];
	        this.model = source["model"];
	        this.baseUrl = source["baseUrl"];
	        this.source = source["source"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class AppLogEntry {
	    id: number;
	    level: string;
	    ts: number;
	    caller: string;
	    msg: string;
	    error?: string;
	    createdAt: string;
	
	    static createFrom(source: any = {}) {
	        return new AppLogEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.level = source["level"];
	        this.ts = source["ts"];
	        this.caller = source["caller"];
	        this.msg = source["msg"];
	        this.error = source["error"];
	        this.createdAt = source["createdAt"];
	    }
	}
	export class ApplyActionResult {
	    action: string;
	    ok: boolean;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new ApplyActionResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.action = source["action"];
	        this.ok = source["ok"];
	        this.message = source["message"];
	    }
	}
	export class BlameLine {
	    line: number;
	    commit: string;
	    author: string;
	    date: string;
	    summary: string;
	
	    static createFrom(source: any = {}) {
	        return new BlameLine(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.line = source["line"];
	        this.commit = source["commit"];
	        this.author = source["author"];
	        this.date = source["date"];
	        this.summary = source["summary"];
	    }
	}
	export class EditorRepoChoice {
	    repoId: string;
	    repoName: string;
	    paths: vcsstore.VCSRepoPath[];
	
	    static createFrom(source: any = {}) {
	        return new EditorRepoChoice(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.repoId = source["repoId"];
	        this.repoName = source["repoName"];
	        this.paths = this.convertValues(source["paths"], vcsstore.VCSRepoPath);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class EditorWorktree {
	    repoId: string;
	    repoName: string;
	    branch: string;
	    worktreePath: string;
	
	    static createFrom(source: any = {}) {
	        return new EditorWorktree(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.repoId = source["repoId"];
	        this.repoName = source["repoName"];
	        this.branch = source["branch"];
	        this.worktreePath = source["worktreePath"];
	    }
	}
	export class ContextEditorPrep {
	    ready: boolean;
	    worktrees: EditorWorktree[];
	    choices: EditorRepoChoice[];
	
	    static createFrom(source: any = {}) {
	        return new ContextEditorPrep(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ready = source["ready"];
	        this.worktrees = this.convertValues(source["worktrees"], EditorWorktree);
	        this.choices = this.convertValues(source["choices"], EditorRepoChoice);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	export class FileCommit {
	    hash: string;
	    author: string;
	    date: string;
	    subject: string;
	
	    static createFrom(source: any = {}) {
	        return new FileCommit(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hash = source["hash"];
	        this.author = source["author"];
	        this.date = source["date"];
	        this.subject = source["subject"];
	    }
	}
	export class FileDiff {
	    before: string;
	    after: string;
	
	    static createFrom(source: any = {}) {
	        return new FileDiff(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.before = source["before"];
	        this.after = source["after"];
	    }
	}
	export class MatchedBranch {
	    platform: string;
	    ownerRepo: string;
	    branch: string;
	    label: string;
	
	    static createFrom(source: any = {}) {
	        return new MatchedBranch(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.platform = source["platform"];
	        this.ownerRepo = source["ownerRepo"];
	        this.branch = source["branch"];
	        this.label = source["label"];
	    }
	}
	export class PendingUploadPreview {
	    tempId: string;
	    originalName: string;
	    fileType: string;
	    extractedText: string;
	    fileSize: number;
	
	    static createFrom(source: any = {}) {
	        return new PendingUploadPreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tempId = source["tempId"];
	        this.originalName = source["originalName"];
	        this.fileType = source["fileType"];
	        this.extractedText = source["extractedText"];
	        this.fileSize = source["fileSize"];
	    }
	}
	export class RepoChange {
	    repoId: string;
	    repoName: string;
	    repoPath: string;
	    files: vcs.FileChange[];
	
	    static createFrom(source: any = {}) {
	        return new RepoChange(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.repoId = source["repoId"];
	        this.repoName = source["repoName"];
	        this.repoPath = source["repoPath"];
	        this.files = this.convertValues(source["files"], vcs.FileChange);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ReviewPRResult {
	    contextId: string;
	    contextKey: string;
	    instruction: string;
	
	    static createFrom(source: any = {}) {
	        return new ReviewPRResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.contextId = source["contextId"];
	        this.contextKey = source["contextKey"];
	        this.instruction = source["instruction"];
	    }
	}
	export class UploadBatchResult {
	    files: store.ContextUploadedFile[];
	    errors: string[];
	
	    static createFrom(source: any = {}) {
	        return new UploadBatchResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.files = this.convertValues(source["files"], store.ContextUploadedFile);
	        this.errors = source["errors"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class UploadPreviewBatchResult {
	    previews: PendingUploadPreview[];
	    errors: string[];
	
	    static createFrom(source: any = {}) {
	        return new UploadPreviewBatchResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.previews = this.convertValues(source["previews"], PendingUploadPreview);
	        this.errors = source["errors"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace service {
	
	export class ContextDetail {
	    context: store.Context;
	    jira?: jirastore.JiraProvider;
	    slack?: slackstore.SlackProvider;
	    issues: jirastore.Issue[];
	    slackThreads?: slackstore.SlackThread[];
	    slackChannels?: slackstore.SlackChannel[];
	    slackDMConfigs?: slackstore.SlackDMConfig[];
	    contextTexts?: store.ContextText[];
	    uploadedFiles?: store.ContextUploadedFile[];
	    vcsBranches?: vcsstore.ContextVCSBranch[];
	    pullRequests?: vcsstore.PullRequest[];
	    relations?: store.ContextRelation[];
	
	    static createFrom(source: any = {}) {
	        return new ContextDetail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.context = this.convertValues(source["context"], store.Context);
	        this.jira = this.convertValues(source["jira"], jirastore.JiraProvider);
	        this.slack = this.convertValues(source["slack"], slackstore.SlackProvider);
	        this.issues = this.convertValues(source["issues"], jirastore.Issue);
	        this.slackThreads = this.convertValues(source["slackThreads"], slackstore.SlackThread);
	        this.slackChannels = this.convertValues(source["slackChannels"], slackstore.SlackChannel);
	        this.slackDMConfigs = this.convertValues(source["slackDMConfigs"], slackstore.SlackDMConfig);
	        this.contextTexts = this.convertValues(source["contextTexts"], store.ContextText);
	        this.uploadedFiles = this.convertValues(source["uploadedFiles"], store.ContextUploadedFile);
	        this.vcsBranches = this.convertValues(source["vcsBranches"], vcsstore.ContextVCSBranch);
	        this.pullRequests = this.convertValues(source["pullRequests"], vcsstore.PullRequest);
	        this.relations = this.convertValues(source["relations"], store.ContextRelation);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class RefreshResult {
	    issuesRefreshed: number;
	    newComments: number;
	    threadsRefreshed: number;
	    dmsRefreshed: number;
	    prsRefreshed: number;
	    newPRComments: number;
	    newNotifications: number;
	    syncSessionId?: string;
	
	    static createFrom(source: any = {}) {
	        return new RefreshResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.issuesRefreshed = source["issuesRefreshed"];
	        this.newComments = source["newComments"];
	        this.threadsRefreshed = source["threadsRefreshed"];
	        this.dmsRefreshed = source["dmsRefreshed"];
	        this.prsRefreshed = source["prsRefreshed"];
	        this.newPRComments = source["newPRComments"];
	        this.newNotifications = source["newNotifications"];
	        this.syncSessionId = source["syncSessionId"];
	    }
	}
	export class ReloadAllResult {
	    contextId: string;
	    contextKey: string;
	    newIssues: number;
	    newComments: number;
	    threadsRefreshed: number;
	    dmsRefreshed: number;
	    prsRefreshed: number;
	    newPRComments: number;
	    newNotifications: number;
	    newJiraStatusChanges: number;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new ReloadAllResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.contextId = source["contextId"];
	        this.contextKey = source["contextKey"];
	        this.newIssues = source["newIssues"];
	        this.newComments = source["newComments"];
	        this.threadsRefreshed = source["threadsRefreshed"];
	        this.dmsRefreshed = source["dmsRefreshed"];
	        this.prsRefreshed = source["prsRefreshed"];
	        this.newPRComments = source["newPRComments"];
	        this.newNotifications = source["newNotifications"];
	        this.newJiraStatusChanges = source["newJiraStatusChanges"];
	        this.error = source["error"];
	    }
	}
	export class SearchDiff {
	    contextId: string;
	    new: jira.SearchResult[];
	    existing: jira.SearchResult[];
	
	    static createFrom(source: any = {}) {
	        return new SearchDiff(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.contextId = source["contextId"];
	        this.new = this.convertValues(source["new"], jira.SearchResult);
	        this.existing = this.convertValues(source["existing"], jira.SearchResult);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SlackChannelResult {
	    id: string;
	    name: string;
	    messages: slackstore.SlackMessage[];
	    content?: string;
	
	    static createFrom(source: any = {}) {
	        return new SlackChannelResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.messages = this.convertValues(source["messages"], slackstore.SlackMessage);
	        this.content = source["content"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SlackWorkspaceResult {
	    id: string;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new SlackWorkspaceResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	    }
	}

}

export namespace slackstore {
	
	export class SlackChannel {
	    id: string;
	    contextId: string;
	    workspace: string;
	    channelId: string;
	    channelName: string;
	    startFrom: string;
	    createdAt: string;
	    isActive: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SlackChannel(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.contextId = source["contextId"];
	        this.workspace = source["workspace"];
	        this.channelId = source["channelId"];
	        this.channelName = source["channelName"];
	        this.startFrom = source["startFrom"];
	        this.createdAt = source["createdAt"];
	        this.isActive = source["isActive"];
	    }
	}
	export class SlackMessage {
	    user: string;
	    userId: string;
	    text: string;
	    timestamp: string;
	    rawTs?: string;
	    replies?: SlackMessage[];
	
	    static createFrom(source: any = {}) {
	        return new SlackMessage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.user = source["user"];
	        this.userId = source["userId"];
	        this.text = source["text"];
	        this.timestamp = source["timestamp"];
	        this.rawTs = source["rawTs"];
	        this.replies = this.convertValues(source["replies"], SlackMessage);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SlackChannelInput {
	    channelId: string;
	    channelName: string;
	    startFrom?: string;
	    messages?: SlackMessage[];
	
	    static createFrom(source: any = {}) {
	        return new SlackChannelInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.channelId = source["channelId"];
	        this.channelName = source["channelName"];
	        this.startFrom = source["startFrom"];
	        this.messages = this.convertValues(source["messages"], SlackMessage);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SlackDMConfig {
	    id: string;
	    contextId: string;
	    workspace: string;
	    startFrom: string;
	    content: string;
	    createdAt: string;
	    updatedAt: string;
	    isActive: boolean;
	    viewedAt?: string;
	    aiProcessedAt?: string;
	    messageId?: string;
	
	    static createFrom(source: any = {}) {
	        return new SlackDMConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.contextId = source["contextId"];
	        this.workspace = source["workspace"];
	        this.startFrom = source["startFrom"];
	        this.content = source["content"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	        this.isActive = source["isActive"];
	        this.viewedAt = source["viewedAt"];
	        this.aiProcessedAt = source["aiProcessedAt"];
	        this.messageId = source["messageId"];
	    }
	}
	
	export class SlackProvider {
	    id: string;
	    contextId: string;
	    workspace: string;
	    searchQuery: string;
	    createdAt: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new SlackProvider(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.contextId = source["contextId"];
	        this.workspace = source["workspace"];
	        this.searchQuery = source["searchQuery"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class SlackThread {
	    id: string;
	    contextId: string;
	    slackChannelId?: string;
	    workspace: string;
	    channelId: string;
	    threadTs: string;
	    parentThreadTs?: string;
	    channelName: string;
	    permalink: string;
	    searchQuery: string;
	    content: string;
	    createdAt: string;
	    isActive: boolean;
	    viewedAt?: string;
	    aiProcessedAt?: string;
	    messageId?: string;
	
	    static createFrom(source: any = {}) {
	        return new SlackThread(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.contextId = source["contextId"];
	        this.slackChannelId = source["slackChannelId"];
	        this.workspace = source["workspace"];
	        this.channelId = source["channelId"];
	        this.threadTs = source["threadTs"];
	        this.parentThreadTs = source["parentThreadTs"];
	        this.channelName = source["channelName"];
	        this.permalink = source["permalink"];
	        this.searchQuery = source["searchQuery"];
	        this.content = source["content"];
	        this.createdAt = source["createdAt"];
	        this.isActive = source["isActive"];
	        this.viewedAt = source["viewedAt"];
	        this.aiProcessedAt = source["aiProcessedAt"];
	        this.messageId = source["messageId"];
	    }
	}

}

export namespace store {
	
	export class PromptPlaceholder {
	    name: string;
	    description: string;
	
	    static createFrom(source: any = {}) {
	        return new PromptPlaceholder(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	    }
	}
	export class AIPrompt {
	    id: string;
	    label: string;
	    body: string;
	    defaultBody: string;
	    placeholders: PromptPlaceholder[];
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new AIPrompt(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.body = source["body"];
	        this.defaultBody = source["defaultBody"];
	        this.placeholders = this.convertValues(source["placeholders"], PromptPlaceholder);
	        this.updatedAt = source["updatedAt"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ActivityFeedItem {
	    id: string;
	    contextId: string;
	    contextKey: string;
	    entityType: string;
	    eventType: string;
	    oldValue: string;
	    newValue: string;
	    createdAt: string;
	
	    static createFrom(source: any = {}) {
	        return new ActivityFeedItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.contextId = source["contextId"];
	        this.contextKey = source["contextKey"];
	        this.entityType = source["entityType"];
	        this.eventType = source["eventType"];
	        this.oldValue = source["oldValue"];
	        this.newValue = source["newValue"];
	        this.createdAt = source["createdAt"];
	    }
	}
	export class AppSettings {
	    contextRefetchEnabled: boolean;
	    contextRefetchIntervalMin: number;
	    reviewsRefetchEnabled: boolean;
	    reviewsRefetchIntervalMin: number;
	    terminalAppPath: string;
	    codingAgentApp: string;
	    autoLinkReposOnContextCreate: boolean;
	    editorSettings: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new AppSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.contextRefetchEnabled = source["contextRefetchEnabled"];
	        this.contextRefetchIntervalMin = source["contextRefetchIntervalMin"];
	        this.reviewsRefetchEnabled = source["reviewsRefetchEnabled"];
	        this.reviewsRefetchIntervalMin = source["reviewsRefetchIntervalMin"];
	        this.terminalAppPath = source["terminalAppPath"];
	        this.codingAgentApp = source["codingAgentApp"];
	        this.autoLinkReposOnContextCreate = source["autoLinkReposOnContextCreate"];
	        this.editorSettings = source["editorSettings"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class ArtifactRef {
	    artifact_id: string;
	    artifact_type: string;
	    source_system: string;
	    external_url?: string;
	    title?: string;
	    created_at?: string;
	    author?: string;
	
	    static createFrom(source: any = {}) {
	        return new ArtifactRef(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.artifact_id = source["artifact_id"];
	        this.artifact_type = source["artifact_type"];
	        this.source_system = source["source_system"];
	        this.external_url = source["external_url"];
	        this.title = source["title"];
	        this.created_at = source["created_at"];
	        this.author = source["author"];
	    }
	}
	export class AuthorAssignment {
	    sourceId: string;
	    rawAuthor: string;
	    userId: string;
	
	    static createFrom(source: any = {}) {
	        return new AuthorAssignment(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sourceId = source["sourceId"];
	        this.rawAuthor = source["rawAuthor"];
	        this.userId = source["userId"];
	    }
	}
	export class BoardAssignment {
	    id: string;
	    contextId: string;
	    contextKey: string;
	    name: string;
	    body: string;
	    generatedAt?: string;
	    analyzedThrough?: string;
	    isOutdated: boolean;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new BoardAssignment(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.contextId = source["contextId"];
	        this.contextKey = source["contextKey"];
	        this.name = source["name"];
	        this.body = source["body"];
	        this.generatedAt = source["generatedAt"];
	        this.analyzedThrough = source["analyzedThrough"];
	        this.isOutdated = source["isOutdated"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class BoardBlocker {
	    eventId: string;
	    contextId: string;
	    contextKey: string;
	    author: string;
	    summary: string;
	    timestamp: string;
	    createdAt: string;
	
	    static createFrom(source: any = {}) {
	        return new BoardBlocker(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.eventId = source["eventId"];
	        this.contextId = source["contextId"];
	        this.contextKey = source["contextKey"];
	        this.author = source["author"];
	        this.summary = source["summary"];
	        this.timestamp = source["timestamp"];
	        this.createdAt = source["createdAt"];
	    }
	}
	export class BoardJiraAssignment {
	    key: string;
	    summary: string;
	    status: string;
	    issueType: string;
	    priority: string;
	    created: string;
	    seen: boolean;
	    fetchedAt: string;
	    org: string;
	    contextId?: string;
	    contextKey?: string;
	
	    static createFrom(source: any = {}) {
	        return new BoardJiraAssignment(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.summary = source["summary"];
	        this.status = source["status"];
	        this.issueType = source["issueType"];
	        this.priority = source["priority"];
	        this.created = source["created"];
	        this.seen = source["seen"];
	        this.fetchedAt = source["fetchedAt"];
	        this.org = source["org"];
	        this.contextId = source["contextId"];
	        this.contextKey = source["contextKey"];
	    }
	}
	export class BoardPR {
	    id: string;
	    contextId: string;
	    contextKey: string;
	    repoName: string;
	    prNumber: number;
	    title: string;
	    url: string;
	    headBranch: string;
	    reviewStatus: string;
	    ciStatus: string;
	    mergeable: string;
	    isDraft: boolean;
	    state: string;
	    prUpdatedAt: string;
	    unresolvedCount: number;
	
	    static createFrom(source: any = {}) {
	        return new BoardPR(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.contextId = source["contextId"];
	        this.contextKey = source["contextKey"];
	        this.repoName = source["repoName"];
	        this.prNumber = source["prNumber"];
	        this.title = source["title"];
	        this.url = source["url"];
	        this.headBranch = source["headBranch"];
	        this.reviewStatus = source["reviewStatus"];
	        this.ciStatus = source["ciStatus"];
	        this.mergeable = source["mergeable"];
	        this.isDraft = source["isDraft"];
	        this.state = source["state"];
	        this.prUpdatedAt = source["prUpdatedAt"];
	        this.unresolvedCount = source["unresolvedCount"];
	    }
	}
	export class BoardRecentEvent {
	    eventId: string;
	    contextId: string;
	    contextKey: string;
	    type: string;
	    sourceType: string;
	    author: string;
	    summary: string;
	    createdAt: string;
	
	    static createFrom(source: any = {}) {
	        return new BoardRecentEvent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.eventId = source["eventId"];
	        this.contextId = source["contextId"];
	        this.contextKey = source["contextKey"];
	        this.type = source["type"];
	        this.sourceType = source["sourceType"];
	        this.author = source["author"];
	        this.summary = source["summary"];
	        this.createdAt = source["createdAt"];
	    }
	}
	export class BusinessContextSummary {
	    product_or_initiative?: string;
	    business_problem?: string;
	    target_users: string[];
	    desired_outcomes: string[];
	    non_goals: string[];
	    inferred_tradeoffs: string[];
	    global_confidence: number;
	
	    static createFrom(source: any = {}) {
	        return new BusinessContextSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.product_or_initiative = source["product_or_initiative"];
	        this.business_problem = source["business_problem"];
	        this.target_users = source["target_users"];
	        this.desired_outcomes = source["desired_outcomes"];
	        this.non_goals = source["non_goals"];
	        this.inferred_tradeoffs = source["inferred_tradeoffs"];
	        this.global_confidence = source["global_confidence"];
	    }
	}
	export class Context {
	    id: string;
	    key: string;
	    status: string;
	    orgId: string;
	    createdAt: string;
	    updatedAt: string;
	    issueCount?: number;
	    lastItemAt?: string;
	    unviewedCount?: number;
	    hasJira?: boolean;
	    hasSlack?: boolean;
	    hasVcs?: boolean;
	    hasFiles?: boolean;
	    snapshotStatus: string;
	    snapshotIsStale: boolean;
	    snapshotStatusComment?: string;
	    importance?: number;
	
	    static createFrom(source: any = {}) {
	        return new Context(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.key = source["key"];
	        this.status = source["status"];
	        this.orgId = source["orgId"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	        this.issueCount = source["issueCount"];
	        this.lastItemAt = source["lastItemAt"];
	        this.unviewedCount = source["unviewedCount"];
	        this.hasJira = source["hasJira"];
	        this.hasSlack = source["hasSlack"];
	        this.hasVcs = source["hasVcs"];
	        this.hasFiles = source["hasFiles"];
	        this.snapshotStatus = source["snapshotStatus"];
	        this.snapshotIsStale = source["snapshotIsStale"];
	        this.snapshotStatusComment = source["snapshotStatusComment"];
	        this.importance = source["importance"];
	    }
	}
	export class ContextEventHook {
	    id: string;
	    contextId: string;
	    eventCategory: string;
	    actionType: string;
	    scriptCommand: string;
	    enabled: boolean;
	    createdAt: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new ContextEventHook(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.contextId = source["contextId"];
	        this.eventCategory = source["eventCategory"];
	        this.actionType = source["actionType"];
	        this.scriptCommand = source["scriptCommand"];
	        this.enabled = source["enabled"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class ContextItemMatches {
	    issueIds: string[];
	    channelIds: string[];
	    threadIds: string[];
	    textIds: string[];
	    fileIds: string[];
	    branchIds: string[];
	    prIds: string[];
	
	    static createFrom(source: any = {}) {
	        return new ContextItemMatches(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.issueIds = source["issueIds"];
	        this.channelIds = source["channelIds"];
	        this.threadIds = source["threadIds"];
	        this.textIds = source["textIds"];
	        this.fileIds = source["fileIds"];
	        this.branchIds = source["branchIds"];
	        this.prIds = source["prIds"];
	    }
	}
	export class ContextPlan {
	    id: string;
	    contextId: string;
	    personAssignmentId: string;
	    body: string;
	    createdAt: string;
	
	    static createFrom(source: any = {}) {
	        return new ContextPlan(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.contextId = source["contextId"];
	        this.personAssignmentId = source["personAssignmentId"];
	        this.body = source["body"];
	        this.createdAt = source["createdAt"];
	    }
	}
	export class ContextRelation {
	    id: string;
	    contextId: string;
	    targetId: string;
	    targetKey: string;
	    targetOrgId: string;
	    targetOrgName: string;
	    relationType: string;
	    createdAt: string;
	
	    static createFrom(source: any = {}) {
	        return new ContextRelation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.contextId = source["contextId"];
	        this.targetId = source["targetId"];
	        this.targetKey = source["targetKey"];
	        this.targetOrgId = source["targetOrgId"];
	        this.targetOrgName = source["targetOrgName"];
	        this.relationType = source["relationType"];
	        this.createdAt = source["createdAt"];
	    }
	}
	export class StatusLogEntry {
	    id: string;
	    contextId: string;
	    statusSlug: string;
	    comment: string;
	    source: string;
	    prevId?: string;
	    createdAt: string;
	
	    static createFrom(source: any = {}) {
	        return new StatusLogEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.contextId = source["contextId"];
	        this.statusSlug = source["statusSlug"];
	        this.comment = source["comment"];
	        this.source = source["source"];
	        this.prevId = source["prevId"];
	        this.createdAt = source["createdAt"];
	    }
	}
	export class ContextSnapshot {
	    contextId: string;
	    currentStepDesc: string;
	    currentStepConfidence: string;
	    currentStepFrom: string;
	    currentStepOverridden: boolean;
	    sectionOperational: string;
	    sectionNarrative: string;
	    sectionEngineering: string;
	    status: string;
	    aiNextStepsShort: string;
	    aiNextStepsMid: string;
	    aiNextStepsLong: string;
	    userNextStepsShort: string;
	    userNextStepsMid: string;
	    userNextStepsLong: string;
	    lastAnalyzedSessionId?: string;
	    analyzedAt?: string;
	    analyzedThrough?: string;
	    isStale: boolean;
	    version: number;
	    latestStatusLogEntry?: StatusLogEntry;
	    estimatedHours?: number;
	    estimatedCompletionDate?: string;
	    estimationConfidence: string;
	    estimationBasis: string;
	
	    static createFrom(source: any = {}) {
	        return new ContextSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.contextId = source["contextId"];
	        this.currentStepDesc = source["currentStepDesc"];
	        this.currentStepConfidence = source["currentStepConfidence"];
	        this.currentStepFrom = source["currentStepFrom"];
	        this.currentStepOverridden = source["currentStepOverridden"];
	        this.sectionOperational = source["sectionOperational"];
	        this.sectionNarrative = source["sectionNarrative"];
	        this.sectionEngineering = source["sectionEngineering"];
	        this.status = source["status"];
	        this.aiNextStepsShort = source["aiNextStepsShort"];
	        this.aiNextStepsMid = source["aiNextStepsMid"];
	        this.aiNextStepsLong = source["aiNextStepsLong"];
	        this.userNextStepsShort = source["userNextStepsShort"];
	        this.userNextStepsMid = source["userNextStepsMid"];
	        this.userNextStepsLong = source["userNextStepsLong"];
	        this.lastAnalyzedSessionId = source["lastAnalyzedSessionId"];
	        this.analyzedAt = source["analyzedAt"];
	        this.analyzedThrough = source["analyzedThrough"];
	        this.isStale = source["isStale"];
	        this.version = source["version"];
	        this.latestStatusLogEntry = this.convertValues(source["latestStatusLogEntry"], StatusLogEntry);
	        this.estimatedHours = source["estimatedHours"];
	        this.estimatedCompletionDate = source["estimatedCompletionDate"];
	        this.estimationConfidence = source["estimationConfidence"];
	        this.estimationBasis = source["estimationBasis"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ContextText {
	    id: string;
	    contextId: string;
	    name: string;
	    content: string;
	    createdAt: string;
	    updatedAt: string;
	    isActive: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ContextText(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.contextId = source["contextId"];
	        this.name = source["name"];
	        this.content = source["content"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	        this.isActive = source["isActive"];
	    }
	}
	export class ContextUpdateAction {
	    action: string;
	    eventType?: string;
	    summary?: string;
	    statusSlug?: string;
	    statusComment?: string;
	    jiraKeys?: string[];
	    platform?: string;
	    ownerRepo?: string;
	    branch?: string;
	    repoLabel?: string;
	    targetContextKey?: string;
	    relationType?: string;
	    description: string;
	
	    static createFrom(source: any = {}) {
	        return new ContextUpdateAction(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.action = source["action"];
	        this.eventType = source["eventType"];
	        this.summary = source["summary"];
	        this.statusSlug = source["statusSlug"];
	        this.statusComment = source["statusComment"];
	        this.jiraKeys = source["jiraKeys"];
	        this.platform = source["platform"];
	        this.ownerRepo = source["ownerRepo"];
	        this.branch = source["branch"];
	        this.repoLabel = source["repoLabel"];
	        this.targetContextKey = source["targetContextKey"];
	        this.relationType = source["relationType"];
	        this.description = source["description"];
	    }
	}
	export class ContextUploadedFile {
	    id: string;
	    contextId: string;
	    originalName: string;
	    fileType: string;
	    extractedText: string;
	    fileSize: number;
	    createdAt: string;
	    updatedAt: string;
	    isActive: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ContextUploadedFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.contextId = source["contextId"];
	        this.originalName = source["originalName"];
	        this.fileType = source["fileType"];
	        this.extractedText = source["extractedText"];
	        this.fileSize = source["fileSize"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	        this.isActive = source["isActive"];
	    }
	}
	export class EntityRef {
	    entity_type: string;
	    entity_id: string;
	    label?: string;
	
	    static createFrom(source: any = {}) {
	        return new EntityRef(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.entity_type = source["entity_type"];
	        this.entity_id = source["entity_id"];
	        this.label = source["label"];
	    }
	}
	export class EstimationLog {
	    id: string;
	    contextId: string;
	    recordedAt: string;
	    triggerType: string;
	    triggerDescription: string;
	    deltaHours?: number;
	    estimatedHoursBefore?: number;
	    estimatedHoursAfter?: number;
	    estimatedCompletionBefore?: string;
	    estimatedCompletionAfter?: string;
	    estimationBasis: string;
	
	    static createFrom(source: any = {}) {
	        return new EstimationLog(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.contextId = source["contextId"];
	        this.recordedAt = source["recordedAt"];
	        this.triggerType = source["triggerType"];
	        this.triggerDescription = source["triggerDescription"];
	        this.deltaHours = source["deltaHours"];
	        this.estimatedHoursBefore = source["estimatedHoursBefore"];
	        this.estimatedHoursAfter = source["estimatedHoursAfter"];
	        this.estimatedCompletionBefore = source["estimatedCompletionBefore"];
	        this.estimatedCompletionAfter = source["estimatedCompletionAfter"];
	        this.estimationBasis = source["estimationBasis"];
	    }
	}
	export class Event {
	    id: string;
	    contextId: string;
	    syncSessionId?: string;
	    type: string;
	    timestamp: string;
	    sourceType: string;
	    sourceId: string;
	    author: string;
	    summary: string;
	    rawReference: string;
	    createdAt: string;
	
	    static createFrom(source: any = {}) {
	        return new Event(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.contextId = source["contextId"];
	        this.syncSessionId = source["syncSessionId"];
	        this.type = source["type"];
	        this.timestamp = source["timestamp"];
	        this.sourceType = source["sourceType"];
	        this.sourceId = source["sourceId"];
	        this.author = source["author"];
	        this.summary = source["summary"];
	        this.rawReference = source["rawReference"];
	        this.createdAt = source["createdAt"];
	    }
	}
	export class EvidenceSpan {
	    artifact_id: string;
	    quote: string;
	    rationale?: string;
	
	    static createFrom(source: any = {}) {
	        return new EvidenceSpan(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.artifact_id = source["artifact_id"];
	        this.quote = source["quote"];
	        this.rationale = source["rationale"];
	    }
	}
	export class GlobalEventHook {
	    id: string;
	    eventCategory: string;
	    actionType: string;
	    scriptCommand: string;
	    enabled: boolean;
	    createdAt: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new GlobalEventHook(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.eventCategory = source["eventCategory"];
	        this.actionType = source["actionType"];
	        this.scriptCommand = source["scriptCommand"];
	        this.enabled = source["enabled"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class HandoffCandidate {
	    handoff_title: string;
	    objective: string;
	    in_scope_requirements: string[];
	    out_of_scope_requirements: string[];
	    key_business_rules: string[];
	    acceptance_criteria: string[];
	    open_questions: string[];
	    confidence: number;
	
	    static createFrom(source: any = {}) {
	        return new HandoffCandidate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.handoff_title = source["handoff_title"];
	        this.objective = source["objective"];
	        this.in_scope_requirements = source["in_scope_requirements"];
	        this.out_of_scope_requirements = source["out_of_scope_requirements"];
	        this.key_business_rules = source["key_business_rules"];
	        this.acceptance_criteria = source["acceptance_criteria"];
	        this.open_questions = source["open_questions"];
	        this.confidence = source["confidence"];
	    }
	}
	export class RequirementRelation {
	    relation_type: string;
	    target_requirement_id: string;
	    reason?: string;
	
	    static createFrom(source: any = {}) {
	        return new RequirementRelation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.relation_type = source["relation_type"];
	        this.target_requirement_id = source["target_requirement_id"];
	        this.reason = source["reason"];
	    }
	}
	export class RequirementInference {
	    requirement_id: string;
	    requirement_type: string;
	    title: string;
	    statement: string;
	    status: string;
	    priority: string;
	    confidence: number;
	    ambiguity_notes?: string;
	    business_value?: string;
	    source_artifacts: ArtifactRef[];
	    evidence: EvidenceSpan[];
	    affected_entities: EntityRef[];
	    relations: RequirementRelation[];
	    valid_from?: string;
	    last_seen_at?: string;
	    superseded_by?: string;
	
	    static createFrom(source: any = {}) {
	        return new RequirementInference(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.requirement_id = source["requirement_id"];
	        this.requirement_type = source["requirement_type"];
	        this.title = source["title"];
	        this.statement = source["statement"];
	        this.status = source["status"];
	        this.priority = source["priority"];
	        this.confidence = source["confidence"];
	        this.ambiguity_notes = source["ambiguity_notes"];
	        this.business_value = source["business_value"];
	        this.source_artifacts = this.convertValues(source["source_artifacts"], ArtifactRef);
	        this.evidence = this.convertValues(source["evidence"], EvidenceSpan);
	        this.affected_entities = this.convertValues(source["affected_entities"], EntityRef);
	        this.relations = this.convertValues(source["relations"], RequirementRelation);
	        this.valid_from = source["valid_from"];
	        this.last_seen_at = source["last_seen_at"];
	        this.superseded_by = source["superseded_by"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class InferredRequirementsPacket {
	    packet_id: string;
	    context_id: string;
	    sync_session_id?: string;
	    generated_at: string;
	    context_window_start?: string;
	    context_window_end?: string;
	    summary: BusinessContextSummary;
	    inferred_requirements: RequirementInference[];
	    handoff_candidate?: HandoffCandidate;
	    extraction_notes?: string;
	
	    static createFrom(source: any = {}) {
	        return new InferredRequirementsPacket(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.packet_id = source["packet_id"];
	        this.context_id = source["context_id"];
	        this.sync_session_id = source["sync_session_id"];
	        this.generated_at = source["generated_at"];
	        this.context_window_start = source["context_window_start"];
	        this.context_window_end = source["context_window_end"];
	        this.summary = this.convertValues(source["summary"], BusinessContextSummary);
	        this.inferred_requirements = this.convertValues(source["inferred_requirements"], RequirementInference);
	        this.handoff_candidate = this.convertValues(source["handoff_candidate"], HandoffCandidate);
	        this.extraction_notes = source["extraction_notes"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Keybinding {
	    id: string;
	    command: string;
	    label: string;
	    description: string;
	    modifierKeys: string;
	    keyCode: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new Keybinding(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.command = source["command"];
	        this.label = source["label"];
	        this.description = source["description"];
	        this.modifierKeys = source["modifierKeys"];
	        this.keyCode = source["keyCode"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class Message {
	    id: string;
	    authorId?: string;
	    authorDisplayName?: string;
	    rawAuthor: string;
	    contextId?: string;
	    organizationId?: string;
	    parentId?: string;
	    title: string;
	    body: string;
	    sourceId: string;
	    sourceLabel?: string;
	    externalId: string;
	    createdAt: string;
	    updatedAt: string;
	    insertedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new Message(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.authorId = source["authorId"];
	        this.authorDisplayName = source["authorDisplayName"];
	        this.rawAuthor = source["rawAuthor"];
	        this.contextId = source["contextId"];
	        this.organizationId = source["organizationId"];
	        this.parentId = source["parentId"];
	        this.title = source["title"];
	        this.body = source["body"];
	        this.sourceId = source["sourceId"];
	        this.sourceLabel = source["sourceLabel"];
	        this.externalId = source["externalId"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	        this.insertedAt = source["insertedAt"];
	    }
	}
	export class MyBoardData {
	    blockers: BoardBlocker[];
	    myAssignments: BoardAssignment[];
	    myOpenPRs: BoardPR[];
	    myAssignedJiraIssues: BoardJiraAssignment[];
	    recentActivity: BoardRecentEvent[];
	
	    static createFrom(source: any = {}) {
	        return new MyBoardData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.blockers = this.convertValues(source["blockers"], BoardBlocker);
	        this.myAssignments = this.convertValues(source["myAssignments"], BoardAssignment);
	        this.myOpenPRs = this.convertValues(source["myOpenPRs"], BoardPR);
	        this.myAssignedJiraIssues = this.convertValues(source["myAssignedJiraIssues"], BoardJiraAssignment);
	        this.recentActivity = this.convertValues(source["recentActivity"], BoardRecentEvent);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Organization {
	    id: string;
	    name: string;
	    sortOrder: number;
	    createdAt: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new Organization(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.sortOrder = source["sortOrder"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class PersonAssignment {
	    id: string;
	    contextId: string;
	    name: string;
	    body: string;
	    generatedAt?: string;
	    analyzedThrough?: string;
	    isOutdated: boolean;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new PersonAssignment(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.contextId = source["contextId"];
	        this.name = source["name"];
	        this.body = source["body"];
	        this.generatedAt = source["generatedAt"];
	        this.analyzedThrough = source["analyzedThrough"];
	        this.isOutdated = source["isOutdated"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class PersonAssignmentVersion {
	    id: string;
	    personAssignmentId: string;
	    body: string;
	    llmPrompt?: string;
	    isActive: boolean;
	    createdAt: string;
	
	    static createFrom(source: any = {}) {
	        return new PersonAssignmentVersion(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.personAssignmentId = source["personAssignmentId"];
	        this.body = source["body"];
	        this.llmPrompt = source["llmPrompt"];
	        this.isActive = source["isActive"];
	        this.createdAt = source["createdAt"];
	    }
	}
	
	export class ReasoningChainStep {
	    id: string;
	    entityType: string;
	    entityId: string;
	    stepIndex: number;
	    sourceType: string;
	    messageId?: string;
	    messageContent: string;
	    userInstruction?: string;
	    inputState: string;
	    outputState: string;
	    compressedSummary: string;
	    eventsSnapshot: string;
	    promptVersion: string;
	    llmPrompt?: string;
	    qualityScore?: number;
	    qualityNotes?: string;
	    isCurrent: boolean;
	    createdAt: string;
	
	    static createFrom(source: any = {}) {
	        return new ReasoningChainStep(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.entityType = source["entityType"];
	        this.entityId = source["entityId"];
	        this.stepIndex = source["stepIndex"];
	        this.sourceType = source["sourceType"];
	        this.messageId = source["messageId"];
	        this.messageContent = source["messageContent"];
	        this.userInstruction = source["userInstruction"];
	        this.inputState = source["inputState"];
	        this.outputState = source["outputState"];
	        this.compressedSummary = source["compressedSummary"];
	        this.eventsSnapshot = source["eventsSnapshot"];
	        this.promptVersion = source["promptVersion"];
	        this.llmPrompt = source["llmPrompt"];
	        this.qualityScore = source["qualityScore"];
	        this.qualityNotes = source["qualityNotes"];
	        this.isCurrent = source["isCurrent"];
	        this.createdAt = source["createdAt"];
	    }
	}
	
	
	
	export class TerminalCommand {
	    id: string;
	    title: string;
	    command: string;
	    sendEnter: boolean;
	    createdAt: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new TerminalCommand(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.command = source["command"];
	        this.sendEnter = source["sendEnter"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class UnresolvedAuthor {
	    sourceId: string;
	    sourceLabel: string;
	    rawAuthor: string;
	    messageCount: number;
	
	    static createFrom(source: any = {}) {
	        return new UnresolvedAuthor(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sourceId = source["sourceId"];
	        this.sourceLabel = source["sourceLabel"];
	        this.rawAuthor = source["rawAuthor"];
	        this.messageCount = source["messageCount"];
	    }
	}
	export class UnviewedBreakdown {
	    issues: number;
	    comments: number;
	    threads: number;
	    dms: number;
	    prs: number;
	    prComments: number;
	    notifications: number;
	    total: number;
	
	    static createFrom(source: any = {}) {
	        return new UnviewedBreakdown(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.issues = source["issues"];
	        this.comments = source["comments"];
	        this.threads = source["threads"];
	        this.dms = source["dms"];
	        this.prs = source["prs"];
	        this.prComments = source["prComments"];
	        this.notifications = source["notifications"];
	        this.total = source["total"];
	    }
	}
	export class User {
	    id: string;
	    orgId?: string;
	    displayName: string;
	    email: string;
	    avatarUrl: string;
	    createdAt: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new User(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.orgId = source["orgId"];
	        this.displayName = source["displayName"];
	        this.email = source["email"];
	        this.avatarUrl = source["avatarUrl"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class WorkflowStatus {
	    id: string;
	    slug: string;
	    label: string;
	    aiHint: string;
	    color: string;
	    isSystem: boolean;
	    createdAt: string;
	
	    static createFrom(source: any = {}) {
	        return new WorkflowStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.slug = source["slug"];
	        this.label = source["label"];
	        this.aiHint = source["aiHint"];
	        this.color = source["color"];
	        this.isSystem = source["isSystem"];
	        this.createdAt = source["createdAt"];
	    }
	}

}

export namespace vcs {
	
	export class DeviceCodeInfo {
	    userCode: string;
	    verificationUri: string;
	    deviceCode: string;
	    expiresIn: number;
	    interval: number;
	
	    static createFrom(source: any = {}) {
	        return new DeviceCodeInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.userCode = source["userCode"];
	        this.verificationUri = source["verificationUri"];
	        this.deviceCode = source["deviceCode"];
	        this.expiresIn = source["expiresIn"];
	        this.interval = source["interval"];
	    }
	}
	export class FileChange {
	    path: string;
	    status: string;
	    diff: string;
	    indexStatus: string;
	    workStatus: string;
	    commitStatus: string;
	
	    static createFrom(source: any = {}) {
	        return new FileChange(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.status = source["status"];
	        this.diff = source["diff"];
	        this.indexStatus = source["indexStatus"];
	        this.workStatus = source["workStatus"];
	        this.commitStatus = source["commitStatus"];
	    }
	}
	export class LocalBranch {
	    name: string;
	    isCurrent: boolean;
	    isGone: boolean;
	
	    static createFrom(source: any = {}) {
	        return new LocalBranch(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.isCurrent = source["isCurrent"];
	        this.isGone = source["isGone"];
	    }
	}

}

export namespace vcsstore {
	
	export class VCSRepo {
	    id: string;
	    platform: string;
	    owner: string;
	    name: string;
	    localPath: string;
	    label: string;
	    description: string;
	    onCheckoutCommand: string;
	    createdAt: string;
	
	    static createFrom(source: any = {}) {
	        return new VCSRepo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.platform = source["platform"];
	        this.owner = source["owner"];
	        this.name = source["name"];
	        this.localPath = source["localPath"];
	        this.label = source["label"];
	        this.description = source["description"];
	        this.onCheckoutCommand = source["onCheckoutCommand"];
	        this.createdAt = source["createdAt"];
	    }
	}
	export class ContextVCSBranch {
	    id: string;
	    contextId: string;
	    repoId: string;
	    branch: string;
	    label: string;
	    createdAt: string;
	    repo?: VCSRepo;
	
	    static createFrom(source: any = {}) {
	        return new ContextVCSBranch(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.contextId = source["contextId"];
	        this.repoId = source["repoId"];
	        this.branch = source["branch"];
	        this.label = source["label"];
	        this.createdAt = source["createdAt"];
	        this.repo = this.convertValues(source["repo"], VCSRepo);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PRComment {
	    id: string;
	    prId: string;
	    externalId: string;
	    parentId?: string;
	    commentType: string;
	    author: string;
	    body: string;
	    path?: string;
	    line?: number;
	    isResolved: boolean;
	    inReplyToId?: string;
	    commentCreatedAt: string;
	    commentUpdatedAt: string;
	    viewedAt?: string;
	    aiProcessedAt?: string;
	    messageId?: string;
	    replies?: PRComment[];
	
	    static createFrom(source: any = {}) {
	        return new PRComment(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.prId = source["prId"];
	        this.externalId = source["externalId"];
	        this.parentId = source["parentId"];
	        this.commentType = source["commentType"];
	        this.author = source["author"];
	        this.body = source["body"];
	        this.path = source["path"];
	        this.line = source["line"];
	        this.isResolved = source["isResolved"];
	        this.inReplyToId = source["inReplyToId"];
	        this.commentCreatedAt = source["commentCreatedAt"];
	        this.commentUpdatedAt = source["commentUpdatedAt"];
	        this.viewedAt = source["viewedAt"];
	        this.aiProcessedAt = source["aiProcessedAt"];
	        this.messageId = source["messageId"];
	        this.replies = this.convertValues(source["replies"], PRComment);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PullRequest {
	    id: string;
	    repoId: string;
	    branchId: string;
	    prNumber: number;
	    title: string;
	    author: string;
	    state: string;
	    isDraft: boolean;
	    reviewStatus: string;
	    url: string;
	    headBranch: string;
	    mergeable: string;
	    ciStatus: string;
	    prCreatedAt: string;
	    prUpdatedAt: string;
	    fetchedAt: string;
	    viewedAt?: string;
	    aiProcessedAt?: string;
	    messageId?: string;
	    comments?: PRComment[];
	
	    static createFrom(source: any = {}) {
	        return new PullRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.repoId = source["repoId"];
	        this.branchId = source["branchId"];
	        this.prNumber = source["prNumber"];
	        this.title = source["title"];
	        this.author = source["author"];
	        this.state = source["state"];
	        this.isDraft = source["isDraft"];
	        this.reviewStatus = source["reviewStatus"];
	        this.url = source["url"];
	        this.headBranch = source["headBranch"];
	        this.mergeable = source["mergeable"];
	        this.ciStatus = source["ciStatus"];
	        this.prCreatedAt = source["prCreatedAt"];
	        this.prUpdatedAt = source["prUpdatedAt"];
	        this.fetchedAt = source["fetchedAt"];
	        this.viewedAt = source["viewedAt"];
	        this.aiProcessedAt = source["aiProcessedAt"];
	        this.messageId = source["messageId"];
	        this.comments = this.convertValues(source["comments"], PRComment);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ReviewerDecision {
	    login: string;
	    state: string;
	    submittedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new ReviewerDecision(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.login = source["login"];
	        this.state = source["state"];
	        this.submittedAt = source["submittedAt"];
	    }
	}
	export class ReviewRequest {
	    id: string;
	    repoId: string;
	    prNumber: number;
	    prTitle: string;
	    prAuthor: string;
	    prUrl: string;
	    reviewStatus: string;
	    headBranch: string;
	    requestedAt: string;
	    fetchedAt: string;
	    mergeable: string;
	    reviewedAt?: string;
	    dismissedAt?: string;
	    viewedAt?: string;
	    reviewers?: ReviewerDecision[];
	
	    static createFrom(source: any = {}) {
	        return new ReviewRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.repoId = source["repoId"];
	        this.prNumber = source["prNumber"];
	        this.prTitle = source["prTitle"];
	        this.prAuthor = source["prAuthor"];
	        this.prUrl = source["prUrl"];
	        this.reviewStatus = source["reviewStatus"];
	        this.headBranch = source["headBranch"];
	        this.requestedAt = source["requestedAt"];
	        this.fetchedAt = source["fetchedAt"];
	        this.mergeable = source["mergeable"];
	        this.reviewedAt = source["reviewedAt"];
	        this.dismissedAt = source["dismissedAt"];
	        this.viewedAt = source["viewedAt"];
	        this.reviewers = this.convertValues(source["reviewers"], ReviewerDecision);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class VCSAuth {
	    id: string;
	    platform: string;
	    username: string;
	    avatarUrl: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new VCSAuth(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.platform = source["platform"];
	        this.username = source["username"];
	        this.avatarUrl = source["avatarUrl"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class VCSPlatformInfo {
	    platform: string;
	    label: string;
	    enabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new VCSPlatformInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.platform = source["platform"];
	        this.label = source["label"];
	        this.enabled = source["enabled"];
	    }
	}
	
	export class VCSRepoPath {
	    id: string;
	    repoId: string;
	    path: string;
	    createdAt: string;
	
	    static createFrom(source: any = {}) {
	        return new VCSRepoPath(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.repoId = source["repoId"];
	        this.path = source["path"];
	        this.createdAt = source["createdAt"];
	    }
	}
	export class VCSWorkspace {
	    id: string;
	    repoPathId: string;
	    repoId: string;
	    branch: string;
	    worktreePath: string;
	    status: string;
	    createdAt: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new VCSWorkspace(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.repoPathId = source["repoPathId"];
	        this.repoId = source["repoId"];
	        this.branch = source["branch"];
	        this.worktreePath = source["worktreePath"];
	        this.status = source["status"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}

}

