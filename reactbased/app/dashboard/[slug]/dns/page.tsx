// eslint-disable-next-line @typescript-eslint/prefer-as-const

import dynamic from "next/dynamic";

const DNSPageContent = dynamic(() => import("@/components/DNS-Record-Page"));

const data = [
  {
    type: "A",
    name: "@",
    content: "69.69.69.69",
    status: "unproxied",
    ttl: "auto",
  },
  {
    type: "A",
    name: "admin",
    content: "69.69.69.69",
    status: "proxied",
    ttl: "auto",
  },
] as const;

export default async function Page({
  params,
}: {
  params: Promise<{ domain: string; slug: string }>;
}) {
  const slug = (await params).slug;
  return <DNSPageContent slug={slug} data={data} />;
}
