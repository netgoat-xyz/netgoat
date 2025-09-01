"use client";

import { Fragment, useState, MouseEvent } from "react";
import Link from "next/link";
import { Popover, Transition } from "@headlessui/react";
import clsx from "clsx";
import Image from "next/image";

import { Button } from "@/components/Button";
import { Container } from "@/components/Container";
import { NavLink } from "@/components/NavLink";
import backgroundImage from "@/public/bg_img/background-call-to-action.jpg";

type Post = {
  id: number;
  title: string;
  href: string;
  description: string;
  imageUrl: string;
  date: string;
  datetime: string;
  category: { title: string; href: string };
  author: {
    name: string;
    role: string;
    href: string;
    imageUrl: string;
  };
};

const posts: Post[] = [
  {
    id: 1,
    title: "Lorem ipsum dolor sit amet",
    href: "#",
    description: "Illo sint voluptas. Error voluptates culpa eligendi...",
    imageUrl:
      "https://images.unsplash.com/photo-1496128858413-b36217c2ce36?ixlib=rb-4.0.3&auto=format&fit=crop&w=3603&q=80",
    date: "Mar 16, 2020",
    datetime: "2020-03-16",
    category: { title: "Marketing", href: "#" },
    author: {
      name: "Michael Foster",
      role: "Co-Founder / CTO",
      href: "#",
      imageUrl:
        "https://images.unsplash.com/photo-1519244703995-f4e0f30006d5?ixlib=rb-1.2.1&auto=format&fit=facearea&facepad=2&w=256&h=256&q=80",
    },
  },
  // Add more posts here...
];

const categoryColors: Record<string, string> = {
  Marketing: "bg-blue-100 text-blue-800",
  Engineering: "bg-green-100 text-green-800",
  Design: "bg-pink-100 text-pink-800",
  Default: "bg-gray-100 text-gray-800",
};

function MobileNavLink({
  href,
  children,
}: {
  href: string;
  children: React.ReactNode;
}) {
  return (
    <Popover.Button as={Link} href={href} className="block w-full p-2">
      {children}
    </Popover.Button>
  );
}

function MobileNavIcon({ open }: { open: boolean }) {
  return (
    <svg
      aria-hidden="true"
      className="h-3.5 w-3.5 overflow-visible stroke-slate-700"
      fill="none"
      strokeWidth={2}
      strokeLinecap="round"
    >
      <path
        d="M0 1H14M0 7H14M0 13H14"
        className={clsx(
          "origin-center transition",
          open && "scale-90 opacity-0"
        )}
      />
      <path
        d="M2 2L12 12M12 2L2 12"
        className={clsx(
          "origin-center transition",
          !open && "scale-90 opacity-0"
        )}
      />
    </svg>
  );
}

function MobileNavigation() {
  return (
    <Popover>
      <Popover.Button
        className="relative z-10 flex h-8 w-8 items-center justify-center ui-not-focus-visible:outline-none"
        aria-label="Toggle Navigation"
      >
        {({ open }) => <MobileNavIcon open={open} />}
      </Popover.Button>
      <Transition.Root>
        <Transition.Child
          as={Fragment}
          enter="duration-150 ease-out"
          enterFrom="opacity-0"
          enterTo="opacity-100"
          leave="duration-150 ease-in"
          leaveFrom="opacity-100"
          leaveTo="opacity-0"
        >
          <Popover.Overlay className="fixed inset-0 bg-slate-300/50" />
        </Transition.Child>
        <Transition.Child
          as={Fragment}
          enter="duration-150 ease-out"
          enterFrom="opacity-0 scale-95"
          enterTo="opacity-100 scale-100"
          leave="duration-100 ease-in"
          leaveFrom="opacity-100 scale-100"
          leaveTo="opacity-0 scale-95"
        >
          <Popover.Panel
            as="div"
            className="absolute inset-x-0 top-full mt-4 flex origin-top flex-col rounded-2xl bg-white p-4 text-lg tracking-tight text-slate-900 shadow-xl ring-1 ring-slate-900/5"
          >
            <MobileNavLink href="#features">Features</MobileNavLink>
            <MobileNavLink href="#testimonials">Testimonials</MobileNavLink>
            <MobileNavLink href="#pricing">Pricing</MobileNavLink>
            <hr className="m-2 border-slate-300/40" />
            <MobileNavLink href="/login">Sign in</MobileNavLink>
          </Popover.Panel>
        </Transition.Child>
      </Transition.Root>
    </Popover>
  );
}

