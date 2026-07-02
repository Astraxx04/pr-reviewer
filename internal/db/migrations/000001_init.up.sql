-- Consolidated baseline schema for PR Reviewer.
-- Merges migrations 000001–000005 into a single init.
-- River queue tables are managed separately by rivermigrate, not here.

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

CREATE EXTENSION IF NOT EXISTS vector WITH SCHEMA public;

SET default_tablespace = '';
SET default_table_access_method = heap;

-- api_tokens
CREATE TABLE public.api_tokens (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    name text NOT NULL,
    scope text NOT NULL,
    hash text NOT NULL,
    prefix text NOT NULL,
    last_used_at timestamp with time zone,
    expires_at timestamp with time zone,
    created_at timestamp with time zone
);
CREATE SEQUENCE public.api_tokens_id_seq START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;
ALTER SEQUENCE public.api_tokens_id_seq OWNED BY public.api_tokens.id;

-- assignment_rules
CREATE TABLE public.assignment_rules (
    id bigint NOT NULL,
    repo_id bigint NOT NULL,
    strategy text NOT NULL,
    config jsonb,
    created_at timestamp with time zone
);
CREATE SEQUENCE public.assignment_rules_id_seq START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;
ALTER SEQUENCE public.assignment_rules_id_seq OWNED BY public.assignment_rules.id;

-- assignments
CREATE TABLE public.assignments (
    id bigint NOT NULL,
    review_id bigint NOT NULL,
    assignee_login text NOT NULL,
    assigned_at timestamp with time zone,
    completed_at timestamp with time zone
);
CREATE SEQUENCE public.assignments_id_seq START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;
ALTER SEQUENCE public.assignments_id_seq OWNED BY public.assignments.id;

-- audit_logs
CREATE TABLE public.audit_logs (
    id bigint NOT NULL,
    actor_login text NOT NULL,
    actor_id bigint,
    action text NOT NULL,
    entity_type text NOT NULL,
    entity_id text,
    before jsonb,
    after jsonb,
    ip_address text,
    created_at timestamp with time zone
);
CREATE SEQUENCE public.audit_logs_id_seq START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;
ALTER SEQUENCE public.audit_logs_id_seq OWNED BY public.audit_logs.id;

-- bot_comments
CREATE TABLE public.bot_comments (
    id bigint NOT NULL,
    review_id bigint NOT NULL,
    github_comment_id bigint NOT NULL,
    path text,
    line bigint,
    body text,
    resolved boolean,
    created_at timestamp with time zone
);
CREATE SEQUENCE public.bot_comments_id_seq START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;
ALTER SEQUENCE public.bot_comments_id_seq OWNED BY public.bot_comments.id;

-- bot_replies
CREATE TABLE public.bot_replies (
    id bigint NOT NULL,
    github_comment_id bigint NOT NULL,
    replied_at timestamp with time zone NOT NULL
);
CREATE SEQUENCE public.bot_replies_id_seq START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;
ALTER SEQUENCE public.bot_replies_id_seq OWNED BY public.bot_replies.id;

-- code_embeddings
CREATE TABLE public.code_embeddings (
    id bigint NOT NULL,
    repo_id bigint NOT NULL,
    content text NOT NULL,
    embedding public.vector(1536) NOT NULL,
    kind text NOT NULL,
    path text,
    created_at timestamp with time zone
);
CREATE SEQUENCE public.code_embeddings_id_seq START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;
ALTER SEQUENCE public.code_embeddings_id_seq OWNED BY public.code_embeddings.id;

-- comment_feedbacks
CREATE TABLE public.comment_feedbacks (
    id bigint NOT NULL,
    review_comment_id bigint NOT NULL,
    user_login text NOT NULL,
    vote bigint NOT NULL,
    created_at timestamp with time zone
);
CREATE SEQUENCE public.comment_feedbacks_id_seq START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;
ALTER SEQUENCE public.comment_feedbacks_id_seq OWNED BY public.comment_feedbacks.id;

-- github_app_configs (git_hub_token_encrypted removed in 000004)
CREATE TABLE public.github_app_configs (
    id bigint NOT NULL,
    app_id bigint NOT NULL,
    private_key_encrypted text NOT NULL,
    webhook_secret_encrypted text,
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);
CREATE SEQUENCE public.github_app_configs_id_seq START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;
ALTER SEQUENCE public.github_app_configs_id_seq OWNED BY public.github_app_configs.id;

