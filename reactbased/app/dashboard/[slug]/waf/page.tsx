"use client";

import { useState } from "react";
import { AppSidebar } from "@/components/domain-sidebar";
import SiteHeader from "@/components/site-header";
import { SidebarInset, SidebarProvider } from "@/components/ui/sidebar";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Switch } from "@/components/ui/switch";
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from "@/components/ui/table";
import React from "react";
import {
  DndContext,
  closestCenter,
  PointerSensor,
  useSensor,
  useSensors,
} from "@dnd-kit/core";
import {
  arrayMove,
  useSortable,
  SortableContext,
  verticalListSortingStrategy,
} from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import { GripVertical } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { motion } from "framer-motion";
import { Drawer, DrawerContent, DrawerHeader, DrawerTitle, DrawerDescription, DrawerFooter, DrawerClose } from "@/components/ui/drawer";
import { Prism as SyntaxHighlighter } from "react-syntax-highlighter";
import vscDarkPlus from "react-syntax-highlighter/dist/esm/styles/prism/vsc-dark-plus";
import MonacoEditor from "@monaco-editor/react";

const wafRules = [
  { id: 1, name: "SQL Injection", status: true, description: "Blocks SQLi attempts" },
  { id: 2, name: "XSS Filter", status: true, description: "Blocks XSS attacks" },
  { id: 3, name: "Bad Bots", status: false, description: "Blocks known bad bots" },
  { id: 4, name: "Rate Limiting", status: true, description: "Limits excessive requests" },
];

function DraggableRow({ rule, listeners, attributes, isDragging, toggleRule, ref, style, transform, transition }: any) {
  return (
    <motion.tr
      ref={ref}
      layout
      initial={false}
      animate={{
        scale: isDragging ? 1.04 : 1,
        y: transform ? transform.y : 0,
        boxShadow: isDragging ? "0 8px 32px 0 rgba(31, 38, 135, 0.37)" : "0 0px 0px 0 rgba(0,0,0,0)",
        opacity: isDragging ? 0.7 : 1,
        zIndex: isDragging ? 10 : 1,
        backgroundColor: isDragging ? "rgba(255,255,255,0.04)" : "unset",
      }}
      transition={{
        type: "spring",
        stiffness: 500,
        damping: 30,
        ...transition,
      }}
      style={{
        cursor: "grab",
        position: isDragging ? "relative" : undefined,
        ...style,
      }}
      {...attributes}
      {...listeners}
    >
      <TableCell className="font-medium flex items-center gap-2">
        <span className="cursor-grab text-muted-foreground">
          <GripVertical size={16} />
        </span>
        {rule.name}
      </TableCell>
      <TableCell>
        {rule.description}
        {rule.code && (
          <div className="mt-2 rounded bg-muted p-2 overflow-x-auto">
            <SyntaxHighlighter language="javascript" style={vscDarkPlus} customStyle={{ background: "transparent", fontSize: 13, margin: 0, padding: 0 }}>
              {String(rule.code)}
            </SyntaxHighlighter>
          </div>
        )}
      </TableCell>
      <TableCell>
        <Switch checked={rule.status} onCheckedChange={() => toggleRule(rule.id)} />
        <span className={"ml-2 " + (rule.status ? "text-green-500" : "text-red-500")}>{rule.status ? "On" : "Off"}</span>
      </TableCell>
    </motion.tr>
  );
}

