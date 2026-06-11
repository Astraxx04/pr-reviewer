package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
	"gorm.io/gorm"

	"pr-reviewer/internal/ai"
	"pr-reviewer/internal/ai/agents"
	"pr-reviewer/internal/ai/embeddings"
	"pr-reviewer/internal/ai/llm"
	"pr-reviewer/internal/ai/llm/adapters"
	"pr-reviewer/internal/ai/rag"
	"pr-reviewer/internal/config"
	"pr-reviewer/internal/db"
	dbModels "pr-reviewer/internal/db/models"
	"pr-reviewer/internal/db/repo"
	"pr-reviewer/internal/events"
	gh "pr-reviewer/internal/github"
	prHttp "pr-reviewer/internal/http"
	"pr-reviewer/internal/http/handlers"
	"pr-reviewer/internal/http/middleware"
	"pr-reviewer/internal/jobs"
	"pr-reviewer/internal/metrics"
	"pr-reviewer/internal/notifications"
	"pr-reviewer/internal/pr"
	"pr-reviewer/internal/review"
	"pr-reviewer/internal/telemetry"
	"pr-reviewer/pkg/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	cfg.Validate()
	log := logger.New(cfg.AppEnv)
	// Make the configured logger the slog default so package-level slog calls
	// (e.g. outbound API-call logging in internal/github, internal/notifications)
	// share the same handler and formatting.
	slog.SetDefault(log.Logger)
	ctx := context.Background()

	// Initialise OpenTelemetry (no-op if OTEL_EXPORTER_OTLP_ENDPOINT is unset).
	otelShutdown, err := telemetry.Init(ctx)
	if err != nil {
		log.Error("otel init failed", "error", err)
		os.Exit(1)
	}

	registry := llm.NewProviderRegistry()

	eventHub := events.NewHub()

	ghClient := gh.NewClient(cfg.GitHubToken)
	prService := pr.NewService(ghClient, log)

	orchestrator := ai.NewAgentOrchestrator()
	orchestrator.RegisterAgent("code-review", agents.NewCodeReviewAgent(registry))
	orchestrator.RegisterAgent("security", agents.NewSecurityAgent(registry))
	orchestrator.RegisterAgent("performance", agents.NewPerformanceAgent(registry))
	orchestrator.RegisterAgent("database", agents.NewDatabaseAgent(registry))
	orchestrator.RegisterAgent("conversation", agents.NewConversationAgent(registry))
	aggregator := review.NewAggregator()

	var embedder embeddings.Embedder // populated from DB after connect

	aiService := ai.NewReviewer(cfg, log, embedder, nil, orchestrator)

	var (
		riverClient  *river.Client[pgx.Tx]
		deliveryRepo *repo.DeliveryRepo
		gormDB       *gorm.DB
		indexer      *rag.Indexer // non-nil when an embedding provider is configured
	)

	if cfg.DatabaseURL != "" {
		gormDB, err = db.Connect(cfg.DatabaseURL)
		if err != nil {
			log.Error("database connection failed", "error", err)
			os.Exit(1)
		}
		// Report pgvector availability: the code_embeddings table has a
		// vector(1536) column (created by the migrations), so surface a clear
		// message here rather than leaving RAG features silently broken.
		if ok, version, vErr := db.EnsurePgVector(gormDB); ok {
			log.Info("pgvector available", "version", version)
			if version < "0.5.0" {
				log.Warn("pgvector < 0.5.0: HNSW index unsupported, vector search will use slower exact scan", "version", version)
			}
		} else {
			log.Warn("pgvector NOT available — RAG/embeddings disabled; install the extension to enable vector search", "error", vErr)
		}
		// MIGRATE_ONLY mode — used by the docker-compose migrate service to apply
		// all migrations (app schema via golang-migrate + River queue) and exit.
		if cfg.MigrateOnly {
			if cfg.SkipMigrations {
				log.Info("SKIP_MIGRATIONS set — skipping migrations and exiting")
				return
			}
			if err := db.RunMigrations(cfg.DatabaseURL); err != nil {
				log.Error("app migrations failed", "error", err)
				os.Exit(1)
			}
			pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
			if err != nil {
				log.Error("pgxpool connect failed", "error", err)
				os.Exit(1)
			}
			migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
			if err != nil {
				pool.Close()
				log.Error("river migrator init failed", "error", err)
				os.Exit(1)
			}
			if _, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
				pool.Close()
				log.Error("river migration failed", "error", err)
				os.Exit(1)
			}
			pool.Close()
			log.Info("migrations applied, exiting")
			return
		}

		// Migrations are an explicit step (run `migrate up`, or the docker
		// migrate service). Verify the schema is current and fail fast if it is
		// behind rather than running against a stale schema.
		if err := db.VerifySchema(cfg.DatabaseURL); err != nil {
			log.Error("database schema check failed", "error", err)
			os.Exit(1)
		}
		log.Info("database connected; schema verified")

		deliveryRepo = repo.NewDeliveryRepo(gormDB)

		pgxCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
		if err != nil {
			log.Error("pgxpool config parse failed", "error", err)
			os.Exit(1)
		}
		dbPool, err := pgxpool.NewWithConfig(ctx, pgxCfg)
		if err != nil {
			log.Error("pgxpool connect failed", "error", err)
			os.Exit(1)
		}

		loadDBProvidersIntoRegistry(gormDB, cfg.EncryptionKey, registry, log)
		loadDBGithubSecrets(gormDB, cfg)

		if dbEmbedder := buildEmbedderFromDB(gormDB, cfg.EncryptionKey, log); dbEmbedder != nil {
			embedder = dbEmbedder
		}

		// Rebuild GitHub client if a DB token was loaded and overrides the env var.
		if cfg.GitHubToken != "" {
			ghClient = gh.NewClient(cfg.GitHubToken)
			prService = pr.NewService(ghClient, log)
		}

		// Wire RAG retriever + indexer when embedder is available.
		var retriever *rag.PgvectorRetriever
		if embedder != nil {
			retriever = rag.NewPgvectorRetriever(gormDB, embedder)
			indexer = rag.NewIndexer(gormDB, embedder)
			aiService = ai.NewReviewer(cfg, log, embedder, retriever, orchestrator)
		}

		// Key used to encrypt channel secrets (SMTP password, webhook secret) at rest.
		notifications.SetEncryptionKey(cfg.EncryptionKey)
		notifService := notifications.NewService(gormDB)

		workers := river.NewWorkers()
		river.AddWorker(workers, &jobs.ReviewWorker{
			PRService:     prService,
			AIService:     aiService,
			Aggregator:    aggregator,
			GHClient:      ghClient,
			DB:            gormDB,
			Log:           log,
			Indexer:       indexer,
			NotifService:  notifService,
			EventHub:      eventHub,
			EncryptionKey: cfg.EncryptionKey,
			FrontendURL:   cfg.FrontendURL,
		})
		river.AddWorker(workers, &jobs.ConversationWorker{
			GHClient:     ghClient,
			DB:           gormDB,
			Log:          log,
			Orchestrator: orchestrator,
		})
		river.AddWorker(workers, &jobs.TeamSyncWorker{
			DB:          gormDB,
			Log:         log,
			GitHubToken: cfg.GitHubToken,
		})
		river.AddWorker(workers, &jobs.EmailDigestWorker{
			DB:  gormDB,
			Log: log,
		})
		var indexAllReposWorker *jobs.IndexAllReposWorker
		if indexer != nil {
			river.AddWorker(workers, &jobs.IndexRepoWorker{
				GHClient: ghClient,
				DB:       gormDB,
				Indexer:  indexer,
				Log:      log,
			})
			indexAllReposWorker = &jobs.IndexAllReposWorker{DB: gormDB, Log: log}
			river.AddWorker(workers, indexAllReposWorker)
		}

		teamSyncPeriodic := river.NewPeriodicJob(
			river.PeriodicInterval(6*time.Hour),
			func() (river.JobArgs, *river.InsertOpts) {
				return jobs.TeamSyncJobArgs{}, nil
			},
			&river.PeriodicJobOpts{RunOnStart: false},
		)

		// Email digests: daily and weekly cadences. The worker only emails configs
		// whose digest setting matches the period, so both are safe to always schedule.
		dailyDigestPeriodic := river.NewPeriodicJob(
			river.PeriodicInterval(24*time.Hour),
			func() (river.JobArgs, *river.InsertOpts) {
				return jobs.EmailDigestJobArgs{Period: "daily"}, nil
			},
			&river.PeriodicJobOpts{RunOnStart: false},
		)
		weeklyDigestPeriodic := river.NewPeriodicJob(
			river.PeriodicInterval(7*24*time.Hour),
			func() (river.JobArgs, *river.InsertOpts) {
				return jobs.EmailDigestJobArgs{Period: "weekly"}, nil
			},
			&river.PeriodicJobOpts{RunOnStart: false},
		)

		periodicJobs := []*river.PeriodicJob{teamSyncPeriodic, dailyDigestPeriodic, weeklyDigestPeriodic}
		if indexer != nil {
			// 9.3: Weekly full re-index of all enabled repositories.
			periodicJobs = append(periodicJobs, river.NewPeriodicJob(
				river.PeriodicInterval(7*24*time.Hour),
				func() (river.JobArgs, *river.InsertOpts) {
					return jobs.IndexAllReposJobArgs{}, nil
				},
				&river.PeriodicJobOpts{RunOnStart: false},
			))
		}

		riverClient, err = river.NewClient(riverpgxv5.New(dbPool), &river.Config{
			Queues:       map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: 5}},
			Workers:      workers,
			PeriodicJobs: periodicJobs,
		})
		if err != nil {
			log.Error("river client init failed", "error", err)
			os.Exit(1)
		}
		if err := riverClient.Start(ctx); err != nil {
			log.Error("river workers failed to start", "error", err)
			os.Exit(1)
		}
		log.Info("river workers started")

		// Wire enqueuer for index fan-out worker now that riverClient exists.
		if indexAllReposWorker != nil {
			indexAllReposWorker.Enqueuer = riverClient
		}

		// Poll River queue depth for Prometheus every 30 s.
		go pollQueueDepth(ctx, gormDB, log)
	}

	var webhookHandler prHttp.WebhookHandlerIface
	if riverClient != nil {
		wh := prHttp.NewWebhookHandler(log, deliveryRepo, riverClient, cfg.GitHubWebhookSecret)
		if gormDB != nil {
			wh = wh.WithDB(gormDB)
		}
		// Reuse REQUIRED_GITHUB_ORG to lock installations to a single org.
		wh = wh.WithAllowedOrg(cfg.RequiredGithubOrg)
		webhookHandler = wh
	} else {
		log.Info("no DATABASE_URL — using in-process handler (jobs lost on restart)")
		webhookHandler = prHttp.NewInProcessWebhookHandler(log, prService, aiService, aggregator, ghClient, cfg.GitHubWebhookSecret)
	}

	routerCfg := prHttp.RouterConfig{
		WebhookHandler: webhookHandler,
		JWTSecret:      cfg.JWTSecret,
		AllowedOrigin:  cfg.CORSOrigins,
	}

	if gormDB != nil {
		routerCfg.DB = gormDB
		routerCfg.HealthHandler = handlers.NewHealthHandler(gormDB)
		routerCfg.AuthHandler = handlers.NewAuthHandler(
			cfg.GitHubClientID, cfg.GitHubClientSecret,
			cfg.JWTSecret, cfg.FrontendURL, cfg.ServerURL, gormDB,
			cfg.RequiredGithubOrg, cfg.InviteOnly, cfg.JWTTTLHours,
		)
		repoHandler := handlers.NewRepoHandler(gormDB, cfg.EncryptionKey)
		if riverClient != nil && indexer != nil {
			repoHandler = repoHandler.WithEnqueuer(riverClient)
		}
		routerCfg.RepoHandler = repoHandler
		routerCfg.ReviewHandler = handlers.NewReviewHandler(gormDB)
		routerCfg.DashHandler = handlers.NewDashboardHandler(gormDB)
		routerCfg.TeamHandler = handlers.NewTeamHandler(gormDB, riverClient, cfg.FrontendURL)
		routerCfg.AssignHandler = handlers.NewAssignmentHandler(gormDB)
		routerCfg.AnalyticsHandler = handlers.NewAnalyticsHandler(gormDB)
		routerCfg.ProviderHandler = handlers.NewProviderHandler(gormDB, cfg.EncryptionKey)
		routerCfg.SetupHandler = handlers.NewSetupHandler(gormDB)
		routerCfg.GithubAppHandler = handlers.NewGithubAppHandler(gormDB, cfg.EncryptionKey)
		routerCfg.UserHandler = handlers.NewUserHandler(gormDB)
		routerCfg.SessionHandler = handlers.NewSessionHandler(gormDB)
		routerCfg.SystemMetricsHandler = handlers.NewSystemMetricsHandler(gormDB)
		routerCfg.WebhookDeliveryHandler = handlers.NewWebhookDeliveryHandler(gormDB)
		routerCfg.ExportHandler = handlers.NewExportHandler(gormDB)
		routerCfg.PRHandler = handlers.NewPRHandler(gormDB, riverClient).WithGHClient(ghClient)
		routerCfg.EventsHandler = handlers.NewEventsHandler(eventHub, cfg.JWTSecret)
		routerCfg.NotificationHandler = handlers.NewNotificationHandler(gormDB).WithEnqueuer(riverClient)
		routerCfg.FeedbackHandler = handlers.NewFeedbackHandler(gormDB)
		routerCfg.ExplainHandler = handlers.NewExplainHandler(gormDB, aiService)
		routerCfg.AuditHandler = handlers.NewAuditHandler(gormDB)
		routerCfg.RetentionHandler = handlers.NewRetentionHandler(gormDB)
		routerCfg.SSOHandler = handlers.NewSSOHandler(gormDB, cfg.EncryptionKey, cfg.ServerURL, cfg.FrontendURL, cfg.JWTSecret, cfg.JWTTTLHours)
		routerCfg.APITokenHandler = handlers.NewAPITokenHandler(gormDB)
		routerCfg.IntegrationHandler = handlers.NewIntegrationHandler(gormDB, cfg.EncryptionKey)
		routerCfg.SlackAppHandler = handlers.NewSlackAppHandler(gormDB, cfg.EncryptionKey, cfg.ServerURL, riverClient, log)
		routerCfg.RateLimiter = middleware.NewRateLimiter(1000) // 1000 req/hour default

		go pollProviderHealth(ctx, gormDB, cfg.EncryptionKey, log)
		go purgeOldDeliveries(ctx, gormDB, log)
		go runRetentionPurge(ctx, gormDB, log)
	}

	srv := &http.Server{
		Addr:    ":" + cfg.ServerPort,
		Handler: prHttp.NewRouter(routerCfg),
	}

	go func() {
		log.Info("server started", "port", cfg.ServerPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if riverClient != nil {
		if err := riverClient.Stop(shutCtx); err != nil {
			log.Error("river stop error", "error", err)
		}
	}
	if err := srv.Shutdown(shutCtx); err != nil {
		log.Error("http shutdown error", "error", err)
	}
	if err := otelShutdown(shutCtx); err != nil {
		log.Error("otel shutdown error", "error", err)
	}
}

// pollProviderHealth tests every configured AI provider every 30 minutes and stores the result.
func pollProviderHealth(ctx context.Context, gormDB *gorm.DB, encKey string, log *logger.Logger) {
	tick := time.NewTicker(30 * time.Minute)
	defer tick.Stop()
	checkProviders(ctx, gormDB, encKey, log)
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			checkProviders(ctx, gormDB, encKey, log)
		}
	}
}

