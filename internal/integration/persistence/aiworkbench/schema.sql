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
    planner_input_hash char(64),
    planner_model_profile jsonb,
    planner_started_at timestamptz,
    planner_deadline timestamptz,
    assistant_message_id uuid,
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
            AND planner_input_hash IS NOT NULL AND planner_model_profile IS NOT NULL
            AND planner_started_at IS NOT NULL AND planner_deadline IS NOT NULL))
);

