import { useEffect } from 'react'
import { useForm } from '@tanstack/react-form'
import { Modal, ModalContent } from '@/components/ui/Modal'
import {
  FormNumberInput,
  FormSettingToggle,
  FormModalFooter,
  FormApiError,
  FormSectionTitle,
} from '@/components/ui/Form'
import { usePutApiLibrariesId } from '@/lib/api'
import { useInvalidateLibraries } from '@/lib/hooks/useInvalidateLibraries'
import { useToast } from '@/lib/hooks/useToast'
import { useFormReset } from '@/lib/hooks/useFormReset'
import type { LibrarySettingsModalProps } from './LibrarySettingsModal.types'
import { librarySettingsSchema, type LibrarySettingsForm } from './LibrarySettingsModal.schema'

const DEFAULT_POLLING_INTERVAL = 60
const DEFAULT_DEBOUNCE_SECONDS = 5

const getDefaultValues = (
  library: LibrarySettingsModalProps['library']
): LibrarySettingsForm => ({
  monitoring_enabled: library.monitoring_enabled ?? true,
  polling_interval_minutes:
    library.monitoring_config?.polling_interval_minutes ?? DEFAULT_POLLING_INTERVAL,
  debounce_seconds: library.monitoring_config?.debounce_seconds ?? DEFAULT_DEBOUNCE_SECONDS,
})

const LibrarySettingsModal = ({ library, isOpen, onClose, onSave }: LibrarySettingsModalProps) => {
  const updateMutation = usePutApiLibrariesId()
  const invalidateLibraries = useInvalidateLibraries()
  const toast = useToast()

  const { apiError, handleSubmitError, resetKey } = useFormReset({
    isOpen,
    deps: [library],
  })

  const form = useForm({
    defaultValues: getDefaultValues(library),
    validators: {
      onChange: librarySettingsSchema,
    },
    onSubmit: async ({ value }) => {
      if (!library.id) {
        handleSubmitError(new Error('Library ID is missing'))
        return
      }

      try {
        await updateMutation.mutateAsync({
          id: library.id,
          data: {
            id: library.id,
            monitoring_enabled: value.monitoring_enabled,
            monitoring_config: {
              polling_interval_minutes: value.polling_interval_minutes,
              debounce_seconds: value.debounce_seconds,
            },
          },
        })
        invalidateLibraries()
        toast.success('Library settings saved')
        onSave()
        onClose()
      } catch (error) {
        handleSubmitError(error)
      }
    },
  })

  // Reset form when resetKey changes (modal opens or library changes)
  useEffect(() => {
    if (isOpen) {
      form.reset(getDefaultValues(library))
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [resetKey])

  return (
    <Modal isOpen={isOpen} onClose={onClose} title={`${library.name} Settings`} size="sm">
      <form
        onSubmit={(e) => {
          e.preventDefault()
          e.stopPropagation()
          form.handleSubmit()
        }}
      >
        <ModalContent>
          <div className="space-y-6">
            <FormApiError error={apiError} />

            {/* Monitoring Toggle */}
            <form.Field name="monitoring_enabled">
              {(field) => (
                <FormSettingToggle
                  field={field}
                  label="Filesystem Monitoring"
                  description="Automatically detect new and changed files"
                />
              )}
            </form.Field>

            {/* Advanced Settings - only show when monitoring is enabled */}
            <form.Subscribe selector={(state) => state.values.monitoring_enabled}>
              {(monitoringEnabled) =>
                monitoringEnabled ? (
                  <div className="space-y-4">
                    <FormSectionTitle>Advanced Settings</FormSectionTitle>

                    <form.Field name="polling_interval_minutes">
                      {(field) => (
                        <FormNumberInput
                          field={field}
                          label="Polling Interval (minutes)"
                          min={1}
                          max={1440}
                          helperText="How often to check for changes on network drives (NFS/SMB). Local drives use instant notifications."
                        />
                      )}
                    </form.Field>

                    <form.Field name="debounce_seconds">
                      {(field) => (
                        <FormNumberInput
                          field={field}
                          label="Debounce Window (seconds)"
                          min={1}
                          max={300}
                          helperText="Time to wait and batch multiple file changes together. Helps when copying many files at once."
                        />
                      )}
                    </form.Field>
                  </div>
                ) : null
              }
            </form.Subscribe>
          </div>
        </ModalContent>

        <FormModalFooter form={form} onCancel={onClose} />
      </form>
    </Modal>
  )
}

export { LibrarySettingsModal }
export type { LibrarySettingsModalProps } from './LibrarySettingsModal.types'
