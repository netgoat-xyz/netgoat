import dynamic from "next/dynamic";

const DNSPageContent = dynamic(() => import("@/components/DNS-Record-Page"));

const data = [
  {
    type: "A",
    name: "@",
    content: "69.69.69.69",
    status: "unproxied" as "unproxied",
    ttl: "auto",
  },
  {
    type: "A",
    name: "admin",
    content: "69.69.69.69",
    status: "proxied" as "proxied",
    ttl: "auto",
  },
];

export default async function Page({
  params,
}: {
  params: { domain: string; slug: string };
}) {
  const slug = params.slug;
  return <DNSPageContent slug={slug} data={data} />;
}
