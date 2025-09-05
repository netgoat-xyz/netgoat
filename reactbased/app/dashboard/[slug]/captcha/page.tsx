"use client";

import React, { useState, useRef, useLayoutEffect } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Drawer, DrawerContent, DrawerHeader, DrawerTitle, DrawerFooter, DrawerClose } from "@/components/ui/drawer";
import { SidebarProvider, SidebarInset } from "@/components/ui/sidebar";
import { AppSidebar } from "@/components/domain-sidebar";
import SiteHeader from "@/components/site-header";
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from "@/components/ui/select";
import { AnimatePresence, motion } from "framer-motion";

export default function CaptchaConfigPage({ params }: { params: { domain: string; slug: string } }) {
  const [enabled, setEnabled] = useState(true);
  const [difficulty, setDifficulty] = useState("medium");
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [captchaType, setCaptchaType] = useState("cloudable");
  // Provider-specific state
  const [recaptchaSiteKey, setRecaptchaSiteKey] = useState("");
  const [turnstileSiteKey, setTurnstileSiteKey] = useState("");
  const [hcaptchaSiteKey, setHcaptchaSiteKey] = useState("");
  const [cloudableChallenge, setCloudableChallenge] = useState("basic_click");

  return (

        <div className="flex flex-1 flex-col">
          <div className="@container/main flex flex-1 flex-col gap-2">
            <div className="flex flex-col gap-4 py-4 md:gap-6 md:py-6">
              <div className="p-6 space-y-6">
                <Card>
                  <CardHeader>
                    <CardTitle>Captcha Settings</CardTitle>
                  </CardHeader>
                  <CardContent className="space-y-6">
                    <div className="flex items-center justify-between">
                      <span className="font-medium">Enable Captcha</span>
                      <Switch checked={enabled} onCheckedChange={setEnabled} />
                    </div>
                    <div>
                      <span className="font-medium">Captcha Provider</span>
                      <Select value={captchaType} onValueChange={setCaptchaType}>
                        <SelectTrigger className="mt-2 w-full md:w-1/2">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="recaptcha">Google reCAPTCHA</SelectItem>
                          <SelectItem value="turnstile">Cloudflare Turnstile</SelectItem>
                          <SelectItem value="hcaptcha">hCaptcha</SelectItem>
                          <SelectItem value="cloudable">Cloudable Captcha</SelectItem>
                        </SelectContent>
                      </Select>
                    </div>
                    {captchaType === "recaptcha" && (
                      <div className="mt-4">
                        <span className="font-medium">reCAPTCHA Site Key</span>
                        <Input
                          placeholder="Enter your reCAPTCHA site key"
                          value={recaptchaSiteKey}
                          onChange={e => setRecaptchaSiteKey(e.target.value)}
                          className="mt-2"
                        />
                      </div>
                    )}
                    {captchaType === "turnstile" && (
                      <div className="mt-4">
                        <span className="font-medium">Cloudflare Turnstile Site Key</span>
                        <Input
                          placeholder="Enter your Turnstile site key"
                          value={turnstileSiteKey}
                          onChange={e => setTurnstileSiteKey(e.target.value)}
                          className="mt-2"
                        />
                      </div>
                    )}
                    {captchaType === "hcaptcha" && (
                      <div className="mt-4">
                        <span className="font-medium">hCaptcha Site Key</span>
                        <Input
                          placeholder="Enter your hCaptcha site key"
                          value={hcaptchaSiteKey}
                          onChange={e => setHcaptchaSiteKey(e.target.value)}
                          className="mt-2"
                        />
                      </div>
                    )}
                    {captchaType === "cloudable" && (
                      <div className="mt-4">
                        <span className="font-medium">Cloudable Captcha Challenge Type</span>
                        <Select value={cloudableChallenge} onValueChange={setCloudableChallenge}>
                          <SelectTrigger className="mt-2 w-full md:w-1/2">
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="basic_click">Basic Click</SelectItem>
                            <SelectItem value="image_selection">Image Selection</SelectItem>
                            <SelectItem value="audio">Audio</SelectItem>
                            <SelectItem value="image_text">Image Text</SelectItem>
                            <SelectItem value="basic_click_mouse_tracking">Basic Click + Mouse Tracking</SelectItem>
                            <SelectItem value="emotion">Emotion Describing</SelectItem>
                          </SelectContent>
                        </Select>
                        <div className="mt-2">
                          <span className="text-xs text-muted-foreground">
                            Choose the type of challenge users will solve for Cloudable Captcha.
                          </span>
                        </div>
                      </div>
                    )}
                    <div className="mt-4">
                      <span className="font-medium">Difficulty</span>
                      <Tabs defaultValue={difficulty} onValueChange={setDifficulty} className="mt-2">
                        <div className="relative">
                          <TabsList className="relative flex bg-muted rounded-lg overflow-hidden">
                            {["easy", "medium", "hard"].map((tab) => (
                              <TabsTrigger
                                key={tab}
                                value={tab}
                                className="relative z-10 flex-1"
                              >
                                {tab.charAt(0).toUpperCase() + tab.slice(1)}
                              </TabsTrigger>
                            ))}
                          </TabsList>
                        </div>
                        <div className="relative min-h-[32px]">
                          <AnimatePresence initial={false} custom={difficulty} mode="wait">
                            {difficulty === "easy" && (
                              <motion.div
                                key="easy"
                                initial={{ opacity: 0, y: 10 }}
                                animate={{ opacity: 1, y: 0 }}
                                exit={{ opacity: 0, y: -10 }}
                                transition={{ duration: 0.25, type: "spring", bounce: 0.2 }}
                                className="mt-2 text-green-500"
                              >
                                Easy: Simple challenges, minimal user friction.
                              </motion.div>
                            )}
                            {difficulty === "medium" && (
                              <motion.div
                                key="medium"
                                initial={{ opacity: 0, y: 10 }}
                                animate={{ opacity: 1, y: 0 }}
                                exit={{ opacity: 0, y: -10 }}
                                transition={{ duration: 0.25, type: "spring", bounce: 0.2 }}
                                className="mt-2 text-yellow-500"
                              >
                                Medium: Balanced security and usability.
                              </motion.div>
                            )}
                            {difficulty === "hard" && (
                              <motion.div
                                key="hard"
                                initial={{ opacity: 0, y: 10 }}
                                animate={{ opacity: 1, y: 0 }}
                                exit={{ opacity: 0, y: -10 }}
                                transition={{ duration: 0.25, type: "spring", bounce: 0.2 }}
                                className="mt-2 text-red-500"
                              >
                                Hard: Strongest protection, may challenge real users.
                              </motion.div>
                            )}
                          </AnimatePresence>
                        </div>
                      </Tabs>
                    </div>
                    <Button className="w-full mt-4">Save Configuration</Button>
                  </CardContent>
                </Card>
                {/* ...existing Drawer for advanced logic if needed... */}
              </div>
            </div>
          </div>
        </div>

  );
}