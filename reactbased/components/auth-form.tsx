"use client";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useState, useEffect } from "react";
import { motion, AnimatePresence } from "framer-motion";
import axios from "axios";
import { jwtDecode } from "jwt-decode";
import { verifySession } from "@/lib/session";

export function AuthForm({ className, ...props }: React.ComponentProps<"div">) {
  const [mode, setMode] = useState<"login" | "register">("login");
  const [direction, setDirection] = useState<1 | -1>(1);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  // Only run in browser
  useEffect(() => {
    if (typeof window !== "undefined") {
      const jwtkey = localStorage.getItem("jwt");
      if (jwtkey) {
        verifySession(jwtkey).then((session) => {
          console.log(session);
        });
      }
    }
  }, []);

  // Helper to switch mode and set direction
  const switchMode = (target: "login" | "register") => {
    if (target === mode) return;
    setDirection(target === "register" ? 1 : -1);
    setMode(target);
    setError(null);
  };

  // Handle login
  async function handleLogin(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setLoading(true);
    setError(null);
    const form = e.currentTarget;
    const email = (form.elements.namedItem("email") as HTMLInputElement)?.value;
    const password = (form.elements.namedItem("password") as HTMLInputElement)
      ?.value;
    try {
      const res = await axios.post(process.env.backendapi + "/api/auth/login", {
        email,
        password,
      });
      if (res.data.jwt) {
        // Save JWT to localStorage
        localStorage.setItem("jwt", res.data.jwt);
        // Optionally verify and decode session
        let session = localStorage.getItem("session");
        if (!session) {
          fetch("/api/session?session=" + res.data.jwt)
            .then((res) => res.json())
            .then((data) => {
              console.log("Session:", data);
              localStorage.setItem("session", JSON.stringify(data));
            })
            .catch((err) => console.error("Session fetch failed", err));
        }

        // Optionally: set user in context or state here
        // window.location.reload();
      } else if (res.data.requires2FA) {
        setError("2FA required. Please complete 2FA flow.");
      } else {
        setError("Unknown login response.");
      }
    } catch (err: any) {
      setError(err.response?.data?.error || "Login failed");
    } finally {
      setLoading(false);
    }
  }

  // Handle register
  async function handleRegister(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setLoading(true);
    setError(null);
    const form = e.currentTarget;
    const username = (form.elements.namedItem("username") as HTMLInputElement)
      ?.value;
    const email = (form.elements.namedItem("email") as HTMLInputElement)?.value;
    const password = (form.elements.namedItem("password") as HTMLInputElement)
      ?.value;
    try {
      await axios.post(process.env.backendapi + "/api/auth/register", {
        username,
        email,
        password,
      });
      setMode("login");

      setDirection(-1);
    } catch (err: any) {
      setError(err.response?.data?.error || "Registration failed");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div
      className={cn(
        "flex flex-col w-full gap-6 items-center justify-center min-h-screen",
        className
      )}
      {...props}
    >
      <Card className="overflow-hidden p-0 w-full max-w-[420px] min-w-[320px] mx-auto flex items-center justify-center">
        <CardContent className="flex flex-col p-0 min-h-0 w-full items-center justify-center">
          <div className="p-8 flex flex-col justify-center w-full items-center">
            <div className="flex flex-col gap-10 w-full items-center">
              <div className="flex flex-col items-center min-h-[90px] -mb-6 mt-2 w-full">
                <AnimatePresence mode="wait" initial={false}>
                  <motion.h1
                    key={mode + "-title"}
                    initial={{ y: direction === 1 ? 24 : -24, opacity: 0 }}
                    animate={{ y: 0, opacity: 1 }}
                    exit={{ y: direction === 1 ? -24 : 24, opacity: 0 }}
                    transition={{ type: "spring", stiffness: 400, damping: 30 }}
                    className="text-2xl font-bold"
                  >
                    {mode === "login" ? "Welcome back" : "Create your account"}
                  </motion.h1>
                  <motion.p
                    key={mode + "-desc"}
                    initial={{ y: direction === 1 ? 24 : -24, opacity: 0 }}
                    animate={{ y: 0, opacity: 1 }}
                    exit={{ y: direction === 1 ? -24 : 24, opacity: 0 }}
                    transition={{
                      type: "spring",
                      stiffness: 400,
                      damping: 30,
                      delay: 0.05,
                    }}
                    className="text-muted-foreground text-balance text-base px-2"
                  >
                    {mode === "login"
                      ? "Login to your NetGoat account"
                      : "Sign up for an NetGoat account"}
                  </motion.p>
                </AnimatePresence>
                {error && (
                  <div className="text-red-500 text-sm mt-2">{error}</div>
                )}
              </div>
              <div className="relative flex items-center w-full justify-center">
                <AnimatePresence mode="wait" initial={false}>
                  {mode === "login" ? (
                    <motion.form
                      key="login"
                      initial={{ x: direction === 1 ? -64 : 64, opacity: 0 }}
                      animate={{ x: 0, opacity: 1 }}
                      exit={{ x: direction === 1 ? 64 : -64, opacity: 0 }}
                      transition={{
                        type: "spring",
                        stiffness: 400,
                        damping: 30,
                      }}
                      className="flex flex-col gap-7 w-full items-center"
                      onSubmit={handleLogin}
                    >
                      <div className="grid gap-2 w-full">
                        <Label htmlFor="email">Email</Label>
                        <Input
                          id="email"
                          name="email"
                          type="email"
                          placeholder="m@example.com"
                          required
                        />
                      </div>
                      <div className="grid gap-2 w-full">
                        <div className="flex items-center">
                          <Label htmlFor="password">Password</Label>
                          <a
                            href="#"
                            className="ml-auto text-sm underline-offset-2 hover:underline"
                          >
                            Forgot your password?
                          </a>
                        </div>
                        <Input
                          id="password"
                          name="password"
                          type="password"
                          required
                        />
                      </div>
                      <Button
                        type="submit"
                        className="w-full"
                        disabled={loading}
                      >
                        {loading ? "Logging in..." : "Login"}
                      </Button>
                      <div className="text-center text-sm mt-2">
                        Don&apos;t have an account?{" "}
                        <button
                          type="button"
                          className="underline underline-offset-4"
                          onClick={() => switchMode("register")}
                        >
                          Sign up
                        </button>
                      </div>
                    </motion.form>
                  ) : (
                    <motion.form
                      key="register"
                      initial={{ x: direction === 1 ? 64 : -64, opacity: 0 }}
                      animate={{ x: 0, opacity: 1 }}
                      exit={{ x: direction === 1 ? -64 : 64, opacity: 0 }}
                      transition={{
                        type: "spring",
                        stiffness: 400,
                        damping: 30,
                      }}
                      className="flex flex-col gap-7 w-full items-center"
                      onSubmit={handleRegister}
                    >
                      <div className="grid gap-2 w-full">
                        <Label htmlFor="username">Username</Label>
                        <Input
                          id="username"
                          name="username"
                          type="text"
                          placeholder="yourname"
                          required
                        />
                      </div>
                      <div className="grid gap-2 w-full">
                        <Label htmlFor="email">Email</Label>
                        <Input
                          id="email"
                          name="email"
                          type="email"
                          placeholder="m@example.com"
                          required
                        />
                      </div>
                      <div className="grid gap-2 w-full">
                        <Label htmlFor="password">Password</Label>
                        <Input
                          id="password"
                          name="password"
                          type="password"
                          required
                        />
                      </div>
                      <Button
                        type="submit"
                        className="w-full"
                        disabled={loading}
                      >
                        {loading ? "Signing up..." : "Sign up"}
                      </Button>
                      <div className="text-center text-sm mt-2">
                        Already have an account?{" "}
                        <button
                          type="button"
                          className="underline underline-offset-4"
                          onClick={() => switchMode("login")}
                        >
                          Sign in
                        </button>
                      </div>
                    </motion.form>
                  )}
                </AnimatePresence>
              </div>
            </div>
          </div>
        </CardContent>
      </Card>
      <div className="text-muted-foreground *:[a]:hover:text-primary text-center text-xs text-balance *:[a]:underline *:[a]:underline-offset-4 mt-2">
        By clicking continue, you agree to our <a href="#">Terms of Service</a>{" "}
        and <a href="#">Privacy Policy</a>.
      </div>
    </div>
  );
}