-- installations
CREATE TABLE public.installations (
    id bigint NOT NULL,
    github_installation_id bigint,
    account_login text NOT NULL,
    account_type text,
    created_at timestamp with time zone
);
CREATE SEQUENCE public.installations_id_seq START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;
ALTER SEQUENCE public.installations_id_seq OWNED BY public.installations.id;

-- invites (installation_id removed in 000003)
CREATE TABLE public.invites (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email text NOT NULL,
    role text NOT NULL,
    token_hash text NOT NULL,
    invited_by text NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    accepted_at timestamp with time zone,
    accepted_by text,
    created_at timestamp with time zone
);

-- jira_configs
CREATE TABLE public.jira_configs (
    id bigint NOT NULL,
    base_url text NOT NULL,
    email text NOT NULL,
    api_token_encrypted text NOT NULL,
    enabled boolean DEFAULT true,
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);
CREATE SEQUENCE public.jira_configs_id_seq START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;
ALTER SEQUENCE public.jira_configs_id_seq OWNED BY public.jira_configs.id;

-- notification_configs (installation_id removed in 000003)
CREATE TABLE public.notification_configs (
    id bigint NOT NULL,
    repo_id bigint,
    channel text NOT NULL,
    config jsonb NOT NULL,
    enabled boolean DEFAULT true,
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);
CREATE SEQUENCE public.notification_configs_id_seq START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;
ALTER SEQUENCE public.notification_configs_id_seq OWNED BY public.notification_configs.id;

-- o_id_c_configs
CREATE TABLE public.o_id_c_configs (
    id bigint NOT NULL,
    issuer text NOT NULL,
    client_id text NOT NULL,
    client_secret_encrypted text NOT NULL,
    redirect_url text,
    attribute_mapping jsonb,
    role_mapping jsonb,
    enforced boolean,
    enabled boolean DEFAULT true,
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);
CREATE SEQUENCE public.o_id_c_configs_id_seq START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;
ALTER SEQUENCE public.o_id_c_configs_id_seq OWNED BY public.o_id_c_configs.id;

-- provider_configs (installation_id removed in 000003)
CREATE TABLE public.provider_configs (
    id bigint NOT NULL,
    name text NOT NULL,
    type text NOT NULL,
    api_key_encrypted text,
    base_url text,
    default_model text,
    supports_embeddings boolean,
    embedding_model text,
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);
CREATE SEQUENCE public.provider_configs_id_seq START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;
ALTER SEQUENCE public.provider_configs_id_seq OWNED BY public.provider_configs.id;

-- provider_healths
CREATE TABLE public.provider_healths (
    id bigint NOT NULL,
    provider_config_id bigint NOT NULL,
    last_tested_at timestamp with time zone,
    latency_ms bigint,
    ok boolean,
    error_msg text,
    updated_at timestamp with time zone
);
CREATE SEQUENCE public.provider_healths_id_seq START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;
ALTER SEQUENCE public.provider_healths_id_seq OWNED BY public.provider_healths.id;

-- pull_requests
CREATE TABLE public.pull_requests (
    id bigint NOT NULL,
    repo_id bigint NOT NULL,
    number bigint NOT NULL,
    title text,
    author text,
    head_sha text,
    created_at timestamp with time zone
);
CREATE SEQUENCE public.pull_requests_id_seq START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;
ALTER SEQUENCE public.pull_requests_id_seq OWNED BY public.pull_requests.id;

-- repo_accesses (installation_id removed in 000003)
CREATE TABLE public.repo_accesses (
    id bigint NOT NULL,
    repo_id bigint NOT NULL,
    login text NOT NULL,
    created_at timestamp with time zone
);
CREATE SEQUENCE public.repo_accesses_id_seq START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;
ALTER SEQUENCE public.repo_accesses_id_seq OWNED BY public.repo_accesses.id;

