package models

import (
	"time"

	"gorm.io/datatypes"
)

type User struct {
	ID        uint   `gorm:"primarykey"`
	GithubID  int64  `gorm:"uniqueIndex;not null"`
	Login     string `gorm:"not null"`
	Email     string
	AvatarURL string
	Role      string `gorm:"default:viewer"` // owner|admin|reviewer|viewer
	Status    string `gorm:"default:active"` // active|pending
	CreatedAt time.Time
}

// Session tracks active user sessions for revocation support.
type Session struct {
	ID           string `gorm:"primarykey"` // UUID
	UserID       uint   `gorm:"index;not null"`
	UserAgent    string
	IPAddress    string
	LastActiveAt time.Time
	ExpiresAt    time.Time `gorm:"index;not null"`
	CreatedAt    time.Time
}

type Installation struct {
	ID uint `gorm:"primarykey"`
	// GithubInstallationID is the GitHub-assigned installation ID. It is NULL
	// for stub installations created by the review path before the
	// installation.created webhook arrives; Postgres treats NULLs as distinct,
	// so many stubs can coexist while real IDs stay unique.
	GithubInstallationID *int64 `gorm:"uniqueIndex"`
	// AccountLogin is the GitHub account (owner) and the natural key: one
	// installation per account. All reconciliation keys on this.
	AccountLogin string `gorm:"uniqueIndex;not null"`
	AccountType  string // User | Organization
	CreatedAt    time.Time

	Repos []Repository `gorm:"foreignKey:InstallationID"`
}

