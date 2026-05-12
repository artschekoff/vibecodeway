package store

// InferredRequirementsPacket is the top-level business-context packet produced by the AI
// when the user triggers a handoff generation. It contains structured business requirements
// inferred from Jira, Slack, and GitHub artifacts — no code analysis, no repo dependency.
type InferredRequirementsPacket struct {
	PacketID           string                 `json:"packet_id"`
	ContextID          string                 `json:"context_id"`
	SyncSessionID      *string                `json:"sync_session_id,omitempty"`
	GeneratedAt        string                 `json:"generated_at"`
	ContextWindowStart *string                `json:"context_window_start,omitempty"`
	ContextWindowEnd   *string                `json:"context_window_end,omitempty"`
	Summary            BusinessContextSummary `json:"summary"`
	Requirements       []RequirementInference `json:"inferred_requirements"`
	HandoffCandidate   *HandoffCandidate      `json:"handoff_candidate,omitempty"`
	ExtractionNotes    *string                `json:"extraction_notes,omitempty"`
}

// BusinessContextSummary is the high-level business understanding synthesized from all artifacts.
type BusinessContextSummary struct {
	ProductOrInitiative *string  `json:"product_or_initiative,omitempty"`
	BusinessProblem     *string  `json:"business_problem,omitempty"`
	TargetUsers         []string `json:"target_users"`
	DesiredOutcomes     []string `json:"desired_outcomes"`
	NonGoals            []string `json:"non_goals"`
	InferredTradeoffs   []string `json:"inferred_tradeoffs"`
	GlobalConfidence    float64  `json:"global_confidence"`
}

// RequirementInference is a single AI-inferred business requirement or related business signal.
type RequirementInference struct {
	RequirementID    string                `json:"requirement_id"`
	RequirementType  string                `json:"requirement_type"`
	Title            string                `json:"title"`
	Statement        string                `json:"statement"`
	Status           string                `json:"status"`
	Priority         string                `json:"priority"`
	Confidence       float64               `json:"confidence"`
	AmbiguityNotes   *string               `json:"ambiguity_notes,omitempty"`
	BusinessValue    *string               `json:"business_value,omitempty"`
	SourceArtifacts  []ArtifactRef         `json:"source_artifacts"`
	Evidence         []EvidenceSpan        `json:"evidence"`
	AffectedEntities []EntityRef           `json:"affected_entities"`
	Relations        []RequirementRelation `json:"relations"`
	ValidFrom        *string               `json:"valid_from,omitempty"`
	LastSeenAt       *string               `json:"last_seen_at,omitempty"`
	SupersededBy     *string               `json:"superseded_by,omitempty"`
}

// ArtifactRef is a reference to a source artifact from which a requirement was inferred.
type ArtifactRef struct {
	ArtifactID   string  `json:"artifact_id"`
	ArtifactType string  `json:"artifact_type"`
	SourceSystem string  `json:"source_system"`
	ExternalURL  *string `json:"external_url,omitempty"`
	Title        *string `json:"title,omitempty"`
	CreatedAt    *string `json:"created_at,omitempty"`
	Author       *string `json:"author,omitempty"`
}

// EvidenceSpan is a specific excerpt from an artifact that supports an inference.
type EvidenceSpan struct {
	ArtifactID string  `json:"artifact_id"`
	Quote      string  `json:"quote"`
	Rationale  *string `json:"rationale,omitempty"`
}

// EntityRef is a domain entity affected by an inferred requirement.
type EntityRef struct {
	EntityType string  `json:"entity_type"`
	EntityID   string  `json:"entity_id"`
	Label      *string `json:"label,omitempty"`
}

// RequirementRelation is a directed semantic link between two inferred requirements.
type RequirementRelation struct {
	RelationType        string  `json:"relation_type"`
	TargetRequirementID string  `json:"target_requirement_id"`
	Reason              *string `json:"reason,omitempty"`
}

// HandoffCandidate is a candidate task packet for a downstream agent built from inferred requirements.
type HandoffCandidate struct {
	HandoffTitle           string   `json:"handoff_title"`
	Objective              string   `json:"objective"`
	InScopeRequirements    []string `json:"in_scope_requirements"`
	OutOfScopeRequirements []string `json:"out_of_scope_requirements"`
	KeyBusinessRules       []string `json:"key_business_rules"`
	AcceptanceCriteria     []string `json:"acceptance_criteria"`
	OpenQuestions          []string `json:"open_questions"`
	Confidence             float64  `json:"confidence"`
}
