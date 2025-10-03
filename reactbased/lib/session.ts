import { jwtVerify } from "jose";

const secretKey = process.env.JWT_SECRET;
const encodedKey = new TextEncoder().encode(secretKey);

export async function verifySession(session: string | undefined = "") {
  try {
    const { payload } = await jwtVerify(session, encodedKey, {
      algorithms: ["HS256"],
    });

    console.log(session, encodedKey);
    return payload;
  } catch (error) {
    console.log(error);
    console.log("Failed to verify session");
    return null;
  }
}
