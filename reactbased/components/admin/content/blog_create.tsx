"use client";

import { useState } from "react";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { ScrollArea } from "@/components/ui/scroll-area";
import { toast } from "sonner";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { PageTitle } from "../SiteTitle";

interface BlogForm {
  title: string;
  subtitle: string;
  excerpt: string;
  category: string;
  tags: string;
  featuredImage: string;
  readTime: string;
  content: string;
  author: string;
}

export default function BlogCreate() {
  const [form, setForm] = useState<BlogForm>({
    title: "",
    subtitle: "",
    excerpt: "",
    category: "",
    tags: "",
    featuredImage: "",
    readTime: "",
    content: "",
    author: "",
  });
  const [loading, setLoading] = useState(false);

  const handleChange = (
    e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>
  ) => {
    setForm({ ...form, [e.target.name]: e.target.value });
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    try {
      const userFDRaw = localStorage.getItem("userFD");
      if (!userFDRaw) {
        toast("Error", { description: "No logged in user found." });
        setLoading(false);
        return;
      }

      const userFD = JSON.parse(userFDRaw);
      const payload = {
        ...form,
        author: userFD.username,
      };
      const res = await fetch("/api/blog", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      });

      if (!res.ok) return toast("Error", { description: "Could not create blog post." });

      toast("Blog created!", {
        description: "Your blog post was successfully added.",
      });

      setForm({
        title: "",
        subtitle: "",
        excerpt: "",
        category: "",
        tags: "",
        featuredImage: "",
        readTime: "",
        content: "",
        author: "",
      });
    } catch {
      toast("Error", { description: "Could not create blog post." });
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="p-6 max-w-5xl mx-auto space-y-8">
      <PageTitle
        title="Blog Management"
        subtitle="Create, edit, and manage blog posts."
      />

      <form onSubmit={handleSubmit} className="space-y-6">
        <div className="grid gap-4 grid-cols-1 md:grid-cols-2">
          <div className="flex flex-col space-y-2">
            <Label htmlFor="title">Title</Label>
            <Input
              id="title"
              name="title"
              value={form.title}
              onChange={handleChange}
              required
            />
          </div>

          <div className="flex flex-col space-y-2">
            <Label htmlFor="subtitle">Subtitle</Label>
            <Input
              id="subtitle"
              name="subtitle"
              value={form.subtitle}
              onChange={handleChange}
              placeholder="Optional"
            />
          </div>

          <div className="flex flex-col space-y-2">
            <Label htmlFor="category">Category</Label>
            <Input
              id="category"
              name="category"
              value={form.category}
              onChange={handleChange}
              required
            />
          </div>

          <div className="flex flex-col space-y-2">
            <Label htmlFor="tags">Tags</Label>
            <Input
              id="tags"
              name="tags"
              value={form.tags}
              onChange={handleChange}
              placeholder="Comma separated tags"
            />
          </div>

          <div className="flex flex-col space-y-2">
            <Label htmlFor="readTime">Read Time</Label>
            <Input
              id="readTime"
              name="readTime"
              value={form.readTime}
              onChange={handleChange}
              placeholder="e.g., 5 min read"
              required
            />
          </div>

          <div className="flex flex-col space-y-2">
            <Label htmlFor="featuredImage">Featured Image URL</Label>
            <Input
              id="featuredImage"
              name="featuredImage"
              value={form.featuredImage}
              onChange={handleChange}
              placeholder="Optional image for preview cards"
            />
          </div>

          <div className="flex flex-col space-y-2 md:col-span-2">
            <Label htmlFor="excerpt">Excerpt</Label>
            <Textarea
              id="excerpt"
              name="excerpt"
              value={form.excerpt}
              onChange={handleChange}
              rows={3}
              placeholder="Short teaser for your blog"
              required
            />
          </div>
        </div>

        {/* Write / Preview Tabs */}
        <Tabs defaultValue="write" className="space-y-2">
          <TabsList>
            <TabsTrigger value="write" className="px-4 py-2">
              Write
            </TabsTrigger>
            <TabsTrigger value="preview" className="px-4 py-2">
              Preview
            </TabsTrigger>
          </TabsList>

          <TabsContent value="write">
            <Textarea
              id="content"
              name="content"
              value={form.content}
              onChange={handleChange}
              className="h-64 resize-none border rounded-md p-2 focus-visible:ring-1 focus-visible:ring-primary"
              placeholder="Write your blog in markdown..."
              required
            />
          </TabsContent>

          <TabsContent value="preview">
            <ScrollArea className="h-64 border rounded-md p-4 prose prose-invert max-w-full overflow-y-auto">
              <ReactMarkdown remarkPlugins={[remarkGfm]}>
                {form.content || "Nothing to preview yet..."}
              </ReactMarkdown>
            </ScrollArea>
          </TabsContent>
        </Tabs>

        <Button type="submit" disabled={loading}>
          {loading ? "Saving..." : "Create Blog"}
        </Button>
      </form>
    </div>
  );
}