-- repositories (installation_id and FK removed in 000003)
CREATE TABLE public.repositories (
    id bigint NOT NULL,
    owner text NOT NULL,
    name text NOT NULL,
    enabled boolean DEFAULT true,
    indexing_status text DEFAULT 'idle'::text,
    config jsonb,
    embedding_model text DEFAULT ''::text NOT NULL,
    embedding_dim integer DEFAULT 0 NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);
CREATE SEQUENCE public.repositories_id_seq START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;
ALTER SEQUENCE public.repositories_id_seq OWNED BY public.repositories.id;

-- review_comments
CREATE TABLE public.review_comments (
    id bigint NOT NULL,
    review_id bigint NOT NULL,
    path text,
    line bigint,
    side text,
    body text,
    severity text,
    priority text,
    created_at timestamp with time zone
);
CREATE SEQUENCE public.review_comments_id_seq START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;
ALTER SEQUENCE public.review_comments_id_seq OWNED BY public.review_comments.id;

-- reviews
CREATE TABLE public.reviews (
    id bigint NOT NULL,
    pr_id bigint NOT NULL,
    status text,
    score bigint,
    summary text,
    token_usage bigint,
    input_tokens bigint,
    output_tokens bigint,
    latency_ms bigint,
    created_at timestamp with time zone
);
CREATE SEQUENCE public.reviews_id_seq START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;
ALTER SEQUENCE public.reviews_id_seq OWNED BY public.reviews.id;

-- sessions
CREATE TABLE public.sessions (
    id text NOT NULL,
    user_id bigint NOT NULL,
    user_agent text,
    ip_address text,
    last_active_at timestamp with time zone,
    expires_at timestamp with time zone NOT NULL,
    created_at timestamp with time zone
);

-- slack_app_configs
CREATE TABLE public.slack_app_configs (
    id bigint NOT NULL,
    signing_secret_encrypted text NOT NULL,
    bot_token_encrypted text,
    enabled boolean DEFAULT true,
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);
CREATE SEQUENCE public.slack_app_configs_id_seq START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;
ALTER SEQUENCE public.slack_app_configs_id_seq OWNED BY public.slack_app_configs.id;

-- system_configs
CREATE TABLE public.system_configs (
    id bigint NOT NULL,
    key text NOT NULL,
    value text NOT NULL
);
CREATE SEQUENCE public.system_configs_id_seq START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;
ALTER SEQUENCE public.system_configs_id_seq OWNED BY public.system_configs.id;

-- users (role default changed to 'reviewer' in 000002)
CREATE TABLE public.users (
    id bigint NOT NULL,
    github_id bigint NOT NULL,
    login text NOT NULL,
    email text,
    avatar_url text,
    role text DEFAULT 'reviewer'::text,
    status text DEFAULT 'active'::text,
    created_at timestamp with time zone
);
CREATE SEQUENCE public.users_id_seq START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;
ALTER SEQUENCE public.users_id_seq OWNED BY public.users.id;

-- webhook_deliveries
CREATE TABLE public.webhook_deliveries (
    delivery_id text NOT NULL,
    processed_at timestamp with time zone NOT NULL,
    event_type text,
    action text,
    owner text,
    repo text,
    pr_number bigint,
    status text
);

-- ── Sequence defaults ────────────────────────────────────────────────────────

