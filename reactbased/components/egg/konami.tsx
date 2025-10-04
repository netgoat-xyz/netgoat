"use client";
import { useEffect, useState } from "react";

const konamiSequence = [
  "ArrowUp",
  "ArrowUp",
  "ArrowDown",
  "ArrowDown",
  "ArrowLeft",
  "ArrowRight",
  "ArrowLeft",
  "ArrowRight",
  "b",
  "a",
];

export default function KonamiEasterEgg({
  onActivate,
}: {
  onActivate: () => void;
}) {
  const [position, setPosition] = useState(0);

  useEffect(() => {
    const keyHandler = (e: KeyboardEvent) => {
      if (e.key === konamiSequence[position]) {
        setPosition((pos) => pos + 1);
        if (position + 1 === konamiSequence.length) {
          onActivate();
          setPosition(0);
        }
      } else {
        setPosition(0);
      }
    };
    window.addEventListener("keydown", keyHandler);
    return () => window.removeEventListener("keydown", keyHandler);
  }, [position, onActivate]);

  return null;
}
