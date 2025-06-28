"use client";

import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { useState } from "react";

export function CreateRecordSheet() {
  const [type, setType] = useState("A");
  const [name, setName] = useState("");
  const [content, setContent] = useState("");
  const [ttl, setTtl] = useState("auto");

  const [priority, setPriority] = useState("");
  const [port, setPort] = useState("");
  const [weight, setWeight] = useState("");
  const [target, setTarget] = useState("");
  const [note, setNote] = useState("");

  const renderCustomFields = () => {
    switch (type) {
      case "MX":
        return (
          <>
            <div className="flex w-full items-center space-x-3">
              <div className="w-full">
                <Label className="mb-2">Name</Label>
                <Input value={name} onChange={(e) => setName(e.target.value)} />
              </div>
              <div className="w-full">
                <Label className="mb-2">Priority</Label>
                <Input
                  type="number"
                  value={priority}
                  onChange={(e) => setPriority(e.target.value)}
                />
              </div>
            </div>
            <div className="w-full">
              <Label className="mb-2">Content</Label>
              <Input value={content} onChange={(e) => setContent(e.target.value)} />
            </div>
          </>
        );
      case "CAA":
        return (
          <div className="flex w-full items-center space-x-3">
            <div className="w-full">
              <Label className="mb-2">Name</Label>
              <Input value={name} onChange={(e) => setName(e.target.value)} />
            </div>
            <div className="w-full">
              <Label className="mb-2">Value</Label>
              <Input
                value={content}
                onChange={(e) => setContent(e.target.value)}
                placeholder={`e.g. 0 issue "letsencrypt.org"`}
              />
            </div>
          </div>
        );
      case "SRV":
        return (
          <>
            <div className="flex w-full items-center space-x-3">
              <div className="w-full">
                <Label className="mb-2">Name</Label>
                <Input value={name} onChange={(e) => setName(e.target.value)} />
              </div>
              <div className="w-full">
                <Label className="mb-2">Target</Label>
                <Input value={target} onChange={(e) => setTarget(e.target.value)} />
              </div>
            </div>
            <div className="flex w-full items-center space-x-3">
              <div className="w-full">
                <Label className="mb-2">Priority</Label>
                <Input
                  type="number"
                  value={priority}
                  onChange={(e) => setPriority(e.target.value)}
                />
              </div>
              <div className="w-full">
                <Label className="mb-2">Weight</Label>
                <Input
                  type="number"
                  value={weight}
                  onChange={(e) => setWeight(e.target.value)}
                />
              </div>
              <div className="w-full">
                <Label className="mb-2">Port</Label>
                <Input
                  type="number"
                  value={port}
                  onChange={(e) => setPort(e.target.value)}
                />
              </div>
            </div>
          </>
        );
      default:
        return (
          <div className="flex w-full items-center space-x-3">
            <div className="w-full">
              <Label className="mb-2">Name</Label>
              <Input value={name} onChange={(e) => setName(e.target.value)} />
            </div>
            <div className="w-full">
              <Label className="mb-2">Content</Label>
              <Input value={content} onChange={(e) => setContent(e.target.value)} />
            </div>
          </div>
        );
    }
  };

  return (
    <Sheet>
      <SheetTrigger asChild>
        <Button>Create Record</Button>
      </SheetTrigger>
      <SheetContent side="bottom" className="space-y-4">
        <SheetHeader>
          <SheetTitle>Create DNS Record</SheetTitle>
        </SheetHeader>
        <div className="grid gap-4 p-3">
          <div className="flex items-center gap-4">
            <div>
              <Label className="mb-2">Type</Label>
              <Select value={type} onValueChange={setType}>
                <SelectTrigger>
                  <SelectValue placeholder="Select Type" />
                </SelectTrigger>
                <SelectContent>
                  {[
                    "A",
                    "AAAA",
                    "CNAME",
                    "TXT",
                    "NS",
                    "MX",
                    "SRV",
                    "CAA",
                    "SPF",
                    "PTR",
                    "SOA",
                  ].map((r) => (
                    <SelectItem key={r} value={r}>
                      {r}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div>
              <Label className="mb-2">TTL</Label>
              <Select value={ttl} onValueChange={setTtl}>
                <SelectTrigger>
                  <SelectValue placeholder="auto" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="auto">Auto</SelectItem>
                  <SelectItem value="300">5 min</SelectItem>
                  <SelectItem value="3600">1 hour</SelectItem>
                  <SelectItem value="86400">1 day</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>

          {renderCustomFields()}

          <div>
            <Label htmlFor="note" className="mb-2">
              Note (Optional)
            </Label>
            <Textarea
              placeholder="Type your message here."
              id="note"
              value={note}
              onChange={(e) => setNote(e.target.value)}
            />
          </div>

          <div className="flex justify-end">
            <Button variant="default">Save</Button>
          </div>
        </div>
      </SheetContent>
    </Sheet>
  );
}
