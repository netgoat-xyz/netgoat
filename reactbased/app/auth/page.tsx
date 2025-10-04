"use client";

import { AuthForm } from "@/components/auth-form";
import { useRouter } from "next/navigation";
export default function LoginPage() {
  const router = useRouter();
  return (
    <div className="flex min-h-svh flex-col items-center justify-center p-6 md:p-10">
      <div className="w-full max-w-sm md:max-w-3xl">
        <AuthForm onSuccess={() => router.push("/dashboard")} />
      </div>
    </div>
  );
}
