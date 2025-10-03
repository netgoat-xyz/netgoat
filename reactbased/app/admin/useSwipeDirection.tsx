import { useEffect, useRef, useState } from "react"
import { usePathname } from "next/navigation"

export default function useSwipeDirection() {
  const pathname = usePathname()
  const prev = useRef(pathname)
  const [direction, setDirection] = useState<"left" | "right">("right")

  useEffect(() => {
    // naive compare: shorter path = "left", longer = "right"
    // tweak logic to your nav structure
    if (pathname !== prev.current) {
      const prevDepth = prev.current.split("/").length
      const nextDepth = pathname.split("/").length
      setDirection(nextDepth < prevDepth ? "left" : "right")
      prev.current = pathname
    }
  }, [pathname])

  return direction
}