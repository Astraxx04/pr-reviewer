//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"gorm.io/gorm"

	"pr-reviewer/internal/db"
	"pr-reviewer/internal/db/models"
	"pr-reviewer/internal/db/repo"
	"pr-reviewer/internal/http/handlers"
	prHttp "pr-reviewer/internal/http"
)

// gormDB is initialised once in TestMain and shared across all tests.
var gormDB *gorm.DB

func TestMain(m *testing.M) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		fmt.Println("skipping: DATABASE_URL not set")
		os.Exit(0)
	}

	var err error
	gormDB, err = db.Connect(dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "integration: db connect: %v\n", err)
		os.Exit(1)
	}
	if err := db.RunMigrations(dsn); err != nil {
		fmt.Fprintf(os.Stderr, "integration: db migrate: %v\n", err)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

// noopWebhookHandler satisfies WebhookHandlerIface for tests that don't exercise webhooks.
type noopWebhookHandler struct{}

func (n *noopWebhookHandler) Handle(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// TestDB_ConnectAndMigrate verifies the DB is reachable and the expected tables exist.
func TestDB_ConnectAndMigrate(t *testing.T) {
	sqlDB, err := gormDB.DB()
	if err != nil {
		t.Fatalf("gormDB.DB(): %v", err)
	}
	if err := sqlDB.PingContext(context.Background()); err != nil {
		t.Fatalf("ping failed: %v", err)
	}

	expectedTables := []string{
		"users",
		"installations",
		"repositories",
		"pull_requests",
		"reviews",
		"review_comments",
		"webhook_deliveries",
		"team_members",
		"provider_configs",
		"notification_configs",
	}
	for _, table := range expectedTables {
		var count int64
		row := sqlDB.QueryRowContext(
			context.Background(),
			"SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public' AND table_name = $1",
			table,
		)
		if err := row.Scan(&count); err != nil {
			t.Errorf("query information_schema for %q: %v", table, err)
			continue
		}
		if count == 0 {
			t.Errorf("table %q not found in information_schema", table)
		}
	}
}

// TestAPI_HealthEndpoint verifies GET /healthz returns 200.
func TestAPI_HealthEndpoint(t *testing.T) {
	healthHandler := handlers.NewHealthHandler(gormDB)
	router := prHttp.NewRouter(prHttp.RouterConfig{
		WebhookHandler: &noopWebhookHandler{},
		HealthHandler:  healthHandler,
		JWTSecret:      "test-secret-for-integration-tests",
		DB:             gormDB,
	})

	srv := httptest.NewServer(router)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

// TestAPI_SetupStatus verifies GET /api/setup/status returns 200 with valid JSON
// containing database_ok: true.
func TestAPI_SetupStatus(t *testing.T) {
	setupHandler := handlers.NewSetupHandler(gormDB)
	router := prHttp.NewRouter(prHttp.RouterConfig{
		WebhookHandler: &noopWebhookHandler{},
		SetupHandler:   setupHandler,
		JWTSecret:      "test-secret-for-integration-tests",
		DB:             gormDB,
	})

	srv := httptest.NewServer(router)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/setup/status")
	if err != nil {
		t.Fatalf("GET /api/setup/status: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var body struct {
		DatabaseOK bool `json:"database_ok"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.DatabaseOK {
		t.Error("expected database_ok to be true")
	}
}

// TestAPI_ProtectedRoutes_Require401 verifies that protected routes reject unauthenticated requests.
func TestAPI_ProtectedRoutes_Require401(t *testing.T) {
	router := prHttp.NewRouter(prHttp.RouterConfig{
		WebhookHandler: &noopWebhookHandler{},
		JWTSecret:      "test-secret-for-integration-tests",
		DB:             gormDB,
	})

	srv := httptest.NewServer(router)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/repos")
	if err != nil {
		t.Fatalf("GET /api/repos: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

// TestDeliveryRepo_CreateAndList creates a WebhookDelivery, lists it, and verifies it appears.
func TestDeliveryRepo_CreateAndList(t *testing.T) {
	deliveryRepo := repo.NewDeliveryRepo(gormDB)
	ctx := context.Background()

	deliveryID := fmt.Sprintf("test-delivery-%d", time.Now().UnixNano())
	d := &models.WebhookDelivery{
		DeliveryID:  deliveryID,
		ProcessedAt: time.Now(),
		EventType:   "pull_request",
		Action:      "opened",
		Owner:       "test-owner",
		Repo:        "test-repo",
		PRNumber:    99,
		Status:      "enqueued",
	}

	if err := deliveryRepo.RecordDelivery(ctx, d); err != nil {
		t.Fatalf("RecordDelivery: %v", err)
	}

	// Clean up after the test regardless of outcome.
	t.Cleanup(func() {
		gormDB.Where("delivery_id = ?", deliveryID).Delete(&models.WebhookDelivery{})
	})

	rows, _, err := deliveryRepo.List(ctx, 100, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	found := false
	for _, row := range rows {
		if row.DeliveryID == deliveryID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("delivery %q not found in list results", deliveryID)
	}

	// Verify IsProcessed reports true for the created delivery.
	if !deliveryRepo.IsProcessed(ctx, deliveryID) {
		t.Errorf("IsProcessed(%q) = false, want true", deliveryID)
	}
}
