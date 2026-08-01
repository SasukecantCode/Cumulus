import React, { useEffect, useRef, useState } from 'react'
import { Modal } from './Modal'
import { Button, Meter, Notice } from './ui/Primitives'
import { formatBytes, cn } from '@/lib/utils'
import { api } from '@/api/client'
import { CheckCircle2, CloudUpload, Loader2, UploadCloud, X } from 'lucide-react'

export function UploadModal({ isOpen, onClose, workspaceId, onUploadSuccess, initialFiles }) {
  const [queue, setQueue] = useState([])
  const [dragging, setDragging] = useState(false)
  const [uploading, setUploading] = useState(false)
  const [doneCount, setDoneCount] = useState(0)
  const [status, setStatus] = useState('')
  const [error, setError] = useState(null)
  const inputRef = useRef(null)

  // Files dropped onto the page arrive pre-loaded.
  useEffect(() => {
    if (isOpen && initialFiles?.length) {
      setQueue(initialFiles)
      setError(null)
    }
    if (!isOpen) {
      setQueue([])
      setDoneCount(0)
      setStatus('')
      setError(null)
      setUploading(false)
    }
  }, [isOpen, initialFiles])

  const addFiles = (list) => {
    const incoming = Array.from(list || [])
    if (incoming.length === 0) return
    setQueue((prev) => [...prev, ...incoming])
    setError(null)
  }

  const removeAt = (idx) => setQueue((prev) => prev.filter((_, i) => i !== idx))

  const totalBytes = queue.reduce((sum, f) => sum + f.size, 0)
  const progress = queue.length ? (doneCount / queue.length) * 100 : 0

  const start = async () => {
    if (queue.length === 0) return
    setUploading(true)
    setError(null)
    setDoneCount(0)

    try {
      for (let i = 0; i < queue.length; i++) {
        setStatus(`Finding room for ${queue[i].name}…`)
        await api.uploadFile(workspaceId, queue[i])
        setDoneCount(i + 1)
      }
      setStatus('Done — everything is up.')
      await onUploadSuccess()
      setTimeout(onClose, 600)
    } catch (err) {
      setError(err.message || 'Upload failed.')
      setUploading(false)
    }
  }

  const onDrop = (e) => {
    e.preventDefault()
    setDragging(false)
    addFiles(e.dataTransfer.files)
  }

  return (
    <Modal
      isOpen={isOpen}
      onClose={uploading ? () => {} : onClose}
      icon={UploadCloud}
      title="Upload files"
      subtitle="Cumulus picks the drive with room to spare"
      footer={
        <>
          <Button onClick={onClose} disabled={uploading}>
            Cancel
          </Button>
          <Button variant="primary" onClick={start} disabled={queue.length === 0 || uploading}>
            {uploading ? (
              <>
                <Loader2 className="w-4 h-4 animate-spin" />
                Uploading…
              </>
            ) : (
              <>
                <CloudUpload className="w-4 h-4" />
                Upload {queue.length > 0 && `${queue.length} file${queue.length === 1 ? '' : 's'}`}
              </>
            )}
          </Button>
        </>
      }
    >
      <div className="flex flex-col gap-4">
        {/* drop zone — the input lives outside the button, since a button
            may not contain another interactive element */}
        <input
          ref={inputRef}
          type="file"
          multiple
          hidden
          onChange={(e) => addFiles(e.target.files)}
          disabled={uploading}
        />
        <button
          type="button"
          onDragEnter={(e) => {
            e.preventDefault()
            setDragging(true)
          }}
          onDragOver={(e) => e.preventDefault()}
          onDragLeave={() => setDragging(false)}
          onDrop={onDrop}
          onClick={() => !uploading && inputRef.current?.click()}
          className={cn(
            'w-full px-6 py-9 flex flex-col items-center text-center gap-2 cursor-pointer',
            'rounded-control border border-dashed transition-colors duration-150',
            dragging
              ? 'border-brand bg-brand-soft'
              : 'border-line bg-sunken hover:border-brand-line hover:bg-brand-soft',
          )}
        >
          <UploadCloud className="w-7 h-7 text-brand" />
          <p className="text-sm font-semibold text-ink">
            {dragging ? 'Let go' : 'Drop files here, or click to browse'}
          </p>
          <p className="text-xs text-muted">Any size, up to the space left in your pool.</p>
        </button>

        {error && <Notice>{error}</Notice>}

        {/* queue */}
        {queue.length > 0 && (
          <div className="flex flex-col gap-2">
            <div className="flex items-center justify-between">
              <span className="label">Ready to upload ({queue.length})</span>
              <span className="text-[11px] font-mono text-muted tabular-nums">
                {formatBytes(totalBytes)}
              </span>
            </div>

            <div className="flex flex-col gap-1.5 max-h-52 overflow-y-auto">
              {queue.map((file, idx) => {
                const done = uploading && idx < doneCount
                const active = uploading && idx === doneCount
                return (
                  <div
                    key={`${file.name}-${idx}`}
                    className="sunken flex items-center gap-2.5 px-3 py-2"
                  >
                    {done ? (
                      <CheckCircle2 className="w-4 h-4 shrink-0 text-ok" />
                    ) : active ? (
                      <Loader2 className="w-4 h-4 shrink-0 animate-spin text-brand" />
                    ) : (
                      <CloudUpload className="w-4 h-4 shrink-0 text-muted" />
                    )}
                    <span className="flex-1 min-w-0 truncate text-sm text-ink">{file.name}</span>
                    <span className="text-[11px] font-mono text-muted shrink-0 tabular-nums">
                      {formatBytes(file.size)}
                    </span>
                    {!uploading && (
                      <button
                        type="button"
                        onClick={() => removeAt(idx)}
                        aria-label={`Remove ${file.name}`}
                        className="shrink-0 text-muted hover:text-danger cursor-pointer"
                      >
                        <X className="w-3.5 h-3.5" />
                      </button>
                    )}
                  </div>
                )
              })}
            </div>
          </div>
        )}

        {uploading && (
          <div className="flex flex-col gap-1.5">
            <div className="flex justify-between text-xs font-mono">
              <span className="text-brand-ink truncate">{status}</span>
              <span className="text-muted tabular-nums shrink-0">
                {doneCount}/{queue.length}
              </span>
            </div>
            <Meter value={progress} />
          </div>
        )}
      </div>
    </Modal>
  )
}
