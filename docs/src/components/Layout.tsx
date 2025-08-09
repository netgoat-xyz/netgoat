'use client'

import Link from 'next/link'
import { usePathname } from 'next/navigation'
import { motion } from 'framer-motion'

import { Footer } from '@/components/Footer'
import { Header } from '@/components/Header'
import { Logo } from '@/components/Logo'
import { Navigation } from '@/components/Navigation'
import { type Section, SectionProvider } from '@/components/SectionProvider'

export function Layout({
  children,
  allSections,
}: {
  children: React.ReactNode
  allSections: Record<string, Array<Section>>
}) {
  let pathname = usePathname()

  return (
    <SectionProvider sections={allSections[pathname] ?? []}>
      <div className="h-full lg:ml-72 xl:ml-80">
        <motion.header
          layoutScroll
          className="contents lg:pointer-events-none lg:fixed lg:inset-0 lg:z-40 lg:flex"
        >
          <div className="contents lg:pointer-events-auto lg:block lg:w-72 lg:overflow-y-auto lg:border-r lg:border-zinc-900/10 lg:px-6 lg:pb-8 lg:pt-4 xl:w-80 lg:dark:border-white/10">
            <div className="hidden lg:flex">
              <Link href="/" aria-label="Home" className="group flex items-center justify-start">
                <Logo className="h-6" />
                <motion.div className="relative flex items-center ml-1.5"
                  initial="rest"
                  whileHover="hover"
                  animate="rest"
                  variants={{ rest: {}, hover: {} }}
                >
                  <span className="relative flex items-center">
                    <h1 className="font-extrabold text-lg tracking-tight text-zinc-900 dark:text-white transition-colors duration-200">
                      Netgoat
                    </h1>
                    <motion.div
                      className="flex items-center "
                      initial={{ x: -32, opacity: 0 }}
                      whileHover={{ x: 0, opacity: 1 }}
                      transition={{ type: 'spring', stiffness: 300, damping: 30 }}
                    >
                      <span className="mx-2 h-6 w-px bg-zinc-400 dark:bg-white" />
                      <span className="font-semibold text-md text-zinc-500 dark:text-zinc-300">Docs</span>
                    </motion.div>
                  </span>
                </motion.div>
              </Link>
            </div>
            <Header />
            <Navigation className="hidden lg:mt-10 lg:block" />
          </div>
        </motion.header>
        <div className="relative flex h-full flex-col px-4 pt-14 sm:px-6 lg:px-8">
          <main className="flex-auto">{children}</main>
          <Footer />
        </div>
      </div>
    </SectionProvider>
  )
}
