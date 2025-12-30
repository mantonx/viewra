-- Rollback people, credits, and studios tables

DROP TABLE IF EXISTS media_studios;
DROP TABLE IF EXISTS studios;
DROP TABLE IF EXISTS credits;
DROP TABLE IF EXISTS people;

DROP TYPE IF EXISTS studio_media_type;
DROP TYPE IF EXISTS credit_type;
DROP TYPE IF EXISTS credit_media_type;
