"use client";

import { useState, useEffect } from "react";
import { HslColorPicker } from "react-colorful";
import chroma from "chroma-js";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";

function generatePalette(base: string) {
  const scale = chroma
    .scale([chroma(base).brighten(2), base, chroma(base).darken(2)])
    .mode("lab");
  const steps = [50, 100, 200, 300, 400, 500, 600, 700, 800, 900, 950];
  const colors: Record<number, string> = {};
  steps.forEach((s, i) => (colors[s] = scale(i / (steps.length - 1)).hex()));
  return colors;
}

function textColor(bg: string) {
  return chroma.contrast(bg, "white") > 4.5 ? "#fff" : "#000";
}

export default function DraggyThemeGenerator() {
  const [hsl, setHsl] = useState({ h: 280, s: 0.7, l: 0.5 });
  const [palette, setPalette] = useState(() => {
    const correctedHue = (360 - hsl.h) % 360;
    const base = chroma.hsl(correctedHue, hsl.s, hsl.l).hex();
    return generatePalette(base);
  });

  useEffect(() => {
    const correctedHue = (360 - hsl.h) % 360;
    const base = chroma.hsl(correctedHue, hsl.s, hsl.l).hex();
    setPalette(generatePalette(base));
  }, [hsl]);

  return (
    <div className="p-8 space-y-8 max-w-4xl mx-auto">
      <Card>
        <CardHeader>
          <CardTitle>Drag Color Picker</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col items-center gap-8">
          <HslColorPicker color={hsl} onChange={setHsl} className="w-64 h-64" />
          <div className="grid grid-cols-6 gap-2 w-full">
            {Object.entries(palette).map(([key, color]) => (
              <div
                key={key}
                className="h-16 flex flex-col justify-center items-center rounded"
                style={{ backgroundColor: color, color: textColor(color) }}
              >
                <span className="text-sm font-mono">{key}</span>
              </div>
            ))}
          </div>
          <div className="space-x-4">
            <Button
              style={{
                backgroundColor: palette[500],
                color: textColor(palette[500]),
              }}
            >
              Primary Button
            </Button>
            <Button
              style={{
                backgroundColor: palette[700],
                color: textColor(palette[700]),
              }}
            >
              Accent Button
            </Button>
          </div>
        </CardContent>
      </Card>
      <pre className="bg-gray-900 text-white p-4 rounded overflow-x-auto">
        {JSON.stringify(palette, null, 2)}
      </pre>
    </div>
  );
}
