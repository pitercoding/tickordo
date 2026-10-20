import type { Metadata } from "next";
import { Geist, Geist_Mono } from "next/font/google";
import "./globals.css";

const geistSans = Geist({
  variable: "--font-geist-sans",
  subsets: ["latin"],
});

const geistMono = Geist_Mono({
  variable: "--font-geist-mono",
  subsets: ["latin"],
});

const siteTitle = "Tickordo — AI-Powered Support Ticket Triage";
const siteDescription =
  "Tickordo organizes support tickets and uses AI to help classify and prioritize them.";

export const metadata: Metadata = {
  metadataBase: new URL("https://tickordo.vercel.app"),
  title: {
    default: siteTitle,
    template: "%s · Tickordo",
  },
  description: siteDescription,
  applicationName: "Tickordo",
  openGraph: {
    type: "website",
    siteName: "Tickordo",
    title: siteTitle,
    description: siteDescription,
    locale: "en_US",
  },
  twitter: {
    card: "summary",
    title: siteTitle,
    description: siteDescription,
  },
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" className={`${geistSans.variable} ${geistMono.variable}`}>
      <body>{children}</body>
    </html>
  );
}