export default function WAFPage({ params }: { params: Promise<{ domain: string; slug: string }> }) {
  const [enabled, setEnabled] = useState(true);
  const [rules, setRules] = useState(wafRules);
  const [loading, setLoading] = useState(false);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [newRule, setNewRule] = useState({ name: "", description: "", code: "" });
  const { slug } = React.use(params);

  // DnD-kit setup
  const sensors = useSensors(useSensor(PointerSensor));
  const handleDragEnd = (event: any) => {
    const { active, over } = event;
    if (active.id !== over.id) {
      setRules((items) => {
        const oldIndex = items.findIndex((i) => i.id === active.id);
        const newIndex = items.findIndex((i) => i.id === over.id);
        return arrayMove(items, oldIndex, newIndex);
      });
    }
  };

  // Simulate toggling a rule
  const toggleRule = (id: number) => {
    setRules((prev) => prev.map(rule => rule.id === id ? { ...rule, status: !rule.status } : rule));
  };

  // Add new rule
  const addRule = () => {
    if (!newRule.name.trim() || !newRule.code.trim()) return;
    setRules((prev) => [
      ...prev,
      {
        id: prev.length ? Math.max(...prev.map(r => r.id)) + 1 : 1,
        name: newRule.name,
        description: newRule.description,
        code: newRule.code,
        status: true,
      },
    ]);
    setNewRule({ name: "", description: "", code: "" });
    setDrawerOpen(false);
  };

  return (
        <div className="flex flex-1 flex-col">
          <div className="@container/main flex flex-1 flex-col gap-2">
            <div className="flex flex-col gap-4 py-4 md:gap-6 md:py-6">
              <div className="p-6 space-y-6">
                <Card>
                  <CardHeader>
                    <CardTitle>WAF Protection</CardTitle>
                    <CardDescription>
                      Protect your application from common web attacks. Toggle the WAF or manage individual rules below.
                    </CardDescription>
                  </CardHeader>
                  <CardContent className="flex items-center gap-4">
                    <span className="font-medium text-lg">WAF Status:</span>
                    <Switch checked={enabled} onCheckedChange={setEnabled} />
                    <span className={enabled ? "text-green-500" : "text-red-500"}>{enabled ? "Enabled" : "Disabled"}</span>
                  </CardContent>
                </Card>
                <Card>
                  <CardHeader>
                    <CardTitle>WAF Rules</CardTitle>
                    <CardDescription>
                      Enable, disable, reorder, or add new protections. <br />
                      <span className="text-xs text-muted-foreground">Top = First to execute, Bottom = Last to execute</span>
                    </CardDescription>
                  </CardHeader>
                  <CardContent>
                    <div className="flex flex-col md:flex-row gap-2 mb-4">
                      <Button onClick={() => setDrawerOpen(true)} className="w-full md:w-auto">Add Rule</Button>
                    </div>
                    <Drawer open={drawerOpen} onOpenChange={setDrawerOpen}>
                      <DrawerContent>
                        <DrawerHeader>
                          <DrawerTitle>Add WAF Rule</DrawerTitle>
                          <DrawerDescription>Define a new WAF rule. You can enter code for custom logic.</DrawerDescription>
                        </DrawerHeader>
                        <div className="flex flex-col gap-4 p-4">
                          <div className="flex flex-row gap-2">
                            <Input
                              placeholder="Rule name"
                              value={newRule.name}
                              onChange={e => setNewRule(r => ({ ...r, name: e.target.value }))}
                              className="w-1/2"
                            />
                            <Input
                              placeholder="Description"
                              value={newRule.description}
                              onChange={e => setNewRule(r => ({ ...r, description: e.target.value }))}
                              className="w-1/2"
                            />
                          </div>
                          <div className="w-full min-h-[120px] rounded border bg-background p-2 font-mono text-sm">
                            <MonacoEditor
                              height="160px"
                              defaultLanguage="javascript"
                              value={newRule.code}
                              onChange={code => setNewRule(r => ({ ...r, code: code ?? "" }))}
                              options={{
                                minimap: { enabled: false },
                                fontSize: 13,
                                fontFamily: "monospace",
                                wordWrap: "on",
                                scrollBeyondLastLine: false,
                                automaticLayout: true,
                              }}
                              theme="vs-dark"
                            />
                          </div>
                          <div className="flex flex-row gap-2 justify-end">
                            <Button onClick={addRule}>Add Rule</Button>
                            <DrawerClose asChild>
                              <Button variant="outline">Cancel</Button>
                            </DrawerClose>
                          </div>
                        </div>
                      </DrawerContent>
                    </Drawer>
                    <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleDragEnd}>
                      <SortableContext items={rules.map((r) => r.id)} strategy={verticalListSortingStrategy}>
                        <Table>
                          <TableHeader>
                            <TableRow>
                              <TableHead>Rule</TableHead>
                              <TableHead>Description</TableHead>
                              <TableHead>Status</TableHead>
                            </TableRow>
                          </TableHeader>
                          <TableBody>
                            {rules.map((rule, idx) => {
                              const { setNodeRef, attributes, listeners, isDragging, transform, transition } = useSortable({ id: rule.id });
                              return (
                                <DraggableRow
                                  key={rule.id}
                                  rule={rule}
                                  ref={setNodeRef}
                                  listeners={listeners}
                                  attributes={attributes}
                                  isDragging={isDragging}
                                  toggleRule={toggleRule}
                                  transform={transform}
                                  transition={transition}
                                />
                              );
                            })}
                          </TableBody>
                        </Table>
                      </SortableContext>
                    </DndContext>
                    <div className="text-xs text-muted-foreground mt-2">
                      <b>Order matters:</b> Rules at the top are executed first, those at the bottom last.
                    </div>
                  </CardContent>
                </Card>
              </div>
            </div>
          </div>
        </div>

  );
}
