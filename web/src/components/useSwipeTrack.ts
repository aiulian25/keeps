import { useCallback, useEffect, useRef, useState } from "react";

/**
 * A horizontal scroll-snap track: the browser provides swiping and momentum;
 * this keeps the current slide index and moves to a slide on request.
 */
export function useSwipeTrack(count: number, startIndex = 0) {
  const trackReference = useRef<HTMLDivElement>(null);
  const [index, setIndex] = useState(startIndex);

  const goTo = useCallback(
    (target: number, smooth = true) => {
      const track = trackReference.current;
      if (!track || count === 0) {
        return;
      }
      const wrapped = (target + count) % count;
      track.scrollTo({
        left: wrapped * track.clientWidth,
        behavior: smooth ? "smooth" : "instant",
      });
      setIndex(wrapped);
    },
    [count],
  );

  useEffect(() => {
    const track = trackReference.current;
    if (!track) {
      return;
    }
    const onScroll = () => {
      if (track.clientWidth > 0) {
        setIndex(Math.round(track.scrollLeft / track.clientWidth));
      }
    };
    track.addEventListener("scroll", onScroll, { passive: true });
    return () => track.removeEventListener("scroll", onScroll);
  }, []);

  return { trackReference, index, goTo };
}
