"use client";

import Header from "@/components/homescreen/header";
import HeroContent from "@/components/homescreen/hero-content";
import ShaderBackground from "@/components/homescreen/shader-background";

export default function ShaderShowcase() {
  return (
    <ShaderBackground>
      <Header />
      <HeroContent />
    </ShaderBackground>
  );
}
