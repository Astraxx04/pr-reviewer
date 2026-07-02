-- Drop everything created by the init migration.

DROP TABLE IF EXISTS public.webhook_deliveries,
    public.sessions, public.review_comments, public.assignments,
    public.reviews, public.pull_requests, public.repo_accesses,
    public.repositories, public.assignment_rules, public.code_embeddings,
    public.bot_comments, public.bot_replies, public.comment_feedbacks,
    public.audit_logs, public.provider_healths, public.provider_configs,
    public.notification_configs, public.invites, public.api_tokens,
    public.users, public.system_configs, public.slack_app_configs,
    public.o_id_c_configs, public.jira_configs, public.github_app_configs,
    public.installations CASCADE;

DROP EXTENSION IF EXISTS vector;
