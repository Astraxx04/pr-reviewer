-- Baseline schema for PR Reviewer.
-- Generated from GORM AutoMigrate via pg_dump (schema-only).
-- River queue tables are managed separately by rivermigrate, not here.

--
-- PostgreSQL database dump
--


-- Dumped from database version 16.14 (Debian 16.14-1.pgdg12+1)
-- Dumped by pg_dump version 16.14 (Debian 16.14-1.pgdg12+1)

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

--
-- Name: vector; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS vector WITH SCHEMA public;


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: api_tokens; Type: TABLE; Schema: public; Owner: -
--

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


--
-- Name: api_tokens_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.api_tokens_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: api_tokens_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.api_tokens_id_seq OWNED BY public.api_tokens.id;


--
-- Name: assignment_rules; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.assignment_rules (
    id bigint NOT NULL,
    repo_id bigint NOT NULL,
    strategy text NOT NULL,
    config jsonb,
    created_at timestamp with time zone
);


--
-- Name: assignment_rules_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.assignment_rules_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: assignment_rules_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.assignment_rules_id_seq OWNED BY public.assignment_rules.id;


--
-- Name: assignments; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.assignments (
    id bigint NOT NULL,
    review_id bigint NOT NULL,
    assignee_login text NOT NULL,
    assigned_at timestamp with time zone,
    completed_at timestamp with time zone
);


--
-- Name: assignments_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.assignments_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: assignments_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.assignments_id_seq OWNED BY public.assignments.id;


--
-- Name: audit_logs; Type: TABLE; Schema: public; Owner: -
--

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


--
-- Name: audit_logs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.audit_logs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: audit_logs_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.audit_logs_id_seq OWNED BY public.audit_logs.id;


--
-- Name: bot_comments; Type: TABLE; Schema: public; Owner: -
--

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


--
-- Name: bot_comments_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.bot_comments_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: bot_comments_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.bot_comments_id_seq OWNED BY public.bot_comments.id;


--
-- Name: bot_replies; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.bot_replies (
    id bigint NOT NULL,
    github_comment_id bigint NOT NULL,
    replied_at timestamp with time zone NOT NULL
);


--
-- Name: bot_replies_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.bot_replies_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: bot_replies_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.bot_replies_id_seq OWNED BY public.bot_replies.id;


--
-- Name: code_embeddings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.code_embeddings (
    id bigint NOT NULL,
    repo_id bigint NOT NULL,
    content text NOT NULL,
    embedding public.vector(1536) NOT NULL,
    kind text NOT NULL,
    path text,
    created_at timestamp with time zone
);


--
-- Name: code_embeddings_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.code_embeddings_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: code_embeddings_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.code_embeddings_id_seq OWNED BY public.code_embeddings.id;


--
-- Name: comment_feedbacks; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.comment_feedbacks (
    id bigint NOT NULL,
    review_comment_id bigint NOT NULL,
    user_login text NOT NULL,
    vote bigint NOT NULL,
    created_at timestamp with time zone
);


--
-- Name: comment_feedbacks_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.comment_feedbacks_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: comment_feedbacks_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.comment_feedbacks_id_seq OWNED BY public.comment_feedbacks.id;


--
-- Name: github_app_configs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.github_app_configs (
    id bigint NOT NULL,
    app_id bigint NOT NULL,
    private_key_encrypted text NOT NULL,
    webhook_secret_encrypted text,
    git_hub_token_encrypted text,
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);


--
-- Name: github_app_configs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.github_app_configs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: github_app_configs_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.github_app_configs_id_seq OWNED BY public.github_app_configs.id;


--
-- Name: installations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.installations (
    id bigint NOT NULL,
    github_installation_id bigint,
    account_login text NOT NULL,
    account_type text,
    created_at timestamp with time zone
);


--
-- Name: installations_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.installations_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: installations_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.installations_id_seq OWNED BY public.installations.id;


--
-- Name: jira_configs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.jira_configs (
    id bigint NOT NULL,
    base_url text NOT NULL,
    email text NOT NULL,
    api_token_encrypted text NOT NULL,
    enabled boolean DEFAULT true,
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);


--
-- Name: jira_configs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.jira_configs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: jira_configs_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.jira_configs_id_seq OWNED BY public.jira_configs.id;