function BlogCard({ post, badgeColor }: { post: Post; badgeColor: string }) {
  const [transform, setTransform] = useState<string>("");

  const handleMouseMove = (e: MouseEvent<HTMLTitleElement>) => {
    const { left, top, width, height } =
      e.currentTarget.getBoundingClientRect();
    const x = e.clientX - left;
    const y = e.clientY - top;
    const rotateX = (y / height - 0.5) * 10;
    const rotateY = (x / width - 0.5) * -10;
    setTransform(
      `perspective(600px) rotateX(${rotateX}deg) rotateY(${rotateY}deg)`
    );
  };

  const resetTransform = () =>
    setTransform("perspective(600px) rotateX(0) rotateY(0)");

  return (
    <article
      onMouseMove={handleMouseMove}
      onMouseLeave={resetTransform}
      style={{ transform }}
      className="flex flex-col items-start bg-gray-100/65 transition-transform duration-200 ease-out filter backdrop-blur rounded-2xl border border-gray-900/10 justify-between"
    >
      <div className="relative w-full">
        <img
          src={post.imageUrl}
          alt={post.title}
          className="aspect-[16/9] w-full rounded-t-2xl bg-gray-100 object-cover sm:aspect-[2/1] lg:aspect-[3/2]"
        />
      </div>
      <div className="max-w-xl px-4 pb-4">
        <div className="mt-8 flex items-center gap-x-4 text-xs">
          <time dateTime={post.datetime} className="text-gray-600">
            {post.date}
          </time>
          <a
            href={post.category.href}
            className={`inline-flex items-center gap-x-1.5 py-1.5 px-3 rounded-full text-xs font-medium ${badgeColor}`}
          >
            {post.category.title}
          </a>
        </div>
        <div className="group relative">
          <h3 className="mt-3 text-lg font-semibold leading-6 text-gray-900 group-hover:text-gray-600">
            <a
              href={post.href}
              className="tracking-tight font-semibold font-inter"
            >
              <span className="absolute inset-0" />
              {post.title}
            </a>
          </h3>
          <p className="mt-2 line-clamp-3 text-sm leading-6 text-gray-700">
            {post.description}
          </p>
          <div className="relative mt-8 flex items-center gap-x-4">
            <img
              src={post.author.imageUrl}
              alt=""
              className="h-10 w-10 rounded-full bg-gray-100"
            />
            <div className="text-sm leading-6">
              <p className="font-semibold text-gray-900">
                <a href={post.author.href}>
                  <span className="absolute inset-0" />
                  {post.author.name}
                </a>
              </p>
              <p className="text-gray-600">{post.author.role}</p>
            </div>
          </div>
        </div>
      </div>
    </article>
  );
}

export default function Blog() {
  return (
    <div className="bg-white dark:bg-white">
      <section id="from-the-blog" className="relative overflow-hidden bg-white">
        <header className="py-8">
          <Container>
            <nav className="relative z-50 flex justify-between">
              <div className="flex items-center md:gap-x-12">
                <Link href="#" aria-label="Home">
                  <h1 className="text-2xl font-semibold leading-tight text-black">
                    Netgoat
                  </h1>
                </Link>
                <div className="hidden md:flex md:gap-x-6"></div>
              </div>
              <div className="flex items-center gap-x-5 md:gap-x-8">
                <div className="hidden md:block">
                  <NavLink href="/login">Sign in</NavLink>
                </div>
                <Button href="/register" color="blue">
                  <span>
                    Get started <span className="hidden lg:inline">today</span>
                  </span>
                </Button>
                <div className="-mr-1 md:hidden">
                  <MobileNavigation />
                </div>
              </div>
            </nav>
          </Container>
        </header>

        <Image
          className="absolute left-1/2 top-1/2 max-w-none -translate-x-1/2 -translate-y-1/2 opacity-20"
          src={backgroundImage}
          alt=""
          width={2347}
          height={1244}
        />
        <div className="mt-10 relative mx-auto max-w-7xl h-full h-screen px-6 lg:px-8">
          <div className="mx-auto max-w-2xl text-center">
            <h2 className="text-3xl font-bold tracking-tight text-gray-900 sm:text-4xl">
              From the blog
            </h2>
            <p className="mt-2 text-lg leading-8 text-gray-600">
              Learn how to grow your business with our expert advice.
            </p>
          </div>

          <div className="mx-auto mt-16 grid max-w-2xl grid-cols-1 gap-x-8 gap-y-20 lg:mx-0 lg:max-w-none lg:grid-cols-3">
            {posts.map((post) => {
              const badgeColor =
                categoryColors[post.category.title] || categoryColors.Default;
              return (
                <BlogCard key={post.id} post={post} badgeColor={badgeColor} />
              );
            })}
          </div>
        </div>
      </section>
    </div>
  );
}