ALTER TABLE ONLY public.api_tokens ALTER COLUMN id SET DEFAULT nextval('public.api_tokens_id_seq'::regclass);
ALTER TABLE ONLY public.assignment_rules ALTER COLUMN id SET DEFAULT nextval('public.assignment_rules_id_seq'::regclass);
ALTER TABLE ONLY public.assignments ALTER COLUMN id SET DEFAULT nextval('public.assignments_id_seq'::regclass);
ALTER TABLE ONLY public.audit_logs ALTER COLUMN id SET DEFAULT nextval('public.audit_logs_id_seq'::regclass);
ALTER TABLE ONLY public.bot_comments ALTER COLUMN id SET DEFAULT nextval('public.bot_comments_id_seq'::regclass);
ALTER TABLE ONLY public.bot_replies ALTER COLUMN id SET DEFAULT nextval('public.bot_replies_id_seq'::regclass);
ALTER TABLE ONLY public.code_embeddings ALTER COLUMN id SET DEFAULT nextval('public.code_embeddings_id_seq'::regclass);
ALTER TABLE ONLY public.comment_feedbacks ALTER COLUMN id SET DEFAULT nextval('public.comment_feedbacks_id_seq'::regclass);
ALTER TABLE ONLY public.github_app_configs ALTER COLUMN id SET DEFAULT nextval('public.github_app_configs_id_seq'::regclass);
ALTER TABLE ONLY public.installations ALTER COLUMN id SET DEFAULT nextval('public.installations_id_seq'::regclass);
ALTER TABLE ONLY public.jira_configs ALTER COLUMN id SET DEFAULT nextval('public.jira_configs_id_seq'::regclass);
ALTER TABLE ONLY public.notification_configs ALTER COLUMN id SET DEFAULT nextval('public.notification_configs_id_seq'::regclass);
ALTER TABLE ONLY public.o_id_c_configs ALTER COLUMN id SET DEFAULT nextval('public.o_id_c_configs_id_seq'::regclass);
ALTER TABLE ONLY public.provider_configs ALTER COLUMN id SET DEFAULT nextval('public.provider_configs_id_seq'::regclass);
ALTER TABLE ONLY public.provider_healths ALTER COLUMN id SET DEFAULT nextval('public.provider_healths_id_seq'::regclass);
ALTER TABLE ONLY public.pull_requests ALTER COLUMN id SET DEFAULT nextval('public.pull_requests_id_seq'::regclass);
ALTER TABLE ONLY public.repo_accesses ALTER COLUMN id SET DEFAULT nextval('public.repo_accesses_id_seq'::regclass);
ALTER TABLE ONLY public.repositories ALTER COLUMN id SET DEFAULT nextval('public.repositories_id_seq'::regclass);
ALTER TABLE ONLY public.review_comments ALTER COLUMN id SET DEFAULT nextval('public.review_comments_id_seq'::regclass);
ALTER TABLE ONLY public.reviews ALTER COLUMN id SET DEFAULT nextval('public.reviews_id_seq'::regclass);
ALTER TABLE ONLY public.slack_app_configs ALTER COLUMN id SET DEFAULT nextval('public.slack_app_configs_id_seq'::regclass);
ALTER TABLE ONLY public.system_configs ALTER COLUMN id SET DEFAULT nextval('public.system_configs_id_seq'::regclass);
ALTER TABLE ONLY public.users ALTER COLUMN id SET DEFAULT nextval('public.users_id_seq'::regclass);

-- ── Primary keys ─────────────────────────────────────────────────────────────

ALTER TABLE ONLY public.api_tokens ADD CONSTRAINT api_tokens_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.assignment_rules ADD CONSTRAINT assignment_rules_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.assignments ADD CONSTRAINT assignments_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.audit_logs ADD CONSTRAINT audit_logs_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.bot_comments ADD CONSTRAINT bot_comments_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.bot_replies ADD CONSTRAINT bot_replies_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.code_embeddings ADD CONSTRAINT code_embeddings_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.comment_feedbacks ADD CONSTRAINT comment_feedbacks_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.github_app_configs ADD CONSTRAINT github_app_configs_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.installations ADD CONSTRAINT installations_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.jira_configs ADD CONSTRAINT jira_configs_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.notification_configs ADD CONSTRAINT notification_configs_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.o_id_c_configs ADD CONSTRAINT o_id_c_configs_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.provider_configs ADD CONSTRAINT provider_configs_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.provider_healths ADD CONSTRAINT provider_healths_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.pull_requests ADD CONSTRAINT pull_requests_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.repo_accesses ADD CONSTRAINT repo_accesses_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.repositories ADD CONSTRAINT repositories_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.review_comments ADD CONSTRAINT review_comments_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.reviews ADD CONSTRAINT reviews_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.sessions ADD CONSTRAINT sessions_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.slack_app_configs ADD CONSTRAINT slack_app_configs_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.system_configs ADD CONSTRAINT system_configs_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.users ADD CONSTRAINT users_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.webhook_deliveries ADD CONSTRAINT webhook_deliveries_pkey PRIMARY KEY (delivery_id);

-- ── Indexes ───────────────────────────────────────────────────────────────────