// ProviderConfig stores one LLM provider configuration per installation.
// APIKeyEncrypted holds an AES-GCM ciphertext; never store the raw key.
type ProviderConfig struct {
	ID                 uint   `gorm:"primarykey"`
	InstallationID     uint   `gorm:"index;not null"`
	Name               string `gorm:"not null"` // user-facing label
	Type               string `gorm:"not null"` // openai|anthropic|ollama|openai_compatible
	APIKeyEncrypted    string
	BaseURL            string
	DefaultModel       string // chat/completion model used for reviews
	SupportsEmbeddings bool
	// EmbeddingModel is the model used for RAG embeddings when SupportsEmbeddings
	// is true (e.g. text-embedding-3-small). Distinct from DefaultModel, which is a
	// chat model — using a chat model for the embeddings endpoint fails.
	EmbeddingModel string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type Repository struct {
	ID             uint `gorm:"primarykey"`
	InstallationID uint `gorm:"index;not null"`
	// (Owner, Name) is the GitHub-global natural key for a repository.
	Owner          string `gorm:"uniqueIndex:idx_repo_owner_name;not null"`
	Name           string `gorm:"uniqueIndex:idx_repo_owner_name;not null"`
	Enabled        bool   `gorm:"default:true"`
	IndexingStatus string `gorm:"default:idle"` // idle | indexing | indexed | error
	// EmbeddingModel is the "<provider>/<model>" signature of the embedder last
	// used to index this repo; EmbeddingDim is the vector dimension it produced.
	// A change in EmbeddingModel triggers a full re-index (the old vectors are
	// not comparable across embedding spaces).
	EmbeddingModel string         `gorm:"not null;default:''"`
	EmbeddingDim   int32          `gorm:"not null;default:0"`
	Config         datatypes.JSON // {"agents":{"code-review":{"provider_id":"...","model":"..."}}}
	CreatedAt      time.Time
	UpdatedAt      time.Time

	PullRequests    []PullRequest    `gorm:"foreignKey:RepoID"`
	AssignmentRules []AssignmentRule `gorm:"foreignKey:RepoID"`
}

type PullRequest struct {
	ID        uint `gorm:"primarykey"`
	RepoID    uint `gorm:"uniqueIndex:idx_pr_repo_number;not null"`
	Number    int  `gorm:"uniqueIndex:idx_pr_repo_number;not null"`
	Title     string
	Author    string
	HeadSHA   string
	CreatedAt time.Time

	Reviews []Review `gorm:"foreignKey:PRID"`
}

type Review struct {
	ID           uint   `gorm:"primarykey"`
	PRID         uint   `gorm:"column:pr_id;index;not null"`
	Status       string // APPROVE | REQUEST_CHANGES | COMMENT
	Score        int
	Summary      string
	TokenUsage   int
	InputTokens  int
	OutputTokens int
	LatencyMS    int64
	CreatedAt    time.Time

	Comments    []ReviewComment `gorm:"foreignKey:ReviewID"`
	Assignments []Assignment    `gorm:"foreignKey:ReviewID"`
}

type ReviewComment struct {
	ID        uint `gorm:"primarykey"`
	ReviewID  uint `gorm:"index;not null"`
	Path      string
	Line      int
	Side      string
	Body      string
	Severity  string
	Priority  string // p0|p1|p2|p3
	CreatedAt time.Time
}

// WebhookDelivery tracks processed GitHub delivery IDs to prevent duplicate reviews.
type WebhookDelivery struct {
	DeliveryID  string    `gorm:"primarykey"`
	ProcessedAt time.Time `gorm:"not null"`
	EventType   string    // pull_request | issue_comment | pull_request_review_comment
	Action      string    // opened | synchronize | re_review | created
	Owner       string
	Repo        string
	PRNumber    int
	Status      string // enqueued | duplicate | skipped | failed
}

// ProviderHealth tracks the last health-check result per configured AI provider.
type ProviderHealth struct {
	ID               uint `gorm:"primarykey"`
	ProviderConfigID uint `gorm:"uniqueIndex;not null"`
	LastTestedAt     time.Time
	LatencyMS        int64
	OK               bool
	ErrorMsg         string
	UpdatedAt        time.Time
}

type AssignmentRule struct {
	ID        uint   `gorm:"primarykey"`
	RepoID    uint   `gorm:"index;not null"`
	Strategy  string `gorm:"not null"` // round-robin|codeowners|load-balanced
	Config    datatypes.JSON
	CreatedAt time.Time
}

type Assignment struct {
	ID            uint   `gorm:"primarykey"`
	ReviewID      uint   `gorm:"index;not null"`
	AssigneeLogin string `gorm:"not null"`
	AssignedAt    time.Time
	CompletedAt   *time.Time
}

type TeamMember struct {
	ID             uint   `gorm:"primarykey"`
	InstallationID uint   `gorm:"index;not null"`
	Login          string `gorm:"not null"`
	Role           string `gorm:"default:reviewer"` // reviewer|admin
	SlackUserID    string // Slack member ID (e.g. U012AB3CD) for DM notifications
	CreatedAt      time.Time
}

// RepoAccess records which GitHub logins may access a repository, synced from the
// repo's GitHub collaborators. Used to scope a non-admin user's repo visibility so
// they only see repos they actually have access to on GitHub. (Owners/admins are
// exempt and see every repo in the installation.)
type RepoAccess struct {
	ID             uint   `gorm:"primarykey"`
	InstallationID uint   `gorm:"index;not null"`
	RepoID         uint   `gorm:"uniqueIndex:idx_repo_login;not null"`
	Login          string `gorm:"uniqueIndex:idx_repo_login;not null"`
	CreatedAt      time.Time
}

// NotificationConfig stores a notification channel configuration per installation (or per repo).
// Config is a JSON blob whose shape depends on Channel: slack | email | webhook.
// RepoID == NULL means installation-wide default; a non-null RepoID overrides the default for that repo.
type NotificationConfig struct {
	ID             uint           `gorm:"primarykey"`
	InstallationID uint           `gorm:"index;not null"`
	RepoID         *uint          `gorm:"index"`
	Channel        string         `gorm:"not null"` // slack | email | webhook
	Config         datatypes.JSON `gorm:"not null"`
	Enabled        bool           `gorm:"default:true"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// SystemConfig stores key/value pairs for global installation settings.
type SystemConfig struct {
	ID    uint   `gorm:"primarykey"`
	Key   string `gorm:"uniqueIndex;not null"`
	Value string `gorm:"not null"`
}

// BotComment tracks every GitHub review comment the bot has posted so that when
// a human replies we can recognise the thread and respond appropriately.
type BotComment struct {
	ID              uint  `gorm:"primarykey"`
	ReviewID        uint  `gorm:"index;not null"`
	GithubCommentID int64 `gorm:"uniqueIndex;not null"`
	Path            string
	Line            int
	Body            string
	Resolved        bool
	CreatedAt       time.Time
}

// BotReply records comment threads where the bot has already replied to prevent double-posting.
type BotReply struct {
	ID              uint      `gorm:"primarykey"`
	GithubCommentID int64     `gorm:"uniqueIndex;not null"` // root comment ID of the thread
	RepliedAt       time.Time `gorm:"not null"`
}

// CommentFeedback stores thumbs-up/down votes on individual AI review comments.
type CommentFeedback struct {
	ID              uint   `gorm:"primarykey"`
	ReviewCommentID uint   `gorm:"index;not null"`
	UserLogin       string `gorm:"not null"`
	Vote            int    `gorm:"not null"` // 1 = thumbs up, -1 = thumbs down
	CreatedAt       time.Time
}

// GithubAppConfig stores the GitHub App credentials used for installation-based auth.
// All sensitive fields hold AES-GCM ciphertext; never store raw secrets.
type GithubAppConfig struct {
	ID                     uint   `gorm:"primarykey"`
	AppID                  int64  `gorm:"not null"`
	PrivateKeyEncrypted    string `gorm:"not null"`
	WebhookSecretEncrypted string
	GitHubTokenEncrypted   string
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

// AuditLog records administrative actions for compliance.
type AuditLog struct {
	ID         uint   `gorm:"primarykey"`
	ActorLogin string `gorm:"not null;index"`
	ActorID    uint   `gorm:"index"`
	Action     string `gorm:"not null"` // e.g. repo.enable, provider.create, team_member.add
	EntityType string `gorm:"not null"` // repo | provider | team_member | config | user
	EntityID   string `gorm:"index"`
	Before     datatypes.JSON
	After      datatypes.JSON
	IPAddress  string
	CreatedAt  time.Time `gorm:"index"`
}

// APIToken allows programmatic API access. Hash is SHA-256(raw token). Prefix stores first 8 chars for display.
type APIToken struct {
	ID         uint   `gorm:"primarykey"`
	UserID     uint   `gorm:"index;not null"`
	Name       string `gorm:"not null"`
	Scope      string `gorm:"not null"` // read | readwrite
	Hash       string `gorm:"uniqueIndex;not null"`
	Prefix     string `gorm:"not null"`
	LastUsedAt *time.Time
	ExpiresAt  *time.Time
	CreatedAt  time.Time
}

// JiraConfig stores a single Jira integration configuration (per installation).
type JiraConfig struct {
	ID                uint   `gorm:"primarykey"`
	BaseURL           string `gorm:"not null"` // e.g. https://yourco.atlassian.net
	Email             string `gorm:"not null"` // Jira account email
	APITokenEncrypted string `gorm:"not null"` // AES-GCM ciphertext of the Jira API token
	Enabled           bool   `gorm:"default:true"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// SlackAppConfig stores the two-way Slack bot credentials (single global config).
// Sensitive fields hold AES-GCM ciphertext; never store raw secrets.
type SlackAppConfig struct {
	ID                     uint   `gorm:"primarykey"`
	SigningSecretEncrypted string `gorm:"not null"` // verifies inbound slash commands & events
	BotTokenEncrypted      string // xoxb- token used for chat.postMessage replies
	Enabled                bool   `gorm:"default:true"`
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

// OIDCConfig stores a single OIDC SSO provider (Okta, Azure AD, Google Workspace, etc.).
type OIDCConfig struct {
	ID                    uint   `gorm:"primarykey"`
	Issuer                string `gorm:"not null"`
	ClientID              string `gorm:"not null"`
	ClientSecretEncrypted string `gorm:"not null"`
	RedirectURL           string
	AttributeMapping      datatypes.JSON // {"email":"email","role_claim":"groups"}
	RoleMapping           datatypes.JSON // {"owner":["owners"],"admin":["admins"],"viewer":["*"]}
	Enforced              bool           // when true, GitHub OAuth login is disabled
	Enabled               bool           `gorm:"default:true"`
	CreatedAt             time.Time
	UpdatedAt             time.Time
}
