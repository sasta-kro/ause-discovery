import { useEffect, useRef, useState } from 'react'

// Project Logo loading lifecycle shared by the search-result, public
// Project-detail, and administrator-preview identities. Initials remain the
// visible placeholder until the browser reports a successful load and the
// image decodes, so no partially prepared Logo is ever painted.
export type DecodedLogoStatus = 'absent' | 'pending' | 'ready' | 'failed'

export type DecodedLogoBinding = {
  status: DecodedLogoStatus
  imageProps: {
    ref: (element: HTMLImageElement | null) => void
    onLoad: () => void
    onError: () => void
  }
}

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

  return {
    status,
    imageProps: {
      ref: (element: HTMLImageElement | null) => {
        imageRef.current = element
      },
      onLoad: () => {
        const image = imageRef.current
        const url = urlRef.current
        if (image && url) decodeAndSettle(image, url)
      },
      onError: () => {
        if (aliveRef.current && urlRef.current) setStatus('failed')
      },
    },
  }
}