CREATE INDEX code_embeddings_hnsw ON public.code_embeddings USING hnsw (embedding public.vector_cosine_ops);
CREATE UNIQUE INDEX idx_api_tokens_hash ON public.api_tokens USING btree (hash);
CREATE INDEX idx_api_tokens_user_id ON public.api_tokens USING btree (user_id);
CREATE INDEX idx_assignment_rules_repo_id ON public.assignment_rules USING btree (repo_id);
CREATE INDEX idx_assignments_review_id ON public.assignments USING btree (review_id);
CREATE INDEX idx_audit_logs_actor_id ON public.audit_logs USING btree (actor_id);
CREATE INDEX idx_audit_logs_actor_login ON public.audit_logs USING btree (actor_login);
CREATE INDEX idx_audit_logs_created_at ON public.audit_logs USING btree (created_at);
CREATE INDEX idx_audit_logs_entity_id ON public.audit_logs USING btree (entity_id);
CREATE UNIQUE INDEX idx_bot_comments_github_comment_id ON public.bot_comments USING btree (github_comment_id);
CREATE INDEX idx_bot_comments_review_id ON public.bot_comments USING btree (review_id);
CREATE UNIQUE INDEX idx_bot_replies_github_comment_id ON public.bot_replies USING btree (github_comment_id);
CREATE INDEX idx_code_embeddings_repo_id ON public.code_embeddings USING btree (repo_id);
CREATE INDEX idx_comment_feedbacks_review_comment_id ON public.comment_feedbacks USING btree (review_comment_id);
CREATE UNIQUE INDEX idx_installations_account_login ON public.installations USING btree (account_login);
CREATE UNIQUE INDEX idx_installations_github_installation_id ON public.installations USING btree (github_installation_id);
CREATE UNIQUE INDEX idx_invites_token_hash ON public.invites USING btree (token_hash);
CREATE UNIQUE INDEX invites_pending_email_uniq ON public.invites (email) WHERE (accepted_at IS NULL);
CREATE INDEX idx_notification_configs_repo_id ON public.notification_configs USING btree (repo_id);
CREATE UNIQUE INDEX idx_pr_repo_number ON public.pull_requests USING btree (repo_id, number);
CREATE UNIQUE INDEX idx_provider_healths_provider_config_id ON public.provider_healths USING btree (provider_config_id);
CREATE UNIQUE INDEX idx_repo_owner_name ON public.repositories USING btree (owner, name);
CREATE UNIQUE INDEX idx_repo_login ON public.repo_accesses USING btree (repo_id, login);
CREATE INDEX idx_review_comments_review_id ON public.review_comments USING btree (review_id);
CREATE INDEX idx_reviews_pr_id ON public.reviews USING btree (pr_id);
CREATE INDEX idx_sessions_expires_at ON public.sessions USING btree (expires_at);
CREATE INDEX idx_sessions_user_id ON public.sessions USING btree (user_id);
CREATE UNIQUE INDEX idx_system_configs_key ON public.system_configs USING btree (key);
CREATE UNIQUE INDEX idx_users_github_id ON public.users USING btree (github_id);

-- ── Foreign keys ──────────────────────────────────────────────────────────────

ALTER TABLE ONLY public.assignment_rules
    ADD CONSTRAINT fk_repositories_assignment_rules FOREIGN KEY (repo_id) REFERENCES public.repositories(id);

ALTER TABLE ONLY public.pull_requests
    ADD CONSTRAINT fk_repositories_pull_requests FOREIGN KEY (repo_id) REFERENCES public.repositories(id);

ALTER TABLE ONLY public.reviews
    ADD CONSTRAINT fk_pull_requests_reviews FOREIGN KEY (pr_id) REFERENCES public.pull_requests(id);

ALTER TABLE ONLY public.assignments
    ADD CONSTRAINT fk_reviews_assignments FOREIGN KEY (review_id) REFERENCES public.reviews(id);

ALTER TABLE ONLY public.review_comments
    ADD CONSTRAINT fk_reviews_comments FOREIGN KEY (review_id) REFERENCES public.reviews(id);

-- ON DELETE CASCADE added in 000005
ALTER TABLE ONLY public.provider_healths
    ADD CONSTRAINT fk_provider_healths_provider_config FOREIGN KEY (provider_config_id) REFERENCES public.provider_configs(id) ON DELETE CASCADE;
