import "./globals.css";
import { GeistSans } from "geist/font/sans";

export const metadata = {
  title: "Your App",
  description: "Welcome to Cloudable",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" className={GeistSans.className}>
      <body className="dark transition-all duration-300 antialiased">
        {children}
      </body>
    </html>
  );
}