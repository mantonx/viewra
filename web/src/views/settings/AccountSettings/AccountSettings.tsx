import { SettingsPage } from '@/components/common'
import {
  AccountInfo,
  ActiveSessions,
  ChangePassword,
  LocationSettings,
} from '@/components/settings'

export const AccountSettings = () => {
  return (
    <SettingsPage>
      <SettingsPage.Header
        title="Account Settings"
        description="Manage your account security and preferences"
      />

      <div className="space-y-6">
        <AccountInfo />
        <LocationSettings />
        <ChangePassword />
        <ActiveSessions />
      </div>
    </SettingsPage>
  )
}