func checkProviders(ctx context.Context, gormDB *gorm.DB, encKey string, log *logger.Logger) {
	var provs []struct {
		ID              uint
		Type            string
		BaseURL         string
		DefaultModel    string
		APIKeyEncrypted string
	}
	if err := gormDB.WithContext(ctx).
		Table("provider_configs").
		Select("id, type, base_url, default_model, api_key_encrypted").
		Scan(&provs).Error; err != nil {
		log.Error("provider health: list failed", "error", err)
		return
	}
	for _, p := range provs {
		apiKey := p.APIKeyEncrypted
		if apiKey != "" && encKey != "" {
			if decrypted, err := db.Decrypt(apiKey, encKey); err == nil {
				apiKey = decrypted
			}
		}
		start := time.Now()
		ok, errMsg := testProviderHealth(p.Type, p.BaseURL, apiKey, p.DefaultModel)
		latency := time.Since(start).Milliseconds()

		health := &dbModels.ProviderHealth{
			ProviderConfigID: p.ID,
			LastTestedAt:     start,
			LatencyMS:        latency,
			OK:               ok,
			ErrorMsg:         errMsg,
			UpdatedAt:        time.Now(),
		}
		var existing dbModels.ProviderHealth
		if gormDB.Where("provider_config_id = ?", p.ID).First(&existing).Error == nil {
			health.ID = existing.ID
		}
		gormDB.Save(health)
		log.Info("provider health checked", "id", p.ID, "ok", ok, "ms", latency)
	}
}

