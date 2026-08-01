import { useRef, useEffect, useState } from 'react'
import { gsap } from 'gsap'
import { ScrollTrigger } from 'gsap/ScrollTrigger'
import { SplitText as GSAPSplitText } from 'gsap/SplitText'
import { useGSAP } from '@gsap/react'

gsap.registerPlugin(ScrollTrigger, GSAPSplitText, useGSAP)

/**
 * SplitText (React Bits) — splits a string into chars/words/lines and staggers
 * them in on scroll. Used only on the landing page headline and subhead.
 *
 * Two adaptations for this codebase:
 *  - `prefers-reduced-motion` short-circuits the split entirely and renders
 *    plain text, matching the rest of the design system.
 *  - the split targets start hidden via CSS, so a slow font load can never
 *    flash the finished text before the tween takes over.
 */
const prefersReducedMotion = () =>
  typeof window !== 'undefined' &&
  window.matchMedia?.('(prefers-reduced-motion: reduce)').matches

const SplitText = ({
  text,
  className = '',
  delay = 50,
  duration = 1.25,
  ease = 'power3.out',
  splitType = 'chars',
  from = { opacity: 0, y: 40 },
  to = { opacity: 1, y: 0 },
  threshold = 0.1,
  rootMargin = '-100px',
  textAlign = 'center',
  tag = 'p',
  onLetterAnimationComplete,
}) => {
  const ref = useRef(null)
  const animationCompletedRef = useRef(false)
  const onCompleteRef = useRef(onLetterAnimationComplete)
  const [fontsLoaded, setFontsLoaded] = useState(false)
  const [reduced] = useState(prefersReducedMotion)

  // Keep callback ref updated
  useEffect(() => {
    onCompleteRef.current = onLetterAnimationComplete
  }, [onLetterAnimationComplete])

  useEffect(() => {
    if (document.fonts.status === 'loaded') {
      setFontsLoaded(true)
    } else {
      document.fonts.ready.then(() => {
        setFontsLoaded(true)
      })
    }
  }, [])

  useGSAP(
    () => {
      if (reduced) return
      if (!ref.current || !text || !fontsLoaded) return
      // Prevent re-animation if already completed
      if (animationCompletedRef.current) return
      const el = ref.current

      if (el._rbsplitInstance) {
        try {
          el._rbsplitInstance.revert()
        } catch {
          /* noop */
        }
        el._rbsplitInstance = null
      }

      const startPct = (1 - threshold) * 100
      const marginMatch = /^(-?\d+(?:\.\d+)?)(px|em|rem|%)?$/.exec(rootMargin)
      const marginValue = marginMatch ? parseFloat(marginMatch[1]) : 0
      const marginUnit = marginMatch ? marginMatch[2] || 'px' : 'px'
      const sign =
        marginValue === 0
          ? ''
          : marginValue < 0
            ? `-=${Math.abs(marginValue)}${marginUnit}`
            : `+=${marginValue}${marginUnit}`
      const start = `top ${startPct}%${sign}`

      let targets
      const assignTargets = (self) => {
        if (splitType.includes('chars') && self.chars.length) targets = self.chars
        if (!targets && splitType.includes('words') && self.words.length) targets = self.words
        if (!targets && splitType.includes('lines') && self.lines.length) targets = self.lines
        if (!targets) targets = self.chars || self.words || self.lines
      }

      const splitInstance = new GSAPSplitText(el, {
        type: splitType,
        smartWrap: true,
        autoSplit: splitType === 'lines',
        linesClass: 'split-line',
        wordsClass: 'split-word',
        charsClass: 'split-char',
        reduceWhiteSpace: false,
        onSplit: (self) => {
          assignTargets(self)
          const tween = gsap.fromTo(
            targets,
            { ...from },
            {
              ...to,
              duration,
              ease,
              stagger: delay / 1000,
              scrollTrigger: {
                trigger: el,
                start,
                once: true,
                fastScrollEnd: true,
                anticipatePin: 0.4,
              },
              onComplete: () => {
                animationCompletedRef.current = true
                onCompleteRef.current?.()
              },
              willChange: 'transform, opacity',
              force3D: true,
            },
          )
          return tween
        },
      })

      el._rbsplitInstance = splitInstance

      return () => {
        ScrollTrigger.getAll().forEach((st) => {
          if (st.trigger === el) st.kill()
        })
        try {
          splitInstance.revert()
        } catch {
          /* noop */
        }
        el._rbsplitInstance = null
      }
    },
    {
      dependencies: [
        text,
        delay,
        duration,
        ease,
        splitType,
        JSON.stringify(from),
        JSON.stringify(to),
        threshold,
        rootMargin,
        fontsLoaded,
        reduced,
      ],
      scope: ref,
    },
  )

  const style = {
    textAlign,
    overflow: 'hidden',
    display: 'inline-block',
    whiteSpace: 'normal',
    wordWrap: 'break-word',
    willChange: 'transform, opacity',
  }
  const Tag = tag || 'p'

  return (
    <Tag ref={ref} style={style} className={`split-parent ${className}`}>
      {text}
    </Tag>
  )
}

export default SplitText
