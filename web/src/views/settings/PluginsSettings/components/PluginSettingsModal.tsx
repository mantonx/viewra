import { Modal, ModalContent, ModalFooter, Button } from '@/components/ui'
import { PluginSettingsForm } from '@/components/settings/forms/PluginSettingsForm'
import type { GithubComMantonxViewraInternalApplicationPluginsPluginSummary as PluginSummary } from '@/lib/api/generated/models'

interface PluginSettingsModalProps {
  isOpen: boolean
  onClose: () => void
  plugin: PluginSummary | null
}

export const PluginSettingsModal = ({ isOpen, onClose, plugin }: PluginSettingsModalProps) => {
  if (!plugin) {
    return null
  }

  const displayName = String(plugin.meta?.displayName ?? plugin.name ?? plugin.id ?? 'Plugin')

  return (
    <Modal isOpen={isOpen} onClose={onClose} title={`Configure: ${displayName}`} size="lg">
      <ModalContent>
        <PluginSettingsForm pluginId={plugin.id ?? ''} />
      </ModalContent>
      <ModalFooter>
        <Button variant="secondary" onClick={onClose}>
          Close
        </Button>
      </ModalFooter>
    </Modal>
  )
}