func testProviderHealth(providerType, baseURL, apiKey, model string) (bool, string) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return handlers.TestProviderConnection(ctx, providerType, baseURL, apiKey, model)
}

// purgeOldDeliveries deletes webhook delivery records older than 7 days, running daily.
func purgeOldDeliveries(ctx context.Context, gormDB *gorm.DB, log *logger.Logger) {
	tick := time.NewTicker(24 * time.Hour)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			cutoff := time.Now().AddDate(0, 0, -7)
			if err := gormDB.WithContext(ctx).
				Where("processed_at < ?", cutoff).
				Delete(&dbModels.WebhookDelivery{}).Error; err != nil {
				log.Error("delivery purge failed", "error", err)
			}
		}
	}
}

// runRetentionPurge checks retention settings daily and purges old reviews accordingly.
func runRetentionPurge(ctx context.Context, gormDB *gorm.DB, log *logger.Logger) {
	tick := time.NewTicker(24 * time.Hour)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			settings := handlers.LoadRetentionSettings(gormDB)
			if settings.ReviewRetentionDays > 0 {
				n, err := handlers.PurgeOldReviews(gormDB, settings.ReviewRetentionDays)
				if err != nil {
					log.Error("retention purge failed", "error", err)
				} else if n > 0 {
					log.Info("purged old reviews", "count", n)
				}
			}
		}
	}
}

