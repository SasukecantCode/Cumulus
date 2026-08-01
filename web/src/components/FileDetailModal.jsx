import React from 'react'
import { Modal } from './Modal'
import { Button } from './ui/Primitives'
import { KINDS, fileFields, googleNativeLabel, isCumulusUpload, kindOf } from '@/lib/files'
import { formatBytes, formatDate } from '@/lib/utils'
import { api } from '@/api/client'
import { CheckCircle2, Download, HardDrive } from 'lucide-react'

function Fact({ label, value }) {
  return (
    <div className="sunken px-3 py-2.5">
      <span className="label">{label}</span>
      <p className="text-sm font-mono text-ink truncate mt-1" title={String(value)}>
        {value}
      </p>
    </div>
  )
}

export function FileDetailModal({ file, isOpen, onClose, workspaceId, accountMap = {} }) {
  if (!isOpen || !file) return null

  const f = fileFields(file)
  const kind = kindOf(f.mime, f.name)
  const home = accountMap[f.accountId] || (f.accountId ? `Account #${f.accountId}` : 'Cloud drive')
  const viaCumulus = isCumulusUpload(file)

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      icon={KINDS[kind].icon}
      title={f.name}
      subtitle={googleNativeLabel(f.mime) || `${formatBytes(f.size)} · ${KINDS[kind].label}`}
      footer={
        <>
          <Button onClick={onClose}>Close</Button>
          <Button
            variant="primary"
            onClick={() => window.open(api.downloadFileUrl(workspaceId, f.id), '_blank')}
          >
            <Download className="w-4 h-4" />
            Download
          </Button>
        </>
      }
    >
      <div className="flex flex-col gap-3">
        <div className="grid grid-cols-2 gap-3">
          <Fact
            label="Size"
            value={googleNativeLabel(f.mime) ? 'No quota used' : formatBytes(f.size)}
          />
          <Fact label="Type" value={f.mime || 'application/octet-stream'} />
          <Fact label="File ID" value={`#${f.id}`} />
          <Fact label="Uploaded" value={f.createdAt ? formatDate(f.createdAt) : '—'} />
        </div>

        <div className="sunken p-4 flex flex-col gap-3">
          <div className="flex items-center justify-between gap-3">
            <span className="label">Stored on</span>
            <span className="inline-flex items-center gap-1.5 text-xs font-semibold text-ok">
              <CheckCircle2 className="w-3.5 h-3.5" />
              {viaCumulus ? 'Uploaded here' : 'Found in Drive'}
            </span>
          </div>

          <div className="flex items-center gap-3">
            <span className="w-9 h-9 grid place-items-center shrink-0 rounded-tile border border-line bg-card">
              <HardDrive className="w-4 h-4 text-brand" />
            </span>
            <div className="min-w-0">
              <p className="text-sm font-semibold text-ink truncate">{home}</p>
              <p className="text-[11px] text-muted">
                {viaCumulus
                  ? 'Chosen automatically when this file went up.'
                  : 'Already in this Drive when Cumulus connected to it.'}
              </p>
            </div>
          </div>
        </div>
      </div>
    </Modal>
  )
}
