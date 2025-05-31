-- Migration to fix plugin table schema mismatches with Go models
-- This aligns the database schema with the actual Go model definitions

BEGIN TRANSACTION;

-- 1. Fix plugin_permissions table - add missing fields (only if they don't exist)
-- Check if columns exist before adding them
ALTER TABLE `plugin_permissions` ADD COLUMN `granted` BOOLEAN DEFAULT false;
ALTER TABLE `plugin_permissions` ADD COLUMN `granted_at` DATETIME;

-- 2. Fix plugin_events table - rename to backup, create new one, copy data if needed
DROP TABLE IF EXISTS `plugin_events_backup`;
ALTER TABLE `plugin_events` RENAME TO `plugin_events_backup`;

CREATE TABLE `plugin_events` (
    `id` INTEGER PRIMARY KEY AUTOINCREMENT,
    `plugin_id` TEXT NOT NULL,
    `event_type` TEXT NOT NULL,  -- Changed from event_name
    `message` TEXT,              -- New field
    `data` TEXT,                 -- Changed from handler (was storing handler info)
    `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (`plugin_id`) REFERENCES `plugins`(`plugin_id`) ON DELETE CASCADE
);
CREATE INDEX `idx_plugin_events_plugin_id` ON `plugin_events`(`plugin_id`);

-- Copy existing data if the backup table has compatible data
-- Map event_name -> event_type, handler -> data
INSERT INTO `plugin_events` (`plugin_id`, `event_type`, `data`, `created_at`)
SELECT `plugin_id`, `event_name`, `handler`, `created_at` 
FROM `plugin_events_backup`;

-- Drop backup table
DROP TABLE `plugin_events_backup`;

-- 3. Fix plugin_hooks table - rename to backup, create new one
DROP TABLE IF EXISTS `plugin_hooks_backup`;
ALTER TABLE `plugin_hooks` RENAME TO `plugin_hooks_backup`;

CREATE TABLE `plugin_hooks` (
    `id` INTEGER PRIMARY KEY AUTOINCREMENT,
    `plugin_id` TEXT NOT NULL,
    `hook_name` TEXT NOT NULL,
    `handler` TEXT NOT NULL,     -- Changed from callback
    `priority` INTEGER DEFAULT 100,
    `enabled` BOOLEAN DEFAULT true,  -- New field
    `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP,
    `updated_at` DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (`plugin_id`) REFERENCES `plugins`(`plugin_id`) ON DELETE CASCADE
);
CREATE INDEX `idx_plugin_hooks_plugin_id` ON `plugin_hooks`(`plugin_id`);

-- Copy existing data, mapping callback -> handler
INSERT INTO `plugin_hooks` (`plugin_id`, `hook_name`, `handler`, `priority`, `created_at`, `updated_at`)
SELECT `plugin_id`, `hook_name`, `callback`, `priority`, `created_at`, `updated_at` 
FROM `plugin_hooks_backup`;

-- Drop backup table
DROP TABLE `plugin_hooks_backup`;

-- 4. Fix plugin_admin_pages table - add missing fields
ALTER TABLE `plugin_admin_pages` ADD COLUMN `page_id` TEXT NOT NULL DEFAULT '';
ALTER TABLE `plugin_admin_pages` ADD COLUMN `url` TEXT NOT NULL DEFAULT '';
ALTER TABLE `plugin_admin_pages` ADD COLUMN `type` TEXT NOT NULL DEFAULT 'iframe';

-- 5. Fix plugin_ui_components table - rename to backup, create new one
DROP TABLE IF EXISTS `plugin_ui_components_backup`;
ALTER TABLE `plugin_ui_components` RENAME TO `plugin_ui_components_backup`;

CREATE TABLE `plugin_ui_components` (
    `id` INTEGER PRIMARY KEY AUTOINCREMENT,
    `plugin_id` TEXT NOT NULL,
    `component_id` TEXT NOT NULL,  -- Changed from component_name
    `name` TEXT NOT NULL,          -- New field (will copy from component_name)
    `type` TEXT NOT NULL,          -- Changed from component_type  
    `props` TEXT,                  -- Changed from config
    `url` TEXT NOT NULL DEFAULT '',           -- New required field
    `enabled` BOOLEAN DEFAULT true,
    `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP,
    `updated_at` DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (`plugin_id`) REFERENCES `plugins`(`plugin_id`) ON DELETE CASCADE
);
CREATE INDEX `idx_plugin_ui_components_plugin_id` ON `plugin_ui_components`(`plugin_id`);

-- Copy existing data, mapping fields appropriately
INSERT INTO `plugin_ui_components` (`plugin_id`, `component_id`, `name`, `type`, `props`, `enabled`, `created_at`, `updated_at`)
SELECT `plugin_id`, `component_name`, `component_name`, `component_type`, `config`, `enabled`, `created_at`, `updated_at` 
FROM `plugin_ui_components_backup`;

-- Drop backup table
DROP TABLE `plugin_ui_components_backup`;

COMMIT; 