export namespace contextlog {
	
	export class ChatMessage {
	    id: string;
	    contextId: string;
	    role: string;
	    content: string;
	    proposedConsolidatedMd?: string;
	    appliedAt?: string;
	    createdAt: string;
	
	    static createFrom(source: any = {}) {
	        return new ChatMessage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.contextId = source["contextId"];
	        this.role = source["role"];
	        this.content = source["content"];
	        this.proposedConsolidatedMd = source["proposedConsolidatedMd"];
	        this.appliedAt = source["appliedAt"];
	        this.createdAt = source["createdAt"];
	    }
	}
	export class ConsolidatedLog {
	    contextId: string;
	    contentMd: string;
	    version: number;
	    updatedBy: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new ConsolidatedLog(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.contextId = source["contextId"];
	        this.contentMd = source["contentMd"];
	        this.version = source["version"];
	        this.updatedBy = source["updatedBy"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class ContextLog {
	    id: string;
	    contextId: string;
	    trigger: string;
	    source?: string;
	    model?: string;
	    promptTokens?: number;
	    completionTokens?: number;
	    diffJson?: string;
	    summaryMd: string;
	    isManual: boolean;
	    editedAt?: string;
	    createdAt: string;
	
	    static createFrom(source: any = {}) {
	        return new ContextLog(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.contextId = source["contextId"];
	        this.trigger = source["trigger"];
	        this.source = source["source"];
	        this.model = source["model"];
	        this.promptTokens = source["promptTokens"];
	        this.completionTokens = source["completionTokens"];
	        this.diffJson = source["diffJson"];
	        this.summaryMd = source["summaryMd"];
	        this.isManual = source["isManual"];
	        this.editedAt = source["editedAt"];
	        this.createdAt = source["createdAt"];
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
	    description: string;
	    jiraUpdated: string;
	    createdAt: string;
	    isActive: boolean;
	    viewedAt?: string;
	    aiProcessedAt?: string;
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
	        this.description = source["description"];
	        this.jiraUpdated = source["jiraUpdated"];
	        this.createdAt = source["createdAt"];
	        this.isActive = source["isActive"];
	        this.viewedAt = source["viewedAt"];
	        this.aiProcessedAt = source["aiProcessedAt"];
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
	    hasKey: boolean;
	    model: string;
	    baseUrl: string;
	    source: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new AISettingsResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hasKey = source["hasKey"];
	        this.model = source["model"];
	        this.baseUrl = source["baseUrl"];
	        this.source = source["source"];
	        this.updatedAt = source["updatedAt"];
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
	export class WorkflowSummary {
	    aiProposedStatus: string;
	    aiProposedComment?: string;
	    aiProposedNextStep?: string;
	    aiProposedAt?: string;
	    userStatusOverride?: string;
	    userCommentOverride?: string;
	    userNextStepOverride?: string;
	    userOverrideUpdatedAt?: string;
	    effectiveStatus: string;
	    effectiveComment: string;
	    effectiveNextStep: string;
	
	    static createFrom(source: any = {}) {
	        return new WorkflowSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.aiProposedStatus = source["aiProposedStatus"];
	        this.aiProposedComment = source["aiProposedComment"];
	        this.aiProposedNextStep = source["aiProposedNextStep"];
	        this.aiProposedAt = source["aiProposedAt"];
	        this.userStatusOverride = source["userStatusOverride"];
	        this.userCommentOverride = source["userCommentOverride"];
	        this.userNextStepOverride = source["userNextStepOverride"];
	        this.userOverrideUpdatedAt = source["userOverrideUpdatedAt"];
	        this.effectiveStatus = source["effectiveStatus"];
	        this.effectiveComment = source["effectiveComment"];
	        this.effectiveNextStep = source["effectiveNextStep"];
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
	export class slackMessage {
	    user: string;
	    userId: string;
	    text: string;
	    timestamp: string;
	    replies?: slackMessage[];
	
	    static createFrom(source: any = {}) {
	        return new slackMessage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.user = source["user"];
	        this.userId = source["userId"];
	        this.text = source["text"];
	        this.timestamp = source["timestamp"];
	        this.replies = this.convertValues(source["replies"], slackMessage);
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
	    messages: slackMessage[];
	    content?: string;
	
	    static createFrom(source: any = {}) {
	        return new SlackChannelResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.messages = this.convertValues(source["messages"], slackMessage);
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
	export class SlackChannelInput {
	    channelId: string;
	    channelName: string;
	    startFrom?: string;
	    content?: string;
	
	    static createFrom(source: any = {}) {
	        return new SlackChannelInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.channelId = source["channelId"];
	        this.channelName = source["channelName"];
	        this.startFrom = source["startFrom"];
	        this.content = source["content"];
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
	    channelName: string;
	    permalink: string;
	    searchQuery: string;
	    content: string;
	    createdAt: string;
	    isActive: boolean;
	    viewedAt?: string;
	    aiProcessedAt?: string;
	
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
	        this.channelName = source["channelName"];
	        this.permalink = source["permalink"];
	        this.searchQuery = source["searchQuery"];
	        this.content = source["content"];
	        this.createdAt = source["createdAt"];
	        this.isActive = source["isActive"];
	        this.viewedAt = source["viewedAt"];
	        this.aiProcessedAt = source["aiProcessedAt"];
	    }
	}

}

export namespace store {
	
	export class AppSettings {
	    contextRefetchEnabled: boolean;
	    contextRefetchIntervalMin: number;
	    reviewsRefetchEnabled: boolean;
	    reviewsRefetchIntervalMin: number;
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
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class Context {
	    id: string;
	    key: string;
	    status: string;
	    orgId?: string;
	    createdAt: string;
	    updatedAt: string;
	    issueCount?: number;
	    lastItemAt?: string;
	    unviewedCount?: number;
	    hasJira?: boolean;
	    hasSlack?: boolean;
	    hasVcs?: boolean;
	    hasFiles?: boolean;
	    aiProposedStatus: string;
	    aiProposedComment?: string;
	    aiProposedNextStep?: string;
	    aiProposedAt?: string;
	    userStatusOverride?: string;
	    userCommentOverride?: string;
	    userNextStepOverride?: string;
	    userOverrideUpdatedAt?: string;
	
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
	        this.aiProposedStatus = source["aiProposedStatus"];
	        this.aiProposedComment = source["aiProposedComment"];
	        this.aiProposedNextStep = source["aiProposedNextStep"];
	        this.aiProposedAt = source["aiProposedAt"];
	        this.userStatusOverride = source["userStatusOverride"];
	        this.userCommentOverride = source["userCommentOverride"];
	        this.userNextStepOverride = source["userNextStepOverride"];
	        this.userOverrideUpdatedAt = source["userOverrideUpdatedAt"];
	    }
	}
	export class ContextRelation {
	    id: string;
	    contextId: string;
	    targetId: string;
	    targetKey: string;
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
	        this.relationType = source["relationType"];
	        this.createdAt = source["createdAt"];
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
	export class OpenQuestion {
	    id: string;
	    contextId: string;
	    question: string;
	    status: string;
	    answer?: string;
	    source: string;
	    createdAt: string;
	    updatedAt: string;
	    closedAt?: string;
	
	    static createFrom(source: any = {}) {
	        return new OpenQuestion(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.contextId = source["contextId"];
	        this.question = source["question"];
	        this.status = source["status"];
	        this.answer = source["answer"];
	        this.source = source["source"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	        this.closedAt = source["closedAt"];
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
	export class WorkflowProposal {
	    id: string;
	    contextId: string;
	    proposedStatus: string;
	    proposedComment?: string;
	    proposedNextStep?: string;
	    userStatusOverride?: string;
	    userCommentOverride?: string;
	    userNextStepOverride?: string;
	    prevUserNextStep?: string;
	    resolvedAt?: string;
	    updatedAt: string;
	    createdAt: string;
	
	    static createFrom(source: any = {}) {
	        return new WorkflowProposal(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.contextId = source["contextId"];
	        this.proposedStatus = source["proposedStatus"];
	        this.proposedComment = source["proposedComment"];
	        this.proposedNextStep = source["proposedNextStep"];
	        this.userStatusOverride = source["userStatusOverride"];
	        this.userCommentOverride = source["userCommentOverride"];
	        this.userNextStepOverride = source["userNextStepOverride"];
	        this.prevUserNextStep = source["prevUserNextStep"];
	        this.resolvedAt = source["resolvedAt"];
	        this.updatedAt = source["updatedAt"];
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
	export class ReviewRequest {
	    id: string;
	    repoId: string;
	    prNumber: number;
	    prTitle: string;
	    prAuthor: string;
	    prUrl: string;
	    reviewStatus: string;
	    requestedAt: string;
	    fetchedAt: string;
	    reviewedAt?: string;
	    dismissedAt?: string;
	
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
	        this.requestedAt = source["requestedAt"];
	        this.fetchedAt = source["fetchedAt"];
	        this.reviewedAt = source["reviewedAt"];
	        this.dismissedAt = source["dismissedAt"];
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

}

