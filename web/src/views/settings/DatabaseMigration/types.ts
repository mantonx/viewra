import type {
  GithubComMantonxViewraInternalApplicationSystemMigrationPostgresConfig as PostgresConfig,
  GithubComMantonxViewraInternalApplicationSystemMigrationSQLiteConfig as SQLiteConfig,
} from '@/lib/api/generated/models'

export type DatabaseDriver = 'sqlite' | 'postgres'

export type WizardStep = 'choose-target' | 'configure' | 'review' | 'progress'

export type WizardState = {
  step: WizardStep
  targetDriver: DatabaseDriver | null
  postgresConfig: PostgresConfig
  sqliteConfig: SQLiteConfig
  connectionTested: boolean
  migrationStarted: boolean
  migrationId: string | null
}

export const initialWizardState: WizardState = {
  step: 'choose-target',
  targetDriver: null,
  postgresConfig: {
    host: 'localhost',
    port: 5432,
    user: 'viewra',
    password: '',
    database: 'viewra',
    sslMode: 'disable',
  },
  sqliteConfig: {
    path: 'data/viewra-new.db',
  },
  connectionTested: false,
  migrationStarted: false,
  migrationId: null,
}
