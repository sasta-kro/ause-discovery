import { useEffect, useRef, useState } from 'react'

// Project Logo loading lifecycle shared by the search-result, public
// Project-detail, and administrator-preview identities. Initials remain the
// visible placeholder until the browser reports a successful load and the
// image decodes, so no partially prepared Logo is ever painted.
export type DecodedLogoStatus = 'absent' | 'pending' | 'ready' | 'failed'

export type DecodedLogoBinding = {
  status: DecodedLogoStatus
  imageProps: {
    key: string
    ref: (element: HTMLImageElement | null) => void
    onLoad: () => void
    onError: () => void
  }
}

// useDecodedLogo owns one Logo request per URL. Callers must spread
// imageProps (including the key) onto the img element so a URL change mounts
// a fresh element: events from a detached previous element then arrive with
// that element's own captured URL and can never settle the replacement
// request. Decode promises carry the same captured URL, so a late decode
// settlement from a superseded request is ignored too.
export function useDecodedLogo(logoUrl?: string | null): DecodedLogoBinding {
  const [status, setStatus] = useState<DecodedLogoStatus>(logoUrl ? 'pending' : 'absent')
  const imageRef = useRef<HTMLImageElement | null>(null)
  const urlRef = useRef(logoUrl)
  const aliveRef = useRef(true)

  // A changed versioned URL immediately returns to pending; the render-time
  // comparison avoids a one-frame stale image from the previous URL.
  if (urlRef.current !== logoUrl) {
    urlRef.current = logoUrl
    setStatus(logoUrl ? 'pending' : 'absent')
  }

  useEffect(() => {
    aliveRef.current = true
    setStatus(logoUrl ? 'pending' : 'absent')
    // An image restored from the immutable cache can already be complete
    // before the load listener attaches; it still reaches the decode path.
    const image = imageRef.current
    if (logoUrl && image?.complete && image.naturalWidth > 0) {
      decodeAndSettle(image, logoUrl)
    }
    return () => {
      aliveRef.current = false
    }
    // decodeAndSettle only touches refs and the settled status, so the
    // effect can depend on the URL alone.
  }, [logoUrl])

  function decodeAndSettle(image: HTMLImageElement, url: string) {
    const settle = (next: DecodedLogoStatus) => {
      if (aliveRef.current && urlRef.current === url) setStatus(next)
    }
    if (typeof image.decode !== 'function') {
      settle('ready')
      return
    }
    Promise.resolve(image.decode()).then(() => settle('ready'), () => settle('failed'))
  }

  // Handlers capture the URL of the render that mounted the element. An
  // event from an older element arrives with its older closure, whose URL
  // no longer matches, so it is ignored.
  const requestUrl = logoUrl ?? ''
  return {
    status,
    imageProps: {
      key: requestUrl,
      ref: (element: HTMLImageElement | null) => {
        imageRef.current = element
      },
      onLoad: () => {
        const image = imageRef.current
        if (image && requestUrl) decodeAndSettle(image, requestUrl)
      },
      onError: () => {
        if (aliveRef.current && requestUrl && urlRef.current === requestUrl) setStatus('failed')
      },
    },
  }
}
