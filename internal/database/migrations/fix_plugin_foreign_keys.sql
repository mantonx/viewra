-- Migration to fix plugin table foreign key constraints
-- This fixes the backwards foreign key relationships and schema issues

BEGIN TRANSACTION;

-- 1. Drop the broken plugins table with wrong foreign keys
DROP TABLE IF EXISTS `plugins`;

-- 2. Recreate plugins table with correct schema
CREATE TABLE `plugins` (
    `id` INTEGER PRIMARY KEY AUTOINCREMENT,
    `plugin_id` TEXT NOT NULL UNIQUE,
    `name` TEXT NOT NULL,
    `version` TEXT NOT NULL,
    `description` TEXT,
    `author` TEXT,
    `website` TEXT,
    `repository` TEXT,
    `license` TEXT,
    `type` TEXT NOT NULL,
    `status` TEXT NOT NULL DEFAULT 'disabled',
    `install_path` TEXT NOT NULL,
    `manifest_data` TEXT,
    `config_data` TEXT,
    `error_message` TEXT,
    `installed_at` DATETIME,
    `enabled_at` DATETIME,
    `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP,
    `updated_at` DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 3. Create index on plugin_id for performance
CREATE UNIQUE INDEX `idx_plugins_plugin_id` ON `plugins`(`plugin_id`);

-- 4. Fix plugin child tables to have correct foreign keys TO plugins table

-- Drop and recreate plugin_events with correct FK
DROP TABLE IF EXISTS `plugin_events`;
CREATE TABLE `plugin_events` (
    `id` INTEGER PRIMARY KEY AUTOINCREMENT,
    `plugin_id` TEXT NOT NULL,
    `event_name` TEXT NOT NULL,
    `handler` TEXT NOT NULL,
    `priority` INTEGER DEFAULT 0,
    `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP,
    `updated_at` DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (`plugin_id`) REFERENCES `plugins`(`plugin_id`) ON DELETE CASCADE
);
CREATE INDEX `idx_plugin_events_plugin_id` ON `plugin_events`(`plugin_id`);

-- Drop and recreate plugin_hooks with correct FK
DROP TABLE IF EXISTS `plugin_hooks`;
CREATE TABLE `plugin_hooks` (
    `id` INTEGER PRIMARY KEY AUTOINCREMENT,
    `plugin_id` TEXT NOT NULL,
    `hook_name` TEXT NOT NULL,
    `callback` TEXT NOT NULL,
    `priority` INTEGER DEFAULT 0,
    `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP,
    `updated_at` DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (`plugin_id`) REFERENCES `plugins`(`plugin_id`) ON DELETE CASCADE
);
CREATE INDEX `idx_plugin_hooks_plugin_id` ON `plugin_hooks`(`plugin_id`);

-- Drop and recreate plugin_admin_pages with correct FK
DROP TABLE IF EXISTS `plugin_admin_pages`;
CREATE TABLE `plugin_admin_pages` (
    `id` INTEGER PRIMARY KEY AUTOINCREMENT,
    `plugin_id` TEXT NOT NULL,
    `title` TEXT NOT NULL,
    `path` TEXT NOT NULL,
    `icon` TEXT,
    `category` TEXT,
    `sort_order` INTEGER DEFAULT 0,
    `enabled` BOOLEAN DEFAULT true,
    `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP,
    `updated_at` DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (`plugin_id`) REFERENCES `plugins`(`plugin_id`) ON DELETE CASCADE
);
CREATE INDEX `idx_plugin_admin_pages_plugin_id` ON `plugin_admin_pages`(`plugin_id`);

-- Drop and recreate plugin_ui_components with correct FK
DROP TABLE IF EXISTS `plugin_ui_components`;
CREATE TABLE `plugin_ui_components` (
    `id` INTEGER PRIMARY KEY AUTOINCREMENT,
    `plugin_id` TEXT NOT NULL,
    `component_name` TEXT NOT NULL,
    `component_type` TEXT NOT NULL,
    `config` TEXT,
    `enabled` BOOLEAN DEFAULT true,
    `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP,
    `updated_at` DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (`plugin_id`) REFERENCES `plugins`(`plugin_id`) ON DELETE CASCADE
);
CREATE INDEX `idx_plugin_ui_components_plugin_id` ON `plugin_ui_components`(`plugin_id`);

-- Drop and recreate plugin_permissions with correct FK
DROP TABLE IF EXISTS `plugin_permissions`;
CREATE TABLE `plugin_permissions` (
    `id` INTEGER PRIMARY KEY AUTOINCREMENT,
    `plugin_id` TEXT NOT NULL,
    `permission` TEXT NOT NULL,
    `description` TEXT,
    `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP,
    `updated_at` DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (`plugin_id`) REFERENCES `plugins`(`plugin_id`) ON DELETE CASCADE
);
CREATE INDEX `idx_plugin_permissions_plugin_id` ON `plugin_permissions`(`plugin_id`);

-- 5. Drop the old music_metadata table if it still exists
DROP TABLE IF EXISTS `music_metadata`;

COMMIT; 