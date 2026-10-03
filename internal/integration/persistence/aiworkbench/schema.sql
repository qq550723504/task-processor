CREATE SCHEMA IF NOT EXISTS ai_workbench;

CREATE TABLE IF NOT EXISTS ai_workbench.conversations (
    id uuid PRIMARY KEY,
    organization_id text NOT NULL CHECK (octet_length(organization_id) BETWEEN 1 AND 128),
    owner_user_id text NOT NULL CHECK (octet_length(owner_user_id) BETWEEN 1 AND 128),
    title text NOT NULL DEFAULT '' CHECK (octet_length(title) <= 256),
    favorite boolean NOT NULL DEFAULT false,
    lifecycle text NOT NULL CHECK (lifecycle IN ('ACTIVE', 'ARCHIVED')),
    metadata_revision bigint NOT NULL CHECK (metadata_revision > 0),
    next_sequence bigint NOT NULL CHECK (next_sequence > 0),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    UNIQUE (id, organization_id, owner_user_id)
);
CREATE INDEX IF NOT EXISTS ai_workbench_conversations_owner_recent
    ON ai_workbench.conversations
    (organization_id, owner_user_id, lifecycle, updated_at DESC, id);

CREATE TABLE IF NOT EXISTS ai_workbench.messages (
    id uuid PRIMARY KEY,
    organization_id text NOT NULL,
    owner_user_id text NOT NULL,
    conversation_id uuid NOT NULL,
    sequence bigint NOT NULL CHECK (sequence > 0),
    author_kind text NOT NULL CHECK (author_kind IN ('USER', 'ASSISTANT')),
    content text NOT NULL CHECK (octet_length(content) BETWEEN 1 AND 16384),
    created_at timestamptz NOT NULL,
    UNIQUE (organization_id, owner_user_id, conversation_id, sequence),
    UNIQUE (organization_id, owner_user_id, conversation_id, id),
    FOREIGN KEY (conversation_id, organization_id, owner_user_id)
        REFERENCES ai_workbench.conversations (id, organization_id, owner_user_id)
);
CREATE INDEX IF NOT EXISTS ai_workbench_messages_latest_user
    ON ai_workbench.messages
    (organization_id, owner_user_id, conversation_id, author_kind, sequence DESC);

CREATE TABLE IF NOT EXISTS ai_workbench.commands (
    organization_id text NOT NULL,
    actor_id text NOT NULL,
    idempotency_key uuid NOT NULL,
    operation text NOT NULL CHECK (operation IN ('conversation_create', 'chat_message_plan')),
    request_fingerprint char(64) NOT NULL,
    conversation_id uuid NOT NULL,
    state text NOT NULL CHECK (state IN ('READY_TO_DISPATCH', 'COMPLETE', 'FAILED_BEFORE_DISPATCH', 'PLANNER_UNKNOWN')),
    user_message_id uuid,
    source_sequence bigint,
    planner_invocation_id char(64),
    member_id text CHECK (member_id IS NULL OR octet_length(member_id) BETWEEN 1 AND 128),
    planner_input_hash char(64),
    work_scope jsonb CHECK (work_scope IS NULL OR octet_length(work_scope::text) <= 1024),
    planner_model_profile jsonb,
    planner_started_at timestamptz,
    planner_deadline timestamptz,
    assistant_message_id uuid,
    proposal_id uuid,
    terminal_digest char(64),
    mode text CHECK (mode IS NULL OR mode IN ('CLARIFY', 'READY')),
    created_at timestamptz NOT NULL,
    committed_at timestamptz,
    PRIMARY KEY (organization_id, actor_id, idempotency_key),
    UNIQUE (planner_invocation_id),
    FOREIGN KEY (conversation_id, organization_id, actor_id)
        REFERENCES ai_workbench.conversations (id, organization_id, owner_user_id),
    FOREIGN KEY (organization_id, actor_id, conversation_id, user_message_id)
        REFERENCES ai_workbench.messages (organization_id, owner_user_id, conversation_id, id),
    FOREIGN KEY (organization_id, actor_id, conversation_id, assistant_message_id)
        REFERENCES ai_workbench.messages (organization_id, owner_user_id, conversation_id, id),
    CHECK ((operation = 'conversation_create' AND state = 'COMPLETE'
            AND user_message_id IS NULL AND planner_invocation_id IS NULL)
        OR (operation = 'chat_message_plan' AND user_message_id IS NOT NULL
            AND source_sequence IS NOT NULL AND planner_invocation_id IS NOT NULL
            AND member_id IS NOT NULL
            AND work_scope IS NOT NULL
            AND (state = 'FAILED_BEFORE_DISPATCH' OR
                (planner_input_hash IS NOT NULL AND planner_model_profile IS NOT NULL
                 AND planner_started_at IS NOT NULL AND planner_deadline IS NOT NULL))))
);