// pollQueueDepth updates the review_queue_depth Prometheus gauge every 30 s.
func pollQueueDepth(ctx context.Context, gormDB *gorm.DB, log *logger.Logger) {
	tick := time.NewTicker(30 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			var count int64
			if err := gormDB.WithContext(ctx).
				Raw("SELECT count(*) FROM river_job WHERE kind = 'review' AND state = 'available'").
				Scan(&count).Error; err != nil {
				log.Error("queue depth query failed", "error", err)
				continue
			}
			metrics.ReviewQueueDepth.Set(float64(count))
		}
	}
}

// buildEmbedderFromDB finds the first ProviderConfig with SupportsEmbeddings=true and
// builds an embedder from it. Returns nil if none is found or the key cannot be decrypted.
func buildEmbedderFromDB(gormDB *gorm.DB, encKey string, log *logger.Logger) embeddings.Embedder {
	var prov struct {
		ID              uint
		Type            string
		BaseURL         string
		EmbeddingModel  string
		APIKeyEncrypted string
	}
	if err := gormDB.Table("provider_configs").
		Where("supports_embeddings = true").
		First(&prov).Error; err != nil {
		return nil // no embedding provider in DB
	}
	apiKey := prov.APIKeyEncrypted
	if apiKey != "" && encKey != "" {
		if decrypted, err := db.Decrypt(apiKey, encKey); err == nil {
			apiKey = decrypted
		}
	}
	switch prov.Type {
	case "openai", "openai_compatible":
		model := prov.EmbeddingModel
		if model == "" {
			model = "text-embedding-3-small"
		}
		log.Info("RAG enabled from DB provider", "type", prov.Type, "embedding_model", model)
		return embeddings.NewOpenAIEmbedder(apiKey, model)
	case "ollama":
		model := prov.EmbeddingModel
		if model == "" {
			model = "nomic-embed-text"
		}
		log.Info("RAG enabled from DB provider", "type", "ollama", "embedding_model", model)
		return embeddings.NewOllamaEmbedder(prov.BaseURL, model)
	}
	return nil
}

