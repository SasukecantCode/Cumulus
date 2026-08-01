import {
  Archive,
  Code2,
  FileText,
  Film,
  Image,
  LayoutGrid,
  Music,
  Video,
} from 'lucide-react'

/**
 * One table describes every file kind: its icon, its label and the sidebar
 * category it belongs to. Kinds carry no colour of their own — the icon is
 * the only signal — so icons and filters can never drift apart.
 */
export const KINDS = {
  image: { icon: Image, label: 'Image' },
  video: { icon: Film, label: 'Video' },
  audio: { icon: Music, label: 'Audio' },
  archive: { icon: Archive, label: 'Archive' },
  code: { icon: Code2, label: 'Code' },
  document: { icon: FileText, label: 'Document' },
  other: { icon: FileText, label: 'File' },
}

export const CATEGORIES = [
  // "All types" rather than "All files": this list narrows by kind, and the
  // account tree above it is what narrows by whose drive the file lives on.
  { id: 'all', label: 'All types', icon: LayoutGrid },
  { id: 'document', label: 'Documents', icon: FileText },
  { id: 'image', label: 'Photos', icon: Image },
  { id: 'video', label: 'Video', icon: Video },
  { id: 'audio', label: 'Audio', icon: Music },
  { id: 'code', label: 'Code', icon: Code2 },
  { id: 'archive', label: 'Archives', icon: Archive },
]

export function kindOf(mimeType, filename) {
  const mime = (mimeType || '').toLowerCase()
  const name = (filename || '').toLowerCase()

  if (mime.startsWith('image/') || /\.(jpe?g|png|gif|webp|svg|avif|heic)$/.test(name)) return 'image'
  if (mime.startsWith('video/') || /\.(mp4|mkv|webm|avi|mov)$/.test(name)) return 'video'
  if (mime.startsWith('audio/') || /\.(mp3|wav|ogg|flac|m4a)$/.test(name)) return 'audio'
  if (
    mime.includes('zip') ||
    mime.includes('tar') ||
    mime.includes('compressed') ||
    /\.(zip|rar|7z|gz|tgz|bz2)$/.test(name)
  ) {
    return 'archive'
  }
  if (
    mime.includes('json') ||
    mime.includes('javascript') ||
    mime.includes('html') ||
    mime.includes('xml') ||
    /\.(jsx?|tsx?|json|html?|css|go|py|rs|rb|java|sh|ya?ml|toml)$/.test(name)
  ) {
    return 'code'
  }
  if (
    mime.includes('pdf') ||
    mime.includes('document') ||
    mime.includes('text') ||
    mime.includes('sheet') ||
    mime.includes('presentation') ||
    /\.(pdf|docx?|txt|md|csv|xlsx?|pptx?)$/.test(name)
  ) {
    return 'document'
  }
  return 'other'
}

/** The API returns Go-cased fields; older payloads use camelCase. Take either. */
export function fileFields(file) {
  return {
    id: file.ID || file.id,
    name: file.Filename || file.filename || file.Name || 'Untitled',
    size: file.SizeBytes || file.sizeBytes || file.Size || 0,
    mime: file.MimeType || file.mimeType || '',
    accountId: file.CloudAccountID || file.cloudAccountId,
    createdAt: file.CreatedAt || file.createdAt,
    // 'CUMULUS' for uploads made here, 'DRIVE' for files adopted from the
    // account by the sync. Rows written before origin existed report neither,
    // and those are all Cumulus uploads.
    origin: file.Origin || file.origin || 'CUMULUS',
  }
}

/** Was this file put here through Cumulus, rather than found in the Drive? */
export function isCumulusUpload(file) {
  return fileFields(file).origin !== 'DRIVE'
}

/**
 * Google Docs, Sheets and Slides are stored by Google rather than as bytes in
 * the drive, so they genuinely weigh nothing against the quota. Reporting
 * "0 B" for them reads as a bug, so name the format instead.
 */
const GOOGLE_NATIVE = {
  'application/vnd.google-apps.document': 'Google Doc',
  'application/vnd.google-apps.spreadsheet': 'Google Sheet',
  'application/vnd.google-apps.presentation': 'Google Slides',
  'application/vnd.google-apps.drawing': 'Google Drawing',
  'application/vnd.google-apps.form': 'Google Form',
}

export function googleNativeLabel(mimeType) {
  if (!mimeType) return null
  if (GOOGLE_NATIVE[mimeType]) return GOOGLE_NATIVE[mimeType]
  return mimeType.startsWith('application/vnd.google-apps.') ? 'Google file' : null
}