CREATE TABLE IF NOT EXISTS ai_workbench.execution_proposals (
    id uuid PRIMARY KEY,
    digest char(64) NOT NULL,
    organization_id text NOT NULL,
    owner_user_id text NOT NULL,
    conversation_id uuid NOT NULL,
    source_user_message_id uuid NOT NULL,
    assistant_message_id uuid NOT NULL,
    source_sequence bigint NOT NULL CHECK (source_sequence > 0),
    kind text NOT NULL CHECK (kind = 'product.title.optimize'),
    goal_summary text NOT NULL CHECK (octet_length(goal_summary) BETWEEN 1 AND 512),
    operation_id text NOT NULL,
    product_key text NOT NULL,
    catalog_version text NOT NULL,
    publication_id text NOT NULL,
    target_platform text NOT NULL CHECK (target_platform IN ('shein', 'temu', 'amazon')),
    agent_id text NOT NULL,
    agent_version text NOT NULL,
    observed_agent_revision text NOT NULL,
    observed_activation_epoch text NOT NULL,
    template_id text,
    template_revision text,
    knowledge_base_id text,
    knowledge_revision_set_digest text,
    execution_model_profile jsonb NOT NULL CHECK (octet_length(execution_model_profile::text) <= 8192),
    created_at timestamptz NOT NULL,
    UNIQUE (id, organization_id, owner_user_id, conversation_id),
    FOREIGN KEY (conversation_id, organization_id, owner_user_id)
        REFERENCES ai_workbench.conversations (id, organization_id, owner_user_id),
    FOREIGN KEY (organization_id, owner_user_id, conversation_id, source_user_message_id)
        REFERENCES ai_workbench.messages (organization_id, owner_user_id, conversation_id, id),
    FOREIGN KEY (organization_id, owner_user_id, conversation_id, assistant_message_id)
        REFERENCES ai_workbench.messages (organization_id, owner_user_id, conversation_id, id)
);
CREATE INDEX IF NOT EXISTS ai_workbench_proposals_conversation
    ON ai_workbench.execution_proposals (organization_id, owner_user_id, conversation_id, created_at DESC);

CREATE TABLE IF NOT EXISTS ai_workbench.business_tasks (
    id uuid PRIMARY KEY,
    organization_id text NOT NULL,
    owner_user_id text NOT NULL,
    conversation_id uuid NOT NULL,
    source_message_id uuid NOT NULL,
    proposal_id uuid NOT NULL,
    proposal_digest char(64) NOT NULL,
    confirmation_fingerprint char(64) NOT NULL,
    kind text NOT NULL CHECK (kind = 'product.title.optimize'),
    title text NOT NULL CHECK (octet_length(title) BETWEEN 1 AND 256),
    goal_summary text NOT NULL CHECK (octet_length(goal_summary) BETWEEN 1 AND 512),
    operation_id text NOT NULL,
    product_key text NOT NULL,
    target_platform text NOT NULL CHECK (target_platform IN ('shein', 'temu', 'amazon')),
    agent_id text NOT NULL,
    agent_version text NOT NULL,
    execution_request_key uuid NOT NULL,
    configuration_snapshot_ref jsonb NOT NULL CHECK (octet_length(configuration_snapshot_ref::text) <= 512),
    context_snapshot_ref jsonb NOT NULL CHECK (octet_length(context_snapshot_ref::text) <= 512),
    execution_request_digest char(64) NOT NULL,
    execution_request jsonb NOT NULL CHECK (octet_length(execution_request::text) <= 8192),
    created_at timestamptz NOT NULL,
    UNIQUE (organization_id, owner_user_id, execution_request_key),
    UNIQUE (id, organization_id, owner_user_id),
    FOREIGN KEY (conversation_id, organization_id, owner_user_id)
        REFERENCES ai_workbench.conversations (id, organization_id, owner_user_id),
    FOREIGN KEY (proposal_id, organization_id, owner_user_id, conversation_id)
        REFERENCES ai_workbench.execution_proposals (id, organization_id, owner_user_id, conversation_id),
    FOREIGN KEY (organization_id, owner_user_id, conversation_id, source_message_id)
        REFERENCES ai_workbench.messages (organization_id, owner_user_id, conversation_id, id)
);
CREATE INDEX IF NOT EXISTS ai_workbench_tasks_owner_recent
    ON ai_workbench.business_tasks (organization_id, owner_user_id, created_at DESC, id);