// loadDBProvidersIntoRegistry reads provider_configs from the DB and registers them.
func loadDBProvidersIntoRegistry(gormDB *gorm.DB, encKey string, registry *llm.ProviderRegistry, log *logger.Logger) {
	var provs []struct {
		ID              uint
		Name            string
		Type            string
		APIKeyEncrypted string
		BaseURL         string
		DefaultModel    string
	}
	if err := gormDB.Table("provider_configs").Find(&provs).Error; err != nil {
		return
	}
	for _, p := range provs {
		apiKey := p.APIKeyEncrypted
		if apiKey != "" && encKey != "" {
			if dec, err := db.Decrypt(apiKey, encKey); err == nil {
				apiKey = dec
			}
		}
		model := p.DefaultModel
		id := fmt.Sprintf("db-%d", p.ID)
		switch p.Type {
		case "anthropic":
			if model == "" {
				model = "claude-sonnet-4-6"
			}
			registry.Register(id, model, adapters.NewAnthropic(apiKey, p.BaseURL))
		case "openai":
			if model == "" {
				model = "gpt-4o"
			}
			registry.Register(id, model, adapters.NewOpenAI(apiKey, p.BaseURL, ""))
		case "openai_compatible":
			if model == "" {
				model = "gpt-4o"
			}
			registry.Register(id, model, adapters.NewOpenAI(apiKey, p.BaseURL, ""))
		case "ollama":
			if model == "" {
				model = "llama3"
			}
			registry.Register(id, model, adapters.NewOllama(p.BaseURL, model))
		default:
			continue
		}
		log.Info("registered DB provider", "name", p.Name, "type", p.Type)
	}
}

// loadDBGithubSecrets reads WebhookSecret and GitHubToken from the DB-stored GithubAppConfig
// and writes them into cfg, overriding any env-var values that are empty.
func loadDBGithubSecrets(gormDB *gorm.DB, cfg *config.Config) {
	var appCfg dbModels.GithubAppConfig
	if err := gormDB.First(&appCfg).Error; err != nil {
		return
	}
	encKey := cfg.EncryptionKey
	if appCfg.WebhookSecretEncrypted != "" {
		if dec, err := db.Decrypt(appCfg.WebhookSecretEncrypted, encKey); err == nil {
			cfg.GitHubWebhookSecret = dec
		}
	}
	if appCfg.GitHubTokenEncrypted != "" {
		if dec, err := db.Decrypt(appCfg.GitHubTokenEncrypted, encKey); err == nil {
			cfg.GitHubToken = dec
		}
	}
}
