import { useState, useCallback } from 'react'
import { Modal, ModalContent } from '@/components/ui'
import { usePostApiAdminSystemDatabaseMigrate } from '@/lib/api/generated/system/system'
import { StepChooseTarget } from './components/StepChooseTarget'
import { StepConfigure } from './components/StepConfigure'
import { StepReview } from './components/StepReview'
import { StepProgress } from './components/StepProgress'
import { initialWizardState, type WizardState, type DatabaseDriver } from './types'

type Props = {
  isOpen: boolean
  onClose: () => void
  currentDriver: DatabaseDriver
}

const stepTitles: Record<WizardState['step'], string> = {
  'choose-target': 'Choose Target Database',
  configure: 'Configure Connection',
  review: 'Review & Confirm',
  progress: 'Migration Progress',
}

const stepNumbers: Record<WizardState['step'], number> = {
  'choose-target': 1,
  configure: 2,
  review: 3,
  progress: 4,
}

export const DatabaseMigrationWizard = ({ isOpen, onClose, currentDriver }: Props) => {
  const [state, setState] = useState<WizardState>(initialWizardState)
  const startMigration = usePostApiAdminSystemDatabaseMigrate()

  const handleClose = useCallback(() => {
    if (state.step === 'progress' && state.migrationStarted) {
      // Don't allow closing during migration
      return
    }
    setState(initialWizardState)
    onClose()
  }, [state.step, state.migrationStarted, onClose])

  const handleSelectDriver = (driver: DatabaseDriver) => {
    setState((s) => ({ ...s, targetDriver: driver, connectionTested: false }))
  }

  const handleNextFromChoose = () => {
    setState((s) => ({ ...s, step: 'configure' }))
  }

  const handleNextFromConfigure = () => {
    setState((s) => ({ ...s, step: 'review' }))
  }

  const handleBack = () => {
    setState((s) => {
      switch (s.step) {
        case 'configure':
          return { ...s, step: 'choose-target' }
        case 'review':
          return { ...s, step: 'configure' }
        default:
          return s
      }
    })
  }

  const handleStartMigration = async () => {
    if (!state.targetDriver) {return}

    const request = {
      targetDriver: state.targetDriver,
      ...(state.targetDriver === 'postgres'
        ? { postgres: state.postgresConfig }
        : { sqlite: state.sqliteConfig }),
    }

    try {
      const result = await startMigration.mutateAsync({ data: request })
      if (result.status === 200 && result.data.started) {
        setState((s) => ({
          ...s,
          step: 'progress',
          migrationStarted: true,
          migrationId: result.data.migrationId ?? null,
        }))
      }
    } catch (_err) {
      // Error handled by mutation
    }
  }

  const handleMigrationComplete = () => {
    setState(initialWizardState)
    onClose()
  }

  const renderStep = () => {
    switch (state.step) {
      case 'choose-target':
        return (
          <StepChooseTarget
            currentDriver={currentDriver}
            selectedDriver={state.targetDriver}
            onSelect={handleSelectDriver}
            onNext={handleNextFromChoose}
            onCancel={handleClose}
          />
        )
      case 'configure':
        return (
          <StepConfigure
            targetDriver={state.targetDriver ?? 'sqlite'}
            postgresConfig={state.postgresConfig}
            sqliteConfig={state.sqliteConfig}
            onPostgresConfigChange={(config) => setState((s) => ({ ...s, postgresConfig: config }))}
            onSqliteConfigChange={(config) => setState((s) => ({ ...s, sqliteConfig: config }))}
            onConnectionTested={(success) => setState((s) => ({ ...s, connectionTested: success }))}
            onBack={handleBack}
            onNext={handleNextFromConfigure}
          />
        )
      case 'review':
        return (
          <StepReview
            sourceDriver={currentDriver}
            targetDriver={state.targetDriver ?? 'sqlite'}
            postgresConfig={state.postgresConfig}
            sqliteConfig={state.sqliteConfig}
            onBack={handleBack}
            onStart={handleStartMigration}
            isStarting={startMigration.isPending}
          />
        )
      case 'progress':
        return (
          <StepProgress
            migrationId={state.migrationId}
            onComplete={handleMigrationComplete}
            onFailed={handleMigrationComplete}
            onForceClose={() => {
              setState(initialWizardState)
              onClose()
            }}
          />
        )
    }
  }

  return (
    <Modal
      isOpen={isOpen}
      onClose={handleClose}
      title={`${stepTitles[state.step]} (${stepNumbers[state.step]}/4)`}
      size="lg"
    >
      <ModalContent>{renderStep()}</ModalContent>
    </Modal>
  )
}
