"use client"
import {
  Disclosure,
  Transition,
} from '@headlessui/react'
import {
  ChevronUpIcon,
  CheckCircleIcon,
  XCircleIcon,
  ExclamationCircleIcon,
} from '@heroicons/react/24/solid'

const monitors = [
  {
    group: 'Core Systems',
    items: [
      { name: 'API Gateway', status: 'operational' },
      { name: 'Authentication', status: 'degraded' },
    ],
  },
  {
    group: 'Database Cluster',
    items: [
      { name: 'Primary DB', status: 'operational' },
      { name: 'Replica DB', status: 'down' },
    ],
  },
]

const statusMap = {
  operational: {
    label: 'Operational',
    color: 'text-green-400',
    icon: CheckCircleIcon,
  },
  degraded: {
    label: 'Degraded',
    color: 'text-yellow-400',
    icon: ExclamationCircleIcon,
  },
  down: {
    label: 'Down',
    color: 'text-red-500',
    icon: XCircleIcon,
  },
}

export default function StatusPage() {
  return (
    <div className="min-h-screen bg-gradient-to-br from-zinc-900 to-black text-white px-4 py-10">
      <div className="mx-auto max-w-3xl space-y-6">
        <h1 className="text-3xl font-bold text-white">System Status</h1>

        {monitors.map((group, idx) => (
          <Disclosure key={idx} defaultOpen>
            {({ open }) => (
              <div className="rounded-2xl bg-white/5 backdrop-blur-md p-4 shadow-inner ring-1 ring-white/10">
                <Disclosure.Button className="flex w-full items-center justify-between text-left text-lg font-semibold text-white">
                  <span>{group.group}</span>
                  <ChevronUpIcon
                    className={`h-5 w-5 transform transition-transform ${
                      open ? 'rotate-180' : ''
                    }`}
                  />
                </Disclosure.Button>

                <Transition
                  show={open}
                  enter="transition ease-out duration-200"
                  enterFrom="opacity-0 -translate-y-2"
                  enterTo="opacity-100 translate-y-0"
                  leave="transition ease-in duration-150"
                  leaveFrom="opacity-100"
                  leaveTo="opacity-0"
                >
                  <Disclosure.Panel className="mt-4 space-y-3">
                    {group.items.map((monitor, i) => {
                      const status = statusMap[monitor.status]
                      const Icon = status.icon

                      return (
                        <div
                          key={i}
                          className="flex items-center justify-between rounded-lg bg-white/10 p-3 ring-1 ring-white/10"
                        >
                          <span className="text-base font-medium">
                            {monitor.name}
                          </span>
                          <div className="flex items-center gap-1">
                            <Icon className={`w-5 h-5 ${status.color}`} />
                            <span className={`text-sm ${status.color}`}>
                              {status.label}
                            </span>
                          </div>
                        </div>
                      )
                    })}
                  </Disclosure.Panel>
                </Transition>
              </div>
            )}
          </Disclosure>
        ))}
      </div>
    </div>
  )
}
