-- LLM Gateway – MariaDB seed / init script
-- Runs once when the Docker volume is first created.

CREATE DATABASE IF NOT EXISTS llm_gatway
  CHARACTER SET utf8mb4
  COLLATE utf8mb4_unicode_ci;

USE llm_gatway;

-- Tables are created automatically by the Go migration on first start.
-- This file is a placeholder for any seed data you want to pre-load.
