import React, { useState } from 'react'
import { Modal } from './Modal'
import { Button, Card, Chip, IconButton, Meter, Notice } from './ui/Primitives'
import { CloudMascot } from './CumulusLogo'
import { formatBytes } from '@/lib/utils'
import { api } from '@/api/client'
import { Loader2, LogOut, Mail, Plus, Users, Trash2 } from 'lucide-react'

/**
 * Account management. The dashboard shows accounts read-only; this is where
 * they are added and detached.
 */
export function AccountsDrawer({ isOpen, onClose, nodes = [], workspaceId, onRefresh, onSignOut }) {
  const [disconnectingId, setDisconnectingId] = useState(null)
  const [error, setError] = useState(null)

  const pooled = nodes.reduce((sum, n) => sum + (n.Quota?.TotalBytes || 0), 0)

  const handleDisconnect = async (cloudAccountId, email) => {
    if (!confirm(`Disconnect ${email}? Files stored on it stop being reachable from Cumulus.`)) return
    setDisconnectingId(cloudAccountId)
    setError(null)
    try {
      await api.disconnectAccount(workspaceId, cloudAccountId)
      await onRefresh()
    } catch (err) {
      setError(err.message || 'Could not disconnect that account.')
    } finally {
      setDisconnectingId(null)
    }
  }

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      variant="drawer"
      icon={Users}
      title="Connected accounts"
      subtitle={`${nodes.length} account${nodes.length === 1 ? '' : 's'} · ${formatBytes(pooled)} pooled`}
      footer={
        <>
          {/* the rail is desktop-only, so this is the way out on mobile */}
          <Button variant="danger" className="lg:hidden mr-auto" onClick={onSignOut}>
            <LogOut className="w-4 h-4" />
            Sign out
          </Button>
          <Button onClick={onClose}>Done</Button>
          <Button
            variant="primary"
            onClick={() => {
              window.location.href = api.connectSecondaryUrl(workspaceId)
            }}
          >
            <Plus className="w-4 h-4" />
            Connect another
          </Button>
        </>
      }
    >
      <div className="flex flex-col gap-3">
        {error && <Notice onDismiss={() => setError(null)}>{error}</Notice>}

        {nodes.length === 0 ? (
          <div className="py-14 flex flex-col items-center text-center gap-3">
            <CloudMascot size={88} />
            <p className="text-sm text-muted max-w-xs leading-relaxed">
              No Google accounts here yet. Connect one and its free space joins the pool.
            </p>
          </div>
        ) : (
          nodes.map((node) => {
            const account = node.Account || {}
            const quota = node.Quota || {}
            const used = quota.UsedBytes || 0
            const total = quota.TotalBytes || 0
            const pct = total > 0 ? (used / total) * 100 : 0
            const busy = disconnectingId === account.ID

            return (
              <Card key={node.ID || account.ID} className="p-4 flex flex-col gap-3.5">
                <div className="flex items-start gap-3">
                  <span className="sunken w-9 h-9 grid place-items-center shrink-0">
                    <Mail className="w-4 h-4 text-brand" />
                  </span>

                  <div className="min-w-0 flex-1">
                    <p className="text-sm font-semibold text-ink truncate" title={account.Email}>
                      {account.Email || 'Google Drive account'}
                    </p>
                    <p className="text-[11px] font-mono text-muted mt-0.5">
                      Node {node.ID || '—'} · Google OAuth
                    </p>
                  </div>

                  {node.Role === 'owner' ? (
                    <Chip className="shrink-0">Primary</Chip>
                  ) : (
                    account.ID && (
                      <IconButton
                        label="Disconnect account"
                        size="sm"
                        variant="danger"
                        onClick={() => handleDisconnect(account.ID, account.Email)}
                        disabled={busy}
                      >
                        {busy ? (
                          <Loader2 className="w-4 h-4 animate-spin" />
                        ) : (
                          <Trash2 className="w-4 h-4" />
                        )}
                      </IconButton>
                    )
                  )}
                </div>

                <div className="flex flex-col gap-1.5">
                  <Meter value={pct} />
                  <div className="flex justify-between text-[11px] font-mono text-muted tabular-nums">
                    <span>{formatBytes(used)} used</span>
                    <span>{formatBytes(Math.max(0, total - used))} free</span>
                  </div>
                </div>
              </Card>
            )
          })
        )}
      </div>
    </Modal>
  )
}
