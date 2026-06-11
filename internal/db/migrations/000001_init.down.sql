-- Reverse of 000001_init: drop all baseline objects.

DROP TABLE IF EXISTS api_tokens, assignment_rules, assignments, audit_logs, bot_comments, bot_replies, code_embeddings, comment_feedbacks, github_app_configs, installations, jira_configs, notification_configs, o_id_c_configs, provider_configs, provider_healths, pull_requests, repo_accesses, repositories, review_comments, reviews, sessions, slack_app_configs, system_configs, team_members, users, webhook_deliveries CASCADE;

DROP EXTENSION IF EXISTS vector;
