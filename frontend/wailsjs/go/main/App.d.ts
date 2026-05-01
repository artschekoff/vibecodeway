// Wails bindings for App. DO NOT EDIT.
// This is a minimal generated file for local development.

import {appstore, cliclient, contextlog, jira, jiraservice, jirastore, slackstore, vcsservice} from '../models';

export function AddIssuesToContext(arg1:string,arg2:Array<string>):Promise<number>;

export function AddSlackChannels(arg1:string,arg2:string,arg3:Array<slackstore.SlackChannelInput>):Promise<void>;

export function AnalyzeContextChanges(arg1:string):Promise<contextlog.ContextLog>;

export function ApplyChatSuggestion(arg1:string):Promise<contextlog.ConsolidatedLog>;

export function AuthStatus():Promise<string>;

export function CancelRefresh():Promise<void>;

export function ChatWithContext(arg1:string,arg2:string):Promise<contextlog.ChatMessage>;

export function CheckAccessibility():Promise<boolean>;

export function CommitPendingUploads(arg1:string,arg2:Array<string>):Promise<Array<appstore.ContextUploadedFile>>;

export function CreateContext(arg1:string):Promise<appstore.Context>;

export function CreateContextLog(arg1:string,arg2:string):Promise<contextlog.ContextLog>;

export function CreateOrganization(arg1:string):Promise<appstore.Organization>;

export function DeleteContext(arg1:string):Promise<void>;

export function DeleteContextLog(arg1:string):Promise<void>;

export function DeleteContextText(arg1:string):Promise<void>;

export function DeleteJiraComment(arg1:string):Promise<void>;

export function DeleteJiraIssue(arg1:string):Promise<void>;

export function DeleteOrganization(arg1:string):Promise<void>;

export function DeleteSlackChannel(arg1:string):Promise<void>;

export function DeleteSlackDMConfig(arg1:string):Promise<void>;

export function DeleteSlackThread(arg1:string):Promise<void>;

export function DeleteUploadedFile(arg1:string):Promise<void>;

export function DeleteVCSBranch(arg1:string):Promise<void>;

export function DismissPicker():Promise<void>;

export function DiscardPendingUpload(arg1:string):Promise<void>;

export function GetAISettings():Promise<AISettingsResponse>;

export function GetConsolidatedContextLog(arg1:string):Promise<contextlog.ConsolidatedLog>;

export function GetContextDetail(arg1:string):Promise<jiraservice.ContextDetail>;

export function GetPendingText():Promise<string>;

export function InitializeConsolidatedLog(arg1:string):Promise<contextlog.ConsolidatedLog>;

export function ListAllContexts():Promise<Array<appstore.Context>>;

export function ListContextChat(arg1:string):Promise<Array<contextlog.ChatMessage>>;

export function ListContextLogs(arg1:string):Promise<Array<contextlog.ContextLog>>;

export function ListContextTexts(arg1:string):Promise<Array<appstore.ContextText>>;

export function ListContextsByOrg(arg1:string):Promise<Array<appstore.Context>>;

export function ListContexts():Promise<Array<appstore.Context>>;

export function ListOrganizations():Promise<Array<appstore.Organization>>;

export function ListSlackChannels(arg1:string):Promise<Array<slackstore.SlackChannel>>;

export function ListSlackDMConfigs(arg1:string):Promise<Array<slackstore.SlackDMConfig>>;

export function ListSlackThreads(arg1:string):Promise<Array<slackstore.SlackThread>>;

export function ListSlackWorkspaces():Promise<Array<jiraservice.SlackWorkspaceResult>>;

export function ListUploadedFiles(arg1:string):Promise<Array<appstore.ContextUploadedFile>>;

export function ListVCSBranches(arg1:string):Promise<Array<vcsservice.VCSBranch>>;

export function LoadSlackContext(arg1:string,arg2:string,arg3:string,arg4:string,arg5:string,arg6:string,arg7:string):Promise<void>;

export function OpenAccessibilityPreferences():Promise<void>;

export function PickAndExtractFiles(arg1:string):Promise<UploadBatchResult>;

export function PickAndExtractFilesPreview():Promise<UploadPreviewBatchResult>;

export function PreviewSlackDMs(arg1:string,arg2:string):Promise<string>;

export function RefreshContext(arg1:string):Promise<jiraservice.RefreshResult>;

export function ReloadAllContexts():Promise<Array<jiraservice.ReloadAllResult>>;

export function RemoveSlackChannel(arg1:string):Promise<void>;

export function RemoveSlackDMEntry(arg1:string,arg2:string):Promise<void>;

export function SearchChannelMetadata(arg1:string,arg2:string):Promise<jiraservice.ChannelMetadataResult>;

export function SearchContextIDs(arg1:string):Promise<Array<string>>;

export function SearchJQL(arg1:string):Promise<Array<jira.SearchResult>>;

export function SearchNewIssues(arg1:string):Promise<jiraservice.SearchDiff>;

export function SearchSlackChannels(arg1:string,arg2:string):Promise<Array<jiraservice.SlackChannelResult>>;

export function SetAISettings(arg1:string,arg2:string,arg3:string,arg4:string,arg5:string):Promise<void>;

export function SetContextOrg(arg1:string,arg2:string):Promise<void>;

export function SetContextsOrder(arg1:Array<string>):Promise<void>;

export function SetJiraConfig(arg1:string,arg2:string):Promise<jirastore.JiraProvider>;

export function SetOrganizationsOrder(arg1:Array<string>):Promise<void>;

export function SetSlackConfig(arg1:string,arg2:string,arg3:string):Promise<slackstore.SlackProvider>;

export function SetSlackPassphrase(arg1:string):Promise<void>;

export function SyncAllComments(arg1:string):Promise<number>;

export function ToggleCommentActive(arg1:string):Promise<jirastore.Comment>;

export function ToggleContextStatus(arg1:string):Promise<appstore.Context>;

export function ToggleContextTextActive(arg1:string):Promise<appstore.ContextText>;

export function ToggleIssueActive(arg1:string):Promise<jirastore.Issue>;

export function ToggleSlackChannelActive(arg1:string):Promise<slackstore.SlackChannel>;

export function ToggleSlackDMConfigActive(arg1:string):Promise<slackstore.SlackDMConfig>;

export function ToggleSlackThreadActive(arg1:string):Promise<slackstore.SlackThread>;

export function ToggleUploadedFileActive(arg1:string):Promise<appstore.ContextUploadedFile>;

export function ToggleVCSBranchActive(arg1:string):Promise<vcsservice.VCSBranch>;

export function UpdateContext(arg1:string,arg2:string,arg3:string):Promise<appstore.Context>;

export function UpdateContextLog(arg1:string,arg2:string):Promise<contextlog.ContextLog>;

export function UpdateConsolidatedContextLog(arg1:string,arg2:string):Promise<contextlog.ConsolidatedLog>;

export function UpdateOrganization(arg1:string,arg2:string):Promise<appstore.Organization>;

export function UpsertContextText(arg1:string,arg2:string,arg3:string,arg4:string):Promise<appstore.ContextText>;

export function UpsertSlackDMConfig(arg1:string,arg2:string,arg3:string):Promise<slackstore.SlackDMConfig>;

export function AddVCSBranch(arg1:string,arg2:string,arg3:string,arg4:string,arg5:string):Promise<vcsservice.VCSBranch>;