--
-- Name: notification_configs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.notification_configs (
    id bigint NOT NULL,
    installation_id bigint NOT NULL,
    repo_id bigint,
    channel text NOT NULL,
    config jsonb NOT NULL,
    enabled boolean DEFAULT true,
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);


--
-- Name: notification_configs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.notification_configs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: notification_configs_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.notification_configs_id_seq OWNED BY public.notification_configs.id;


--
-- Name: o_id_c_configs; Type: TABLE; Schema: public; Owner: -
--

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


--
-- Name: o_id_c_configs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.o_id_c_configs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: o_id_c_configs_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.o_id_c_configs_id_seq OWNED BY public.o_id_c_configs.id;


--
-- Name: provider_configs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.provider_configs (
    id bigint NOT NULL,
    installation_id bigint NOT NULL,
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


--
-- Name: provider_configs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.provider_configs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: provider_configs_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.provider_configs_id_seq OWNED BY public.provider_configs.id;


--
-- Name: provider_healths; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.provider_healths (
    id bigint NOT NULL,
    provider_config_id bigint NOT NULL,
    last_tested_at timestamp with time zone,
    latency_ms bigint,
    ok boolean,
    error_msg text,
    updated_at timestamp with time zone
);


--
-- Name: provider_healths_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.provider_healths_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: provider_healths_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.provider_healths_id_seq OWNED BY public.provider_healths.id;


--
-- Name: pull_requests; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.pull_requests (
    id bigint NOT NULL,
    repo_id bigint NOT NULL,
    number bigint NOT NULL,
    title text,
    author text,
    head_sha text,
    created_at timestamp with time zone
);


--
-- Name: pull_requests_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.pull_requests_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: pull_requests_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.pull_requests_id_seq OWNED BY public.pull_requests.id;


--
-- Name: repositories; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.repositories (
    id bigint NOT NULL,
    installation_id bigint NOT NULL,
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


--
-- Name: repositories_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.repositories_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: repositories_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.repositories_id_seq OWNED BY public.repositories.id;


--
-- Name: review_comments; Type: TABLE; Schema: public; Owner: -
--

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


--
-- Name: review_comments_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.review_comments_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: review_comments_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.review_comments_id_seq OWNED BY public.review_comments.id;


--
-- Name: reviews; Type: TABLE; Schema: public; Owner: -
--

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


--
-- Name: reviews_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.reviews_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: reviews_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.reviews_id_seq OWNED BY public.reviews.id;


--
-- Name: sessions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sessions (
    id text NOT NULL,
    user_id bigint NOT NULL,
    user_agent text,
    ip_address text,
    last_active_at timestamp with time zone,
    expires_at timestamp with time zone NOT NULL,
    created_at timestamp with time zone
);


--
-- Name: slack_app_configs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.slack_app_configs (
    id bigint NOT NULL,
    signing_secret_encrypted text NOT NULL,
    bot_token_encrypted text,
    enabled boolean DEFAULT true,
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);


--
-- Name: slack_app_configs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.slack_app_configs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: slack_app_configs_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.slack_app_configs_id_seq OWNED BY public.slack_app_configs.id;


--
-- Name: system_configs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.system_configs (
    id bigint NOT NULL,
    key text NOT NULL,
    value text NOT NULL
);


--
-- Name: system_configs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.system_configs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: system_configs_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.system_configs_id_seq OWNED BY public.system_configs.id;


--
-- Name: team_members; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.team_members (
    id bigint NOT NULL,
    installation_id bigint NOT NULL,
    login text NOT NULL,
    role text DEFAULT 'reviewer'::text,
    slack_user_id text,
    created_at timestamp with time zone
);


--
-- Name: team_members_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.team_members_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: team_members_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.team_members_id_seq OWNED BY public.team_members.id;


--
-- Name: users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.users (
    id bigint NOT NULL,
    github_id bigint NOT NULL,
    login text NOT NULL,
    email text,
    avatar_url text,
    role text DEFAULT 'viewer'::text,
    status text DEFAULT 'active'::text,
    created_at timestamp with time zone
);


--
-- Name: users_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.users_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: users_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.users_id_seq OWNED BY public.users.id;


--
-- Name: webhook_deliveries; Type: TABLE; Schema: public; Owner: -
--

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


--
-- Name: api_tokens id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.api_tokens ALTER COLUMN id SET DEFAULT nextval('public.api_tokens_id_seq'::regclass);


--
-- Name: assignment_rules id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.assignment_rules ALTER COLUMN id SET DEFAULT nextval('public.assignment_rules_id_seq'::regclass);


--
-- Name: assignments id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.assignments ALTER COLUMN id SET DEFAULT nextval('public.assignments_id_seq'::regclass);


--
-- Name: audit_logs id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.audit_logs ALTER COLUMN id SET DEFAULT nextval('public.audit_logs_id_seq'::regclass);


--
-- Name: bot_comments id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.bot_comments ALTER COLUMN id SET DEFAULT nextval('public.bot_comments_id_seq'::regclass);


--
-- Name: bot_replies id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.bot_replies ALTER COLUMN id SET DEFAULT nextval('public.bot_replies_id_seq'::regclass);


--
-- Name: code_embeddings id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.code_embeddings ALTER COLUMN id SET DEFAULT nextval('public.code_embeddings_id_seq'::regclass);


--
-- Name: comment_feedbacks id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.comment_feedbacks ALTER COLUMN id SET DEFAULT nextval('public.comment_feedbacks_id_seq'::regclass);


--
-- Name: github_app_configs id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.github_app_configs ALTER COLUMN id SET DEFAULT nextval('public.github_app_configs_id_seq'::regclass);


--
-- Name: installations id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.installations ALTER COLUMN id SET DEFAULT nextval('public.installations_id_seq'::regclass);


--
-- Name: jira_configs id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jira_configs ALTER COLUMN id SET DEFAULT nextval('public.jira_configs_id_seq'::regclass);


--
-- Name: notification_configs id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.notification_configs ALTER COLUMN id SET DEFAULT nextval('public.notification_configs_id_seq'::regclass);


--
-- Name: o_id_c_configs id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.o_id_c_configs ALTER COLUMN id SET DEFAULT nextval('public.o_id_c_configs_id_seq'::regclass);


--
-- Name: provider_configs id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.provider_configs ALTER COLUMN id SET DEFAULT nextval('public.provider_configs_id_seq'::regclass);


--
-- Name: provider_healths id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.provider_healths ALTER COLUMN id SET DEFAULT nextval('public.provider_healths_id_seq'::regclass);


--
-- Name: pull_requests id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pull_requests ALTER COLUMN id SET DEFAULT nextval('public.pull_requests_id_seq'::regclass);


--
-- Name: repositories id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.repositories ALTER COLUMN id SET DEFAULT nextval('public.repositories_id_seq'::regclass);


--
-- Name: review_comments id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.review_comments ALTER COLUMN id SET DEFAULT nextval('public.review_comments_id_seq'::regclass);


--
-- Name: reviews id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reviews ALTER COLUMN id SET DEFAULT nextval('public.reviews_id_seq'::regclass);


--
-- Name: slack_app_configs id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.slack_app_configs ALTER COLUMN id SET DEFAULT nextval('public.slack_app_configs_id_seq'::regclass);


--
-- Name: system_configs id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.system_configs ALTER COLUMN id SET DEFAULT nextval('public.system_configs_id_seq'::regclass);


--
-- Name: team_members id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.team_members ALTER COLUMN id SET DEFAULT nextval('public.team_members_id_seq'::regclass);


--
-- Name: users id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users ALTER COLUMN id SET DEFAULT nextval('public.users_id_seq'::regclass);


--
-- Name: api_tokens api_tokens_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.api_tokens
    ADD CONSTRAINT api_tokens_pkey PRIMARY KEY (id);


--
-- Name: assignment_rules assignment_rules_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.assignment_rules
    ADD CONSTRAINT assignment_rules_pkey PRIMARY KEY (id);


--
-- Name: assignments assignments_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.assignments
    ADD CONSTRAINT assignments_pkey PRIMARY KEY (id);


--
-- Name: audit_logs audit_logs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.audit_logs
    ADD CONSTRAINT audit_logs_pkey PRIMARY KEY (id);


--
-- Name: bot_comments bot_comments_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.bot_comments
    ADD CONSTRAINT bot_comments_pkey PRIMARY KEY (id);


--
-- Name: bot_replies bot_replies_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.bot_replies
    ADD CONSTRAINT bot_replies_pkey PRIMARY KEY (id);


--
-- Name: code_embeddings code_embeddings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.code_embeddings
    ADD CONSTRAINT code_embeddings_pkey PRIMARY KEY (id);


--
-- Name: comment_feedbacks comment_feedbacks_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.comment_feedbacks
    ADD CONSTRAINT comment_feedbacks_pkey PRIMARY KEY (id);


--
-- Name: github_app_configs github_app_configs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.github_app_configs
    ADD CONSTRAINT github_app_configs_pkey PRIMARY KEY (id);


--
-- Name: installations installations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.installations
    ADD CONSTRAINT installations_pkey PRIMARY KEY (id);


--
-- Name: jira_configs jira_configs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jira_configs
    ADD CONSTRAINT jira_configs_pkey PRIMARY KEY (id);


--
-- Name: notification_configs notification_configs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.notification_configs
    ADD CONSTRAINT notification_configs_pkey PRIMARY KEY (id);


--
-- Name: o_id_c_configs o_id_c_configs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.o_id_c_configs
    ADD CONSTRAINT o_id_c_configs_pkey PRIMARY KEY (id);


--
-- Name: provider_configs provider_configs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.provider_configs
    ADD CONSTRAINT provider_configs_pkey PRIMARY KEY (id);


--
-- Name: provider_healths provider_healths_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.provider_healths
    ADD CONSTRAINT provider_healths_pkey PRIMARY KEY (id);


--
-- Name: pull_requests pull_requests_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pull_requests
    ADD CONSTRAINT pull_requests_pkey PRIMARY KEY (id);


--
-- Name: repositories repositories_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.repositories
    ADD CONSTRAINT repositories_pkey PRIMARY KEY (id);


--
-- Name: review_comments review_comments_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.review_comments
    ADD CONSTRAINT review_comments_pkey PRIMARY KEY (id);


--
-- Name: reviews reviews_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reviews
    ADD CONSTRAINT reviews_pkey PRIMARY KEY (id);


--
-- Name: sessions sessions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sessions
    ADD CONSTRAINT sessions_pkey PRIMARY KEY (id);


--
-- Name: slack_app_configs slack_app_configs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.slack_app_configs
    ADD CONSTRAINT slack_app_configs_pkey PRIMARY KEY (id);


--
-- Name: system_configs system_configs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.system_configs
    ADD CONSTRAINT system_configs_pkey PRIMARY KEY (id);


--
-- Name: team_members team_members_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.team_members
    ADD CONSTRAINT team_members_pkey PRIMARY KEY (id);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: webhook_deliveries webhook_deliveries_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.webhook_deliveries
    ADD CONSTRAINT webhook_deliveries_pkey PRIMARY KEY (delivery_id);


--
-- Name: code_embeddings_hnsw; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX code_embeddings_hnsw ON public.code_embeddings USING hnsw (embedding public.vector_cosine_ops);


--
-- Name: idx_api_tokens_hash; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_api_tokens_hash ON public.api_tokens USING btree (hash);


--
-- Name: idx_api_tokens_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_api_tokens_user_id ON public.api_tokens USING btree (user_id);


--
-- Name: idx_assignment_rules_repo_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_assignment_rules_repo_id ON public.assignment_rules USING btree (repo_id);


--
-- Name: idx_assignments_review_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_assignments_review_id ON public.assignments USING btree (review_id);


--
-- Name: idx_audit_logs_actor_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_audit_logs_actor_id ON public.audit_logs USING btree (actor_id);


--
-- Name: idx_audit_logs_actor_login; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_audit_logs_actor_login ON public.audit_logs USING btree (actor_login);


--
-- Name: idx_audit_logs_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_audit_logs_created_at ON public.audit_logs USING btree (created_at);


--
-- Name: idx_audit_logs_entity_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_audit_logs_entity_id ON public.audit_logs USING btree (entity_id);


--
-- Name: idx_bot_comments_github_comment_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_bot_comments_github_comment_id ON public.bot_comments USING btree (github_comment_id);


--
-- Name: idx_bot_comments_review_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_bot_comments_review_id ON public.bot_comments USING btree (review_id);


--
-- Name: idx_bot_replies_github_comment_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_bot_replies_github_comment_id ON public.bot_replies USING btree (github_comment_id);


--
-- Name: idx_code_embeddings_repo_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_code_embeddings_repo_id ON public.code_embeddings USING btree (repo_id);


--
-- Name: idx_comment_feedbacks_review_comment_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_comment_feedbacks_review_comment_id ON public.comment_feedbacks USING btree (review_comment_id);


--
-- Name: idx_installations_account_login; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_installations_account_login ON public.installations USING btree (account_login);


--
-- Name: idx_installations_github_installation_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_installations_github_installation_id ON public.installations USING btree (github_installation_id);


--
-- Name: idx_notification_configs_installation_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_notification_configs_installation_id ON public.notification_configs USING btree (installation_id);


--
-- Name: idx_notification_configs_repo_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_notification_configs_repo_id ON public.notification_configs USING btree (repo_id);


--
-- Name: idx_pr_repo_number; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_pr_repo_number ON public.pull_requests USING btree (repo_id, number);


--
-- Name: idx_provider_configs_installation_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_provider_configs_installation_id ON public.provider_configs USING btree (installation_id);


--
-- Name: idx_provider_healths_provider_config_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_provider_healths_provider_config_id ON public.provider_healths USING btree (provider_config_id);


--
-- Name: idx_repo_owner_name; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_repo_owner_name ON public.repositories USING btree (owner, name);


--
-- Name: idx_repositories_installation_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_repositories_installation_id ON public.repositories USING btree (installation_id);


--
-- Name: idx_review_comments_review_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_review_comments_review_id ON public.review_comments USING btree (review_id);


--
-- Name: idx_reviews_pr_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_reviews_pr_id ON public.reviews USING btree (pr_id);


--
-- Name: idx_sessions_expires_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_sessions_expires_at ON public.sessions USING btree (expires_at);


--
-- Name: idx_sessions_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_sessions_user_id ON public.sessions USING btree (user_id);


--
-- Name: idx_system_configs_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_system_configs_key ON public.system_configs USING btree (key);


--
-- Name: idx_team_members_installation_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_team_members_installation_id ON public.team_members USING btree (installation_id);


--
-- Name: idx_users_github_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_users_github_id ON public.users USING btree (github_id);


--
-- Name: repositories fk_installations_repos; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.repositories
    ADD CONSTRAINT fk_installations_repos FOREIGN KEY (installation_id) REFERENCES public.installations(id);


--
-- Name: reviews fk_pull_requests_reviews; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reviews
    ADD CONSTRAINT fk_pull_requests_reviews FOREIGN KEY (pr_id) REFERENCES public.pull_requests(id);


--
-- Name: assignment_rules fk_repositories_assignment_rules; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.assignment_rules
    ADD CONSTRAINT fk_repositories_assignment_rules FOREIGN KEY (repo_id) REFERENCES public.repositories(id);


--
-- Name: pull_requests fk_repositories_pull_requests; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pull_requests
    ADD CONSTRAINT fk_repositories_pull_requests FOREIGN KEY (repo_id) REFERENCES public.repositories(id);


--
-- Name: assignments fk_reviews_assignments; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.assignments
    ADD CONSTRAINT fk_reviews_assignments FOREIGN KEY (review_id) REFERENCES public.reviews(id);


--
-- Name: review_comments fk_reviews_comments; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.review_comments
    ADD CONSTRAINT fk_reviews_comments FOREIGN KEY (review_id) REFERENCES public.reviews(id);


--
-- Name: repo_accesses; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.repo_accesses (
    id bigint NOT NULL,
    installation_id bigint NOT NULL,
    repo_id bigint NOT NULL,
    login text NOT NULL,
    created_at timestamp with time zone
);


--
-- Name: repo_accesses_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.repo_accesses_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: repo_accesses_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.repo_accesses_id_seq OWNED BY public.repo_accesses.id;


--
-- Name: repo_accesses id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.repo_accesses ALTER COLUMN id SET DEFAULT nextval('public.repo_accesses_id_seq'::regclass);


--
-- Name: repo_accesses repo_accesses_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.repo_accesses
    ADD CONSTRAINT repo_accesses_pkey PRIMARY KEY (id);


--
-- Name: idx_repo_accesses_installation_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_repo_accesses_installation_id ON public.repo_accesses USING btree (installation_id);


--
-- Name: idx_repo_login; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_repo_login ON public.repo_accesses USING btree (repo_id, login);


--
-- PostgreSQL database dump complete
--


