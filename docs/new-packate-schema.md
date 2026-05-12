from __future__ import annotations

from datetime import datetime
from enum import Enum
from typing import List, Optional, Dict, Any

from pydantic import BaseModel, Field, ConfigDict


class ArtifactType(str, Enum):
    """
    Type of source artifact from which the AI inferred information.
    """
    task = "task"
    comment = "comment"
    chat_message = "chat_message"
    email = "email"
    meeting_note = "meeting_note"
    document = "document"
    specification = "specification"
    issue = "issue"
    pull_request = "pull_request"
    commit = "commit"
    code_file = "code_file"
    design = "design"
    screenshot = "screenshot"
    other = "other"


class RequirementType(str, Enum):
    """
    High-level type of inferred business requirement.
    """
    goal = "goal"
    business_rule = "business_rule"
    functional_requirement = "functional_requirement"
    acceptance_criterion = "acceptance_criterion"
    constraint = "constraint"
    non_goal = "non_goal"
    dependency = "dependency"
    assumption = "assumption"
    risk = "risk"
    open_question = "open_question"


class InferenceStatus(str, Enum):
    """
    Lifecycle state of an AI-inferred item.
    """
    inferred = "inferred"
    corroborated = "corroborated"
    conflicting = "conflicting"
    stale = "stale"
    rejected = "rejected"
    promoted_to_handoff = "promoted_to_handoff"


class PriorityLevel(str, Enum):
    """
    Relative business importance of an inferred item.
    """
    low = "low"
    medium = "medium"
    high = "high"
    critical = "critical"


class RelationType(str, Enum):
    """
    Semantic relation between two inferred items.
    """
    supports = "supports"
    refines = "refines"
    depends_on = "depends_on"
    conflicts_with = "conflicts_with"
    duplicates = "duplicates"
    derived_from = "derived_from"
    supersedes = "supersedes"
    affects = "affects"


class ArtifactRef(BaseModel):
    """
    Reference to an original artifact that served as source evidence.
    """
    model_config = ConfigDict(extra="forbid")

    artifact_id: str = Field(
        ...,
        description="Stable internal identifier of the artifact, such as task-482 or slack-msg-991."
    )
    artifact_type: ArtifactType = Field(
        ...,
        description="Kind of artifact where the evidence came from."
    )
    source_system: str = Field(
        ...,
        description="External source system, such as jira, slack, github, notion, linear, email, or drive."
    )
    external_url: Optional[str] = Field(
        default=None,
        description="Direct URL to the original artifact, if available."
    )
    title: Optional[str] = Field(
        default=None,
        description="Short human-readable title or label of the artifact."
    )
    created_at: Optional[datetime] = Field(
        default=None,
        description="When the original artifact was created."
    )
    author: Optional[str] = Field(
        default=None,
        description="Author or originator of the artifact, if known."
    )


class EvidenceSpan(BaseModel):
    """
    Specific excerpt, statement, or region inside an artifact that supports an inference.
    """
    model_config = ConfigDict(extra="forbid")

    artifact_id: str = Field(
        ...,
        description="Identifier of the artifact containing the evidence."
    )
    quote: str = Field(
        ...,
        description="Short verbatim excerpt or distilled statement from the artifact that supports the inference."
    )
    start_offset: Optional[int] = Field(
        default=None,
        description="Start character offset of the supporting text in the artifact, if available."
    )
    end_offset: Optional[int] = Field(
        default=None,
        description="End character offset of the supporting text in the artifact, if available."
    )
    rationale: Optional[str] = Field(
        default=None,
        description="Why this evidence supports the inferred requirement."
    )


class EntityRef(BaseModel):
    """
    Domain entity affected by an inferred business requirement.
    """
    model_config = ConfigDict(extra="forbid")

    entity_type: str = Field(
        ...,
        description="Type of affected entity, such as project, feature, workflow, report, user_role, dashboard, or integration."
    )
    entity_id: str = Field(
        ...,
        description="Stable identifier of the affected entity."
    )
    label: Optional[str] = Field(
        default=None,
        description="Human-readable name of the entity."
    )


class RequirementRelation(BaseModel):
    """
    Directed relation between two inferred requirements.
    """
    model_config = ConfigDict(extra="forbid")

    relation_type: RelationType = Field(
        ...,
        description="Type of semantic relation between the source and target requirements."
    )
    target_requirement_id: str = Field(
        ...,
        description="Identifier of the related requirement."
    )
    reason: Optional[str] = Field(
        default=None,
        description="Brief explanation of why this relation exists."
    )


