import React from 'react'
import { CumulusLogo } from './CumulusLogo'
import { Button, Notice } from './ui/Primitives'
import SplitText from './ui/SplitText'
import { api } from '@/api/client'
import { ArrowRight, Layers, ShieldCheck, Sparkles } from 'lucide-react'

const PITCH = [
  {
    icon: Layers,
    title: 'One drive, many accounts',
    body: 'Every Google account you connect adds its free space to the same pool.',
  },
  {
    icon: Sparkles,
    title: 'Files land where they fit',
    body: 'Uploads are routed to whichever account has room. You never pick.',
  },
  {
    icon: ShieldCheck,
    title: 'Your accounts stay yours',
    body: 'Cumulus holds OAuth tokens only, and you can detach any account instantly.',
  },
]

export function LandingPage({ workspaceId, errorMsg }) {
  const handleLogin = () => {
    window.location.href = api.googleLoginUrl(workspaceId || 1)
  }

  return (
    <div className="min-h-screen flex flex-col">
      <header className="shell flex items-center justify-between py-5">
        <CumulusLogo size="md" />
        <Button onClick={handleLogin}>Sign in</Button>
      </header>

      <main className="shell flex-1 flex flex-col items-center justify-center text-center py-16">
        <div className="w-full max-w-2xl flex flex-col items-center gap-6">
          <CumulusLogo size="lg" layout="vertical" withWordmark={false} />

          {errorMsg && (
            <Notice className="w-full text-left">
              {errorMsg === 'already_attached'
                ? 'That account is already part of this workspace.'
                : errorMsg}
            </Notice>
          )}

          {/* The headline is the one animated thing in the app. Each line is
              its own split so the second can carry the brand colour; the
              inline-block boxes get a little bottom padding because the split
              parent clips overflow and would otherwise cut descenders. */}
          <h1 className="font-display text-4xl sm:text-5xl font-bold text-ink leading-[1.12] tracking-tight">
            <span className="block -mb-2">
              <SplitText
                tag="span"
                text="All your Google Drives,"
                className="pb-2"
                delay={28}
                duration={0.8}
                ease="power3.out"
                splitType="chars"
                from={{ opacity: 0, y: 40 }}
                to={{ opacity: 1, y: 0 }}
                threshold={0.1}
                rootMargin="0px"
              />
            </span>
            <span className="block text-brand -mb-2">
              <SplitText
                tag="span"
                text="one sky."
                className="pb-2"
                delay={28}
                duration={0.8}
                ease="power3.out"
                splitType="chars"
                from={{ opacity: 0, y: 40 }}
                to={{ opacity: 1, y: 0 }}
                threshold={0.1}
                rootMargin="0px"
              />
            </span>
          </h1>

          <SplitText
            tag="p"
            text="Cumulus pools the free space across every Google account you own into a single drive — one file list, one search, one storage number."
            className="text-muted text-base sm:text-lg leading-relaxed max-w-xl"
            delay={18}
            duration={0.7}
            ease="power3.out"
            splitType="words"
            from={{ opacity: 0, y: 14 }}
            to={{ opacity: 1, y: 0 }}
            threshold={0.1}
            rootMargin="0px"
          />

          <div className="flex flex-col items-center gap-3">
            <Button variant="primary" size="lg" onClick={handleLogin} className="group">
              <GoogleGlyph />
              <span>Continue with Google</span>
              <ArrowRight className="w-4 h-4 transition-transform duration-150 group-hover:translate-x-0.5" />
            </Button>
            <p className="text-muted text-xs">Read and write access to your Drive. Nothing else.</p>
          </div>
        </div>
      </main>

      <section className="shell pb-16">
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 max-w-5xl mx-auto">
          {PITCH.map(({ icon: Icon, title, body }) => (
            <div key={title} className="card p-5 text-left">
              <span className="sunken w-9 h-9 grid place-items-center mb-3.5">
                <Icon className="w-4 h-4 text-brand" />
              </span>
              <h3 className="font-display font-bold text-ink text-[15px] mb-1">{title}</h3>
              <p className="text-muted text-sm leading-relaxed">{body}</p>
            </div>
          ))}
        </div>
      </section>

      <footer className="shell py-5 text-center text-xs text-muted border-t border-line">
        Cumulus — cloud storage, aggregated.
      </footer>
    </div>
  )
}

function GoogleGlyph() {
  return (
    <span className="grid place-items-center w-5 h-5 rounded-full bg-white shrink-0">
      <svg className="w-3 h-3" viewBox="0 0 24 24">
        <path fill="#EA4335" d="M12 5c1.6 0 3 .6 4.1 1.6l3.1-3.1C17.3 1.7 14.8 1 12 1 7.5 1 3.7 3.6 1.9 7.3l3.7 2.9C6.5 7.4 9 5 12 5z" />
        <path fill="#4285F4" d="M23.5 12.3c0-.8-.1-1.6-.2-2.3H12v4.5h6.5c-.3 1.5-1.1 2.8-2.4 3.7l3.7 2.9c2.2-2 3.7-5 3.7-8.8z" />
        <path fill="#FBBC05" d="M5.6 14.8c-.2-.7-.4-1.5-.4-2.3s.2-1.6.4-2.3L1.9 7.3C.7 9.7 0 10.8 0 12s.7 2.3 1.9 4.7l3.7-2.9z" />
        <path fill="#34A853" d="M12 23c3.2 0 6-1.1 8-3l-3.7-2.9c-1.1.7-2.5 1.2-4.3 1.2-3 0-5.5-2.4-6.4-5.2L1.9 16C3.7 19.7 7.5 23 12 23z" />
      </svg>
    </span>
  )
}
