"use client";

import { useEffect, useState } from "react";
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

interface BlogEditProps {
  slug: string;
  onBack?: () => void;
}

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

export default function BlogEdit({ slug, onBack }: BlogEditProps) {
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
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    async function fetchPost() {
      try {
        const res = await fetch(`/api/blog/${slug}`);
        if (!res.ok) throw new Error("Post not found");
        const data = await res.json();
        setForm({
          title: data.title || "",
          subtitle: data.subtitle || "",
          excerpt: data.excerpt || "",
          category: data.category || "",
          tags: data.tags || "",
          featuredImage: data.featuredImage || "",
          readTime: data.readTime || "",
          content: Array.isArray(data.content) ? data.content.join("\n\n") : data.content || "",
          author: data.author || "",
        });
      } catch {
        toast("Error", { description: "Could not load blog post." });
      } finally {
        setLoading(false);
      }
    }
    fetchPost();
  }, [slug]);

  const handleChange = (
    e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>
  ) => setForm({ ...form, [e.target.name]: e.target.value });

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    try {
      const res = await fetch(`/api/blog/${slug}`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(form),
      });
      if (!res.ok) throw new Error("Update failed");
      toast("Saved!", { description: "Blog post updated." });
    } catch {
      toast("Error", { description: "Failed to update blog post." });
    } finally {
      setSaving(false);
    }
  };

  if (loading) return <div className="p-8 text-center text-muted-foreground">Loading post…</div>;

  return (
    <div className="p-6 max-w-5xl mx-auto space-y-8">
      <PageTitle
        title="Edit Blog"
        subtitle={`Editing: ${form.title || slug}`}
        actions={
          <Button variant="outline" onClick={onBack}>
            ← Back
          </Button>
        }
      />

      <form onSubmit={handleSave} className="space-y-6">
        <div className="grid gap-4 grid-cols-1 md:grid-cols-2">
          {/** Title */}
          <div className="flex flex-col space-y-2">
            <Label htmlFor="title">Title</Label>
            <Input
              id="title"
              name="title"
              value={form.title}
              placeholder="Enter blog title"
              onChange={handleChange}
              required
            />
          </div>

          {/** Subtitle */}
          <div className="flex flex-col space-y-2">
            <Label htmlFor="subtitle">Subtitle</Label>
            <Input
              id="subtitle"
              name="subtitle"
              value={form.subtitle}
              placeholder="Optional subtitle"
              onChange={handleChange}
            />
          </div>

          {/** Category */}
          <div className="flex flex-col space-y-2">
            <Label htmlFor="category">Category</Label>
            <Input
              id="category"
              name="category"
              value={form.category}
              placeholder="Enter category"
              onChange={handleChange}
              required
            />
          </div>

          {/** Tags */}
          <div className="flex flex-col space-y-2">
            <Label htmlFor="tags">Tags</Label>
            <Input
              id="tags"
              name="tags"
              value={form.tags}
              placeholder="Comma separated tags"
              onChange={handleChange}
            />
          </div>

          {/** Read Time */}
          <div className="flex flex-col space-y-2">
            <Label htmlFor="readTime">Read Time</Label>
            <Input
              id="readTime"
              name="readTime"
              value={form.readTime}
              placeholder="e.g., 5 min read"
              onChange={handleChange}
              required
            />
          </div>

          {/** Featured Image */}
          <div className="flex flex-col space-y-2">
            <Label htmlFor="featuredImage">Featured Image URL</Label>
            <Input
              id="featuredImage"
              name="featuredImage"
              value={form.featuredImage}
              placeholder="Optional image URL"
              onChange={handleChange}
            />
          </div>

          {/** Excerpt */}
          <div className="flex flex-col space-y-2 md:col-span-2">
            <Label htmlFor="excerpt">Excerpt</Label>
            <Textarea
              id="excerpt"
              name="excerpt"
              value={form.excerpt}
              placeholder="Short teaser for the blog"
              onChange={handleChange}
              rows={3}
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
              placeholder="Write your blog in markdown..."
              onChange={handleChange}
              className="h-64 resize-none border rounded-md p-2 focus-visible:ring-1 focus-visible:ring-primary"
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

        <Button type="submit" disabled={saving}>
          {saving ? "Saving..." : "Save Changes"}
        </Button>
      </form>
    </div>
  );
}