class RequirementInference(BaseModel):
    """
    Single AI-inferred business requirement or related business signal derived from tasks and artifacts.
    """
    model_config = ConfigDict(extra="forbid")

    requirement_id: str = Field(
        ...,
        description="Stable identifier of the inferred requirement object."
    )
    requirement_type: RequirementType = Field(
        ...,
        description="Business meaning category of the inferred item."
    )
    title: str = Field(
        ...,
        description="Short title summarizing the inferred requirement."
    )
    statement: str = Field(
        ...,
        description="Clear, self-contained requirement statement written as a business fact, need, rule, criterion, risk, or question."
    )
    status: InferenceStatus = Field(
        ...,
        description="Current validation status of this inferred item."
    )
    priority: PriorityLevel = Field(
        default=PriorityLevel.medium,
        description="Relative business importance of this inferred item."
    )
    confidence: float = Field(
        ...,
        ge=0.0,
        le=1.0,
        description="Model confidence that the inferred statement is correct and grounded in the source artifacts."
    )
    ambiguity_notes: Optional[str] = Field(
        default=None,
        description="Short note describing uncertainty, ambiguity, or possible alternate interpretations."
    )
    business_value: Optional[str] = Field(
        default=None,
        description="Why this requirement matters from a business perspective."
    )
    source_artifacts: List[ArtifactRef] = Field(
        default_factory=list,
        description="Artifacts from which this requirement was inferred."
    )
    evidence: List[EvidenceSpan] = Field(
        default_factory=list,
        description="Specific excerpts that justify the inferred requirement."
    )
    affected_entities: List[EntityRef] = Field(
        default_factory=list,
        description="Business entities, product areas, or workflows affected by this requirement."
    )
    relations: List[RequirementRelation] = Field(
        default_factory=list,
        description="Links from this inferred item to other inferred items."
    )
    valid_from: Optional[datetime] = Field(
        default=None,
        description="When this requirement appears to have become valid or relevant."
    )
    last_seen_at: Optional[datetime] = Field(
        default=None,
        description="Most recent time supporting evidence for this requirement was observed."
    )
    superseded_by: Optional[str] = Field(
        default=None,
        description="Requirement ID that replaces this one, if it is obsolete."
    )


class BusinessContextSummary(BaseModel):
    """
    High-level business understanding synthesized from the full artifact set.
    """
    model_config = ConfigDict(extra="forbid")

    product_or_initiative: Optional[str] = Field(
        default=None,
        description="Name of the product, project, or business initiative inferred from the artifacts."
    )
    business_problem: Optional[str] = Field(
        default=None,
        description="Main business problem or pain point the work is trying to solve."
    )
    target_users: List[str] = Field(
        default_factory=list,
        description="User roles or stakeholder groups inferred from the artifacts."
    )
    desired_outcomes: List[str] = Field(
        default_factory=list,
        description="Business outcomes the project appears to optimize for."
    )
    non_goals: List[str] = Field(
        default_factory=list,
        description="Things the project or task explicitly should not attempt to do."
    )
    inferred_tradeoffs: List[str] = Field(
        default_factory=list,
        description="Business-level tradeoffs inferred from discussions, such as speed over completeness or visibility over precision."
    )
    global_confidence: float = Field(
        ...,
        ge=0.0,
        le=1.0,
        description="Overall confidence in the synthesized business context."
    )


class HandoffCandidate(BaseModel):
    """
    Candidate task packet for a downstream coding agent, built from inferred business requirements.
    """
    model_config = ConfigDict(extra="forbid")

    handoff_title: str = Field(
        ...,
        description="Short title of the business task to be delegated."
    )
    objective: str = Field(
        ...,
        description="Concrete business objective the downstream agent should satisfy."
    )
    in_scope_requirements: List[str] = Field(
        default_factory=list,
        description="Requirement IDs that are relevant and should be considered in scope for this handoff."
    )
    out_of_scope_requirements: List[str] = Field(
        default_factory=list,
        description="Requirement IDs or business constraints that should explicitly remain out of scope."
    )
    key_business_rules: List[str] = Field(
        default_factory=list,
        description="Short list of mandatory business rules extracted from the inferred requirements."
    )
    acceptance_criteria: List[str] = Field(
        default_factory=list,
        description="Business-facing acceptance criteria that define success for the delegated task."
    )
    open_questions: List[str] = Field(
        default_factory=list,
        description="Unresolved questions that may block confident implementation."
    )
    confidence: float = Field(
        ...,
        ge=0.0,
        le=1.0,
        description="Confidence that this handoff candidate is sufficiently grounded and coherent."
    )


class InferredRequirementsPacket(BaseModel):
    """
    Top-level object returned by the AI after reading tasks and artifacts.
    It contains inferred business context, requirement objects, and an optional handoff candidate.
    """
    model_config = ConfigDict(extra="forbid")

    packet_id: str = Field(
        ...,
        description="Stable identifier of the generated inference packet."
    )
    generated_at: datetime = Field(
        ...,
        description="Timestamp when the packet was generated."
    )
    context_window_start: Optional[datetime] = Field(
        default=None,
        description="Lower bound of the artifact time range used for inference."
    )
    context_window_end: Optional[datetime] = Field(
        default=None,
        description="Upper bound of the artifact time range used for inference."
    )
    summary: BusinessContextSummary = Field(
        ...,
        description="High-level business understanding synthesized from the available artifacts."
    )
    inferred_requirements: List[RequirementInference] = Field(
        default_factory=list,
        description="All inferred business requirements and related business signals."
    )
    handoff_candidate: Optional[HandoffCandidate] = Field(
        default=None,
        description="Optional business handoff object assembled from the most relevant inferred requirements."
    )
    extraction_notes: Optional[str] = Field(
        default=None,
        description="Free-text note about extraction quality, coverage gaps, or major ambiguities."
    )

