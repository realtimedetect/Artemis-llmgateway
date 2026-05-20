package database

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// Connect establishes a connection to MariaDB.
func Connect() (*sql.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci",
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_NAME"),
	)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("sql.Open: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("db.Ping: %w", err)
	}

	return db, nil
}

// Migrate runs DDL statements to set up or update the schema.
func Migrate(db *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id         CHAR(36)     NOT NULL PRIMARY KEY,
			email      VARCHAR(255) NOT NULL UNIQUE,
			password   VARCHAR(255) NOT NULL,
			role       ENUM('admin','user') NOT NULL DEFAULT 'user',
			created_at DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`,

		`CREATE TABLE IF NOT EXISTS plans (
			id                  VARCHAR(32)  NOT NULL PRIMARY KEY,
			name                VARCHAR(120) NOT NULL,
			monthly_token_limit BIGINT,
			description         VARCHAR(500) NOT NULL DEFAULT '',
			created_at          DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at          DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`,

		`INSERT INTO plans (id, name, monthly_token_limit, description)
		 VALUES
			('basic', 'Basic', 5000000, 'Up to 5 million tokens per month'),
			('professional', 'Professional', NULL, 'Unlimited monthly tokens')
		 ON DUPLICATE KEY UPDATE
			name = VALUES(name),
			monthly_token_limit = VALUES(monthly_token_limit),
			description = VALUES(description);`,

		`CREATE TABLE IF NOT EXISTS providers (
			id         CHAR(36)     NOT NULL PRIMARY KEY,
			name       VARCHAR(100) NOT NULL,
			base_url   VARCHAR(500) NOT NULL,
			adapter    VARCHAR(40)  NOT NULL DEFAULT 'openai',
			api_version VARCHAR(40) NOT NULL DEFAULT '',
			api_key    TEXT         NOT NULL,
			api_keys_json LONGTEXT  NOT NULL DEFAULT '[]',
			enabled    TINYINT(1)   NOT NULL DEFAULT 1,
			created_at DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`,

		`CREATE TABLE IF NOT EXISTS cache_configs (
			id                  CHAR(36)     NOT NULL PRIMARY KEY,
			user_id             CHAR(36)     NOT NULL UNIQUE,
			enabled             TINYINT(1)   NOT NULL DEFAULT 0,
			semantic_enabled    TINYINT(1)   NOT NULL DEFAULT 0,
			semantic_threshold  DECIMAL(4,3) NOT NULL DEFAULT 0.900,
			semantic_max_candidates INT      NOT NULL DEFAULT 30,
			semantic_embedding_model VARCHAR(120) NOT NULL DEFAULT 'text-embedding-3-small',
			redis_addr          VARCHAR(255) NOT NULL DEFAULT 'localhost:6379',
			redis_username      VARCHAR(255) NOT NULL DEFAULT '',
			redis_password      TEXT,
			redis_db            INT          NOT NULL DEFAULT 0,
			default_ttl_seconds INT          NOT NULL DEFAULT 300,
			key_prefix          VARCHAR(100) NOT NULL DEFAULT 'llm-gw',
			created_at          DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at          DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`,

		`CREATE TABLE IF NOT EXISTS routing_configs (
			id                  CHAR(36)     NOT NULL PRIMARY KEY,
			user_id             CHAR(36)     NOT NULL UNIQUE,
			smart_enabled       TINYINT(1)   NOT NULL DEFAULT 0,
			cost_weight         DECIMAL(4,3) NOT NULL DEFAULT 0.700,
			performance_weight  DECIMAL(4,3) NOT NULL DEFAULT 0.300,
			complexity_threshold INT         NOT NULL DEFAULT 1200,
			created_at          DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at          DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`,

		`CREATE TABLE IF NOT EXISTS cost_groups (
			id          CHAR(36)     NOT NULL PRIMARY KEY,
			user_id     CHAR(36)     NOT NULL,
			name        VARCHAR(120) NOT NULL,
			description VARCHAR(500) NOT NULL DEFAULT '',
			created_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			UNIQUE KEY uq_cost_groups_user_name (user_id, name),
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`,

		`CREATE TABLE IF NOT EXISTS api_keys (
			id         CHAR(36)     NOT NULL PRIMARY KEY,
			user_id    CHAR(36)     NOT NULL,
			group_id   CHAR(36),
			name       VARCHAR(100) NOT NULL,
			key_hash   VARCHAR(255) NOT NULL UNIQUE,
			key_prefix VARCHAR(10)  NOT NULL,
			allowed_provider_ids TEXT NOT NULL,
			allowed_models TEXT NOT NULL,
			expires_at DATETIME,
			created_at DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY (group_id) REFERENCES cost_groups(id) ON DELETE SET NULL
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`,

		`CREATE TABLE IF NOT EXISTS requests (
			id              CHAR(36)     NOT NULL PRIMARY KEY,
			user_id         CHAR(36)     NOT NULL,
			api_key_id      CHAR(36),
			group_id        CHAR(36),
			provider_id     CHAR(36),
			model           VARCHAR(100) NOT NULL,
			prompt_tokens   INT          NOT NULL DEFAULT 0,
			completion_tokens INT        NOT NULL DEFAULT 0,
			total_tokens    INT          NOT NULL DEFAULT 0,
			latency_ms      INT          NOT NULL DEFAULT 0,
			status          SMALLINT     NOT NULL DEFAULT 200,
			created_at      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY (api_key_id) REFERENCES api_keys(id) ON DELETE SET NULL,
			FOREIGN KEY (group_id) REFERENCES cost_groups(id) ON DELETE SET NULL,
			FOREIGN KEY (provider_id) REFERENCES providers(id) ON DELETE SET NULL
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`,

		`CREATE TABLE IF NOT EXISTS llm_routes (
			id          CHAR(36)     NOT NULL PRIMARY KEY,
			user_id     CHAR(36)     NOT NULL,
			name        VARCHAR(100) NOT NULL,
			slug        VARCHAR(100) NOT NULL,
			description VARCHAR(500) NOT NULL DEFAULT '',
			provider_id CHAR(36)     NOT NULL,
			model       VARCHAR(100) NOT NULL,
			system_prompt TEXT,
			temperature DECIMAL(3,2) NOT NULL DEFAULT 1.00,
			max_tokens  INT          NOT NULL DEFAULT 0,
			stream_passthrough TINYINT(1) NOT NULL DEFAULT 1,
			prompt_version_id CHAR(36) NOT NULL DEFAULT '',
			enforce_json_schema TINYINT(1) NOT NULL DEFAULT 0,
			output_json_schema LONGTEXT,
			failover_provider_ids TEXT NOT NULL,
			allowed_models TEXT NOT NULL,
			enabled     TINYINT(1)   NOT NULL DEFAULT 1,
			created_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			UNIQUE KEY uq_route_slug_user (user_id, slug),
			FOREIGN KEY (user_id)     REFERENCES users(id)     ON DELETE CASCADE,
			FOREIGN KEY (provider_id) REFERENCES providers(id) ON DELETE CASCADE
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`,

		`CREATE TABLE IF NOT EXISTS prompt_templates (
			id               CHAR(36)     NOT NULL PRIMARY KEY,
			user_id          CHAR(36)     NOT NULL,
			name             VARCHAR(160) NOT NULL,
			description      VARCHAR(600) NOT NULL DEFAULT '',
			active_version_id CHAR(36)    NOT NULL DEFAULT '',
			created_at       DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at       DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			UNIQUE KEY uq_prompt_templates_user_name (user_id, name),
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`,

		`CREATE TABLE IF NOT EXISTS prompt_versions (
			id            CHAR(36)     NOT NULL PRIMARY KEY,
			template_id   CHAR(36)     NOT NULL,
			user_id       CHAR(36)     NOT NULL,
			version       INT          NOT NULL,
			content       LONGTEXT     NOT NULL,
			test_input    LONGTEXT,
			test_output   LONGTEXT,
			test_status   INT          NOT NULL DEFAULT 0,
			activated_at  DATETIME,
			created_at    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE KEY uq_prompt_versions_template_version (template_id, version),
			FOREIGN KEY (template_id) REFERENCES prompt_templates(id) ON DELETE CASCADE,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`,

		`ALTER TABLE llm_routes ADD COLUMN IF NOT EXISTS
			stream_passthrough TINYINT(1) NOT NULL DEFAULT 1;`,

		`ALTER TABLE llm_routes ADD COLUMN IF NOT EXISTS
			prompt_version_id CHAR(36) NOT NULL DEFAULT '';`,

		`ALTER TABLE llm_routes ADD COLUMN IF NOT EXISTS
			enforce_json_schema TINYINT(1) NOT NULL DEFAULT 0;`,

		`ALTER TABLE llm_routes ADD COLUMN IF NOT EXISTS
			output_json_schema LONGTEXT;`,

		`ALTER TABLE llm_routes ADD COLUMN IF NOT EXISTS
			failover_provider_ids TEXT NOT NULL;`,

		`ALTER TABLE llm_routes ADD COLUMN IF NOT EXISTS
			allowed_models TEXT NOT NULL;`,

		`ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS
			allowed_provider_ids TEXT NOT NULL;`,

		`ALTER TABLE users ADD COLUMN IF NOT EXISTS
			plan_id VARCHAR(32) NOT NULL DEFAULT 'basic';`,

		`UPDATE users SET plan_id = 'basic' WHERE TRIM(COALESCE(plan_id,'')) = '';`,

		`ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS
			group_id CHAR(36);`,

		`ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS
			allowed_models TEXT NOT NULL;`,

		`ALTER TABLE providers ADD COLUMN IF NOT EXISTS
			adapter VARCHAR(40) NOT NULL DEFAULT 'openai';`,

		`ALTER TABLE providers ADD COLUMN IF NOT EXISTS
			api_version VARCHAR(40) NOT NULL DEFAULT '';`,

		`ALTER TABLE providers ADD COLUMN IF NOT EXISTS
			api_keys_json LONGTEXT NOT NULL DEFAULT '[]';`,

		`ALTER TABLE cache_configs ADD COLUMN IF NOT EXISTS
			semantic_enabled TINYINT(1) NOT NULL DEFAULT 0;`,

		`ALTER TABLE cache_configs ADD COLUMN IF NOT EXISTS
			semantic_threshold DECIMAL(4,3) NOT NULL DEFAULT 0.900;`,

		`ALTER TABLE cache_configs ADD COLUMN IF NOT EXISTS
			semantic_max_candidates INT NOT NULL DEFAULT 30;`,

		`ALTER TABLE cache_configs ADD COLUMN IF NOT EXISTS
			semantic_embedding_model VARCHAR(120) NOT NULL DEFAULT 'text-embedding-3-small';`,

		// Add cost_usd to existing requests rows (idempotent on MariaDB 10.0+).
		`ALTER TABLE requests ADD COLUMN IF NOT EXISTS
			cost_usd DECIMAL(12,8) NOT NULL DEFAULT 0;`,

		`ALTER TABLE requests ADD COLUMN IF NOT EXISTS
			ttft_ms INT NOT NULL DEFAULT 0;`,

		`ALTER TABLE requests ADD COLUMN IF NOT EXISTS
			api_key_id CHAR(36);`,

		`ALTER TABLE requests ADD COLUMN IF NOT EXISTS
			group_id CHAR(36);`,

		`CREATE INDEX IF NOT EXISTS idx_api_keys_group_id ON api_keys(group_id);`,

		`CREATE INDEX IF NOT EXISTS idx_requests_group_created ON requests(group_id, created_at);`,

		`CREATE TABLE IF NOT EXISTS model_costs (
			id                 CHAR(36)      NOT NULL PRIMARY KEY,
			user_id            CHAR(36)      NOT NULL,
			provider_id        CHAR(36)      NOT NULL,
			model              VARCHAR(100)  NOT NULL,
			input_cost_per_1m  DECIMAL(12,6) NOT NULL DEFAULT 0,
			output_cost_per_1m DECIMAL(12,6) NOT NULL DEFAULT 0,
			currency           VARCHAR(10)   NOT NULL DEFAULT 'USD',
			notes              VARCHAR(500)  NOT NULL DEFAULT '',
			created_at         DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at         DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			UNIQUE KEY uq_cost_user_provider_model (user_id, provider_id, model),
			FOREIGN KEY (user_id)     REFERENCES users(id)     ON DELETE CASCADE,
			FOREIGN KEY (provider_id) REFERENCES providers(id) ON DELETE CASCADE
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`,

		`CREATE TABLE IF NOT EXISTS audit_logs (
			id          CHAR(36)     NOT NULL PRIMARY KEY,
			user_id     CHAR(36)     NOT NULL,
			api_key_id  CHAR(36),
			provider_id CHAR(36),
			request_id  VARCHAR(100) NOT NULL,
			endpoint    VARCHAR(100) NOT NULL,
			direction   VARCHAR(40)  NOT NULL,
			route_slug  VARCHAR(100) NOT NULL DEFAULT '',
			model       VARCHAR(100) NOT NULL DEFAULT '',
			http_status SMALLINT     NOT NULL DEFAULT 0,
			latency_ms  INT          NOT NULL DEFAULT 0,
			success     TINYINT(1)   NOT NULL DEFAULT 1,
			error       TEXT,
			payload     LONGTEXT,
			created_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_audit_user_created (user_id, created_at),
			INDEX idx_audit_request_id (request_id),
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY (api_key_id) REFERENCES api_keys(id) ON DELETE SET NULL,
			FOREIGN KEY (provider_id) REFERENCES providers(id) ON DELETE SET NULL
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`,

		// User Groups for organizing users and tracking token usage by team/department
		`CREATE TABLE IF NOT EXISTS user_groups (
			id          CHAR(36)     NOT NULL PRIMARY KEY,
			owner_id    CHAR(36)     NOT NULL,
			name        VARCHAR(120) NOT NULL,
			description VARCHAR(500) NOT NULL DEFAULT '',
			created_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			UNIQUE KEY uq_user_groups_owner_name (owner_id, name),
			FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE CASCADE
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`,

		// Group membership for adding users to user groups
		`CREATE TABLE IF NOT EXISTS user_group_members (
			id          CHAR(36)     NOT NULL PRIMARY KEY,
			group_id    CHAR(36)     NOT NULL,
			user_id     CHAR(36)     NOT NULL,
			role        ENUM('member','admin') NOT NULL DEFAULT 'member',
			created_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE KEY uq_user_group_members (group_id, user_id),
			FOREIGN KEY (group_id) REFERENCES user_groups(id) ON DELETE CASCADE,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`,

		// Add group_id to user_groups table for cost tracking (similar to cost_groups)
		`ALTER TABLE requests ADD COLUMN IF NOT EXISTS
			user_group_id CHAR(36);`,

		`CREATE INDEX IF NOT EXISTS idx_requests_user_group_created ON requests(user_group_id, created_at);`,

		`ALTER TABLE requests ADD CONSTRAINT IF NOT EXISTS
			fk_requests_user_group_id FOREIGN KEY (user_group_id) REFERENCES user_groups(id) ON DELETE SET NULL;`,

		// Policies table for request/model validation rules
		`CREATE TABLE IF NOT EXISTS policies (
			id          CHAR(36)     NOT NULL PRIMARY KEY,
			user_id     CHAR(36)     NOT NULL,
			name        VARCHAR(120) NOT NULL,
			description VARCHAR(500) NOT NULL DEFAULT '',
			scope       ENUM('global','local') NOT NULL DEFAULT 'global',
			model_name  VARCHAR(100),
			pattern     LONGTEXT     NOT NULL,
			target      VARCHAR(50)  NOT NULL,
			action      ENUM('allow','deny') NOT NULL DEFAULT 'deny',
			priority    INT          NOT NULL DEFAULT 1000,
			enabled     TINYINT(1)   NOT NULL DEFAULT 1,
			notes       VARCHAR(500) NOT NULL DEFAULT '',
			created_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			INDEX idx_policies_user_scope (user_id, scope),
			INDEX idx_policies_user_model (user_id, model_name),
			INDEX idx_policies_priority (priority),
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`,

		// Request metrics table for telemetry collection
		`CREATE TABLE IF NOT EXISTS request_metrics (
			id                  CHAR(36)      NOT NULL PRIMARY KEY,
			user_id             CHAR(36),
			api_key_id          CHAR(36),
			provider_id         CHAR(36),
			model_name          VARCHAR(100)  NOT NULL,
			provider_name       VARCHAR(100)  NOT NULL,
			endpoint            VARCHAR(100)  NOT NULL,
			latency_ms          INT           NOT NULL,
			total_tokens        INT           NOT NULL DEFAULT 0,
			cost_usd            DECIMAL(12,8) NOT NULL DEFAULT 0,
			status              SMALLINT      NOT NULL DEFAULT 200,
			error_message       VARCHAR(500),
			created_at          DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_request_metrics_user_created (user_id, created_at),
			INDEX idx_request_metrics_provider_created (provider_id, created_at),
			INDEX idx_request_metrics_created (created_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`,

		// Telemetry snapshots for historical data aggregation
		`CREATE TABLE IF NOT EXISTS telemetry_snapshots (
			id                    CHAR(36)      NOT NULL PRIMARY KEY,
			user_id               CHAR(36),
			total_requests        BIGINT        NOT NULL,
			successful_requests   BIGINT        NOT NULL,
			failed_requests       BIGINT        NOT NULL,
			avg_latency_ms        DECIMAL(10,2) NOT NULL,
			p50_latency_ms        DECIMAL(10,2) NOT NULL,
			p90_latency_ms        DECIMAL(10,2) NOT NULL,
			p99_latency_ms        DECIMAL(10,2) NOT NULL,
			max_latency_ms        INT           NOT NULL,
			total_tokens          BIGINT        NOT NULL,
			total_cost_usd        DECIMAL(12,8) NOT NULL,
			active_providers      INT           NOT NULL DEFAULT 0,
			requests_per_second   DECIMAL(10,2) NOT NULL,
			success_rate_pct      DECIMAL(5,2)  NOT NULL,
			time_window           VARCHAR(10)   NOT NULL DEFAULT '1m',
			snapshot_at           DATETIME      NOT NULL,
			created_at            DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_telemetry_snapshots_user_created (user_id, created_at),
			INDEX idx_telemetry_snapshots_snapshot_at (snapshot_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`,

		// Provider metrics snapshots for per-provider historical tracking
		`CREATE TABLE IF NOT EXISTS provider_metrics_snapshots (
			id                    CHAR(36)      NOT NULL PRIMARY KEY,
			user_id               CHAR(36),
			provider_id           CHAR(36),
			provider_name         VARCHAR(100)  NOT NULL,
			request_count         BIGINT        NOT NULL,
			success_count         BIGINT        NOT NULL,
			failure_count         BIGINT        NOT NULL,
			avg_latency_ms        DECIMAL(10,2) NOT NULL,
			p50_latency_ms        DECIMAL(10,2) NOT NULL,
			p90_latency_ms        DECIMAL(10,2) NOT NULL,
			p99_latency_ms        DECIMAL(10,2) NOT NULL,
			max_latency_ms        INT           NOT NULL,
			total_tokens          BIGINT        NOT NULL,
			total_cost_usd        DECIMAL(12,8) NOT NULL,
			snapshot_at           DATETIME      NOT NULL,
			created_at            DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_provider_metrics_user_created (user_id, created_at),
			INDEX idx_provider_metrics_provider_created (provider_id, created_at),
			INDEX idx_provider_metrics_snapshot_at (snapshot_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`,

		// Model metrics snapshots for per-model historical tracking
		`CREATE TABLE IF NOT EXISTS model_metrics_snapshots (
			id                    CHAR(36)      NOT NULL PRIMARY KEY,
			user_id               CHAR(36),
			provider_id           CHAR(36),
			model_name            VARCHAR(100)  NOT NULL,
			request_count         BIGINT        NOT NULL,
			success_count         BIGINT        NOT NULL,
			failure_count         BIGINT        NOT NULL,
			avg_latency_ms        DECIMAL(10,2) NOT NULL,
			p90_latency_ms        DECIMAL(10,2) NOT NULL,
			p99_latency_ms        DECIMAL(10,2) NOT NULL,
			max_latency_ms        INT           NOT NULL,
			total_tokens          BIGINT        NOT NULL,
			total_cost_usd        DECIMAL(12,8) NOT NULL,
			snapshot_at           DATETIME      NOT NULL,
			created_at            DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_model_metrics_user_created (user_id, created_at),
			INDEX idx_model_metrics_model_created (model_name, created_at),
			INDEX idx_model_metrics_snapshot_at (snapshot_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`,
	}

	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("migration error: %w", err)
		}
	}

	if err := ensureDefaultAdmin(db); err != nil {
		return fmt.Errorf("default admin seed error: %w", err)
	}
	return nil
}

func ensureDefaultAdmin(db *sql.DB) error {
	enabled := strings.TrimSpace(strings.ToLower(os.Getenv("DEFAULT_ADMIN_ENABLED")))
	if enabled == "" {
		enabled = "true"
	}
	if enabled != "1" && enabled != "true" && enabled != "yes" {
		return nil
	}

	adminID := strings.TrimSpace(os.Getenv("DEFAULT_ADMIN_ID"))
	if adminID == "" {
		adminID = "00000000-0000-0000-0000-000000000001"
	}
	adminEmail := strings.TrimSpace(os.Getenv("DEFAULT_ADMIN_EMAIL"))
	if adminEmail == "" {
		adminEmail = "admin@llm-gatway.local"
	}
	adminHash := strings.TrimSpace(os.Getenv("DEFAULT_ADMIN_PASSWORD_BCRYPT"))
	if adminHash == "" {
		// Default password is: admin123
		adminHash = "$2a$10$9n4l4PjeSi4OXMlcdrzmi.VfSv1ofqdVH9hN6/3rA3Pt0ECNDVJUe"
	}

	_, err := db.Exec(
		`INSERT INTO users (id, email, password, role, plan_id)
		 VALUES (?, ?, ?, 'admin', 'basic')
		 ON DUPLICATE KEY UPDATE role = 'admin'`,
		adminID, adminEmail, adminHash,
	)
	return err
}
