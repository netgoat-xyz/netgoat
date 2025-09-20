import { verifySession } from "@/lib/session";

export async function GET(request: Request) {
  const { searchParams } = new URL(request.url);
  const session = searchParams.get("session");
  console.log(session)
  if (!session) {
    return new Response("Session token is required", { status: 400 });
  }

  try {
    const payload = await verifySession(session);
    console.log(payload)
    if (!payload) {
      return new Response("Invalid session token", { status: 401 });
    }
    return new Response(JSON.stringify(payload), {
      headers: { "Content-Type": "application/json" },
    });
  } catch (error) {
    console.error("Session verification failed:", error);
    return new Response("Internal server error", { status: 500 });
  }
}